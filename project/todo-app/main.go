package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Type App holds the application state
// It's defined in app.go

// Main function to start the server
// TODO for next iteration: graceful shutdown on SIGTERM/SIGINT
func main() {

	// Default port if not set via environment variable
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8080")
	}

	// Default backend image URL if not set via environment variable
	if os.Getenv("IMAGE_BACKEND_URL") == "" {
		os.Setenv("IMAGE_BACKEND_URL", "https://picsum.photos/1200")
	}

	logger = setupLogger()

	app := NewApp(
		"./cache/image.jpg",            // Path to store the cached image, hardcoded now for simplicity
		os.Getenv("IMAGE_BACKEND_URL"), // Backend image URL
		10*time.Minute,                 // Max age for the image
		1*time.Minute,                  // Grace period during which the old image can be fetched _once_
		30*time.Second,                 // Timeout for fetching the image from the backend
	)

	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())

	// Start the background image fetcher
	// It will return if LoadCachedImage fails for any reason
	fetchStatus, fetchStatusChan := app.StartBackgroundImageFetcher(ctx, &wg)
	if fetchStatus.Err != nil {
		logger.Fatal("Failed to start background image fetcher:", fetchStatus.Err)
		panic("Failed to start background image fetcher")
	}

	if !fetchStatus.ImageAvailable {
		logger.Println("Image not available in cache. Waiting for initial fetch...")
		// On cold start, trigger the first image fetch
		app.HeartbeatChan <- struct{}{}

		// Wait for the first image fetch result
		logger.Println("Waiting for initial image fetch result...")
		fetchStatus := <-fetchStatusChan
		logger.Println("Initial image fetch completed.")

		if fetchStatus.Err != nil {
			logger.Println("Initial image fetch failed:", fetchStatus.Err)
			panic("Initial image fetch failed")
		}
	}

	// Start the application heartbeat
	// Currently used only to trigger periodic image refetches
	ticker := app.StartPeriodicRefetchTrigger(ctx, &wg)

	defer func() {
		ticker.Stop()
		cancel()
		wg.Wait()
	}()

	// Setup Gin router and routes
	router := setupRouter(app)

	logger.Println("Server starting in port", os.Getenv("PORT"))
	router.Run("0.0.0.0:" + os.Getenv("PORT"))
}

func setupRouter(app *App) *gin.Engine {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", app.GetIndex)
	router.GET("/images/image.jpg", app.GetImage)
	router.Static("/static", "./static")

	// Add more routes here, using app methods
	return router
}
