package main

import (
	"context"
	"fmt"
	"os"
	"sync"
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

	// Check if the image is cached from previous runs
	// and that it is still valid
	imageAvailable, err := app.LoadCachedImage()
	if !imageAvailable {
		fmt.Println("No cached image found, will fetch a new one:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fetchStatusChan := make(chan FetchResult)
	wg := sync.WaitGroup{}
	app.HeartbeatChan = make(chan struct{})
	wg.Add(1)
	go app.ImageFetcher(ctx, fetchStatusChan, &wg)

	app.StartPeriodicRefetchTrigger(ctx, &wg)
	defer func() {
		cancel()
		wg.Wait()
	}()

	fetchStatus := <-fetchStatusChan

	if fetchStatus.Err != nil {
		fmt.Println("Initial image fetch failed:", fetchStatus.Err)
		panic("Initial image fetch failed")
	}

	// Setup Gin router and routes
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
