package main

import (
	"fmt"
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	router := setupRouter()
	fmt.Println("Server started in port", os.Getenv("PORT"))
	router.Run("0.0.0.0:" + port)
}

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", getIndex)
	router.GET("/image.jpg", getImage)
	// Add more routes here as needed
	return router
}

func getIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.12",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func getImage(c *gin.Context) {
	isFresh := isImageFresh()
	if !isFresh {
		err := fetchImage()
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch image: %v", err)
			return
		}
	}
	imageData, err := readImage()
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

// Fetches an image from the url and saves it to the static folder
func fetchImage() error {
	resp, err := http.Get("https://picsum.photos/1200")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create("./cache/image.jpg")
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func isImageFresh() bool {
	// Check if the image file exists and is not older than 24 hours
	info, err := os.Stat("./cache/image.jpg")
	if err != nil {
		return false
	}
	return info.ModTime().Add(10 * time.Minute).After(time.Now())
}

func readImage() (string, error) {
	// Read the image file and return its content
	data, err := os.ReadFile("/app/cache/image.jpg")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
