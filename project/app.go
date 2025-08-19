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
}

func (app *App) getIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.12",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func (app *App) getImage(c *gin.Context) {
	// Check if the image is fresh, if not, fetch it

	isFresh := isImageFresh(app.ImagePath, 10*time.Minute)
	if !isFresh {
		err := fetchImage(app.ImagePath)
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
	c.Writer.Header().Set("Cache-Control", "public, max-age=10") // Cache for 1o seconds
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
func fetchImage(fname string) error {
	resp, err := http.Get("https://picsum.photos/1200")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
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
