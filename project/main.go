package main

import (
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

	app := NewApp("./cache/image.jpg", "https://picsum.photos/1200", 10*time.Minute, 1*time.Minute)

	router := setupRouter(app)
	fmt.Println("Server started in port", os.Getenv("PORT"))
	router.Run("0.0.0.0:" + port)
}

func setupRouter(app *App) *gin.Engine {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", app.getIndex)
	router.GET("/image.jpg", app.getImage)
	// Add more routes here, using app methods
	return router
}
