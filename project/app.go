package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	// COMMIT_ID is the commit ID of the code
	COMMIT_SHA string
	COMMIT_TAG string
)

type App struct {
	ImagePath string
	ImageUrl  string
	MaxAge    time.Duration
}

func (app *App) getIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.12",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func (app *App) getImage(c *gin.Context) {
	// Check if the image is fresh, if not, fetch it

	isFresh := isImageFresh(app.ImagePath, app.MaxAge)
	if !isFresh {
		err := fetchImage(app.ImagePath, app.ImageUrl)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch image: %v", err)
			return
		}
	}
	imageData, err := readImage(app.ImagePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read image: %v", err)
		return
	}
	c.Writer.Header().Set("Content-Type", "image/jpeg")
	c.Writer.Header().Set("Cache-Control", "public, max-age=10") // Cache for 10 seconds
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write([]byte(imageData))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to write image: %v", err)
		return
	}
}

// Auxiliary functions
//

// Fetches an image from the url and saves it to the static folder
func fetchImage(fname string, url string) error {
	waitTimes := []time.Duration{
		// Fibonacci-like backoff times
		// in total, this will wait for 128 seconds
		// before giving up on fetching the image
		1 * time.Second,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		7 * time.Second,
		12 * time.Second,
		19 * time.Second,
		31 * time.Second,
		50 * time.Second,
	}

	// Try to fetch the image from the URL up to 3 times
	// If the image is not available, return an error
	// This is useful for cases where the image might not be available immediately
	// or the URL might be temporarily down
	// This will test against the following status codes:
	// 200 OK,
	// 502 Bad Gateway,
	// 503 Service Unavailable,
	// 504 Gateway Timeout
	// If the image is not available, return an error
	//
	// It is missing logic to handle codes like 202 Accepted or 204 No Content
	// which might be used in some cases where the image is not available yet
	// or the server is still processing the request
	for _, wait := range waitTimes {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			out, err := os.Create(fname)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			return err
		} else if resp.StatusCode != http.StatusInternalServerError &&
			resp.StatusCode != http.StatusBadGateway &&
			resp.StatusCode != http.StatusServiceUnavailable &&
			resp.StatusCode != http.StatusGatewayTimeout {

			resp.Body.Close()          // Close the response body to avoid resource leaks
			return http.ErrMissingFile // Return an error if the image is not found
		}

		// Sleep for a while before retrying
		time.Sleep(wait)
		resp.Body.Close()
	}

	return http.ErrMissingFile // Return an error if the image could not be fetched
}

func isImageFresh(fname string, max_age time.Duration) bool {
	// Check if the image file exists and is not older than 24 hours
	info, err := os.Stat(fname)
	if err != nil {
		return false
	}
	return info.ModTime().Add(max_age).After(time.Now())
}

func readImage(fname string) (string, error) {
	// Read the image file and return its content
	data, err := os.ReadFile(fname)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
