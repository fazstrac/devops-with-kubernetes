package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Type App holds the application state
// It's defined in app.go

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	app := NewApp(
		"./cache/image.jpg",          // Path to store the cached image
		"https://picsum.photos/1200", // Backend image URL
		10*time.Minute,               // Max age for the image
		1*time.Minute,                // Grace period during which the old image can be fetched _once_
		30*time.Second,               // Timeout for fetching the image from the backend
	)

	ctx, cancel := context.WithCancel(context.Background())
	go app.StartImageFetcher(ctx)
	defer cancel()

	app.LoadCachedImage()
	router := setupRouter(app)

	fmt.Println("Server started in port", os.Getenv("PORT"))
	router.Run("0.0.0.0:" + port)
}

func setupRouter(app *App) *gin.Engine {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", app.GetIndex)
	router.GET("/images/image.jpg", app.GetImage)
	// Add more routes here, using app methods
	return router
}
