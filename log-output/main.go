package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	// COMMIT_SHA and COMMIT_TAG are set by the build system
	COMMIT_SHA string
	COMMIT_TAG string
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	myuuid := uuid.New().String()

	// Start goroutine to print log every 5 seconds
	go func() {
		for {
			logLine := logString(myuuid)
			fmt.Println(logLine)
			time.Sleep(5 * time.Second)
		}
	}()

	router := setupRouter(myuuid)
	router.Run("0.0.0.0:" + port)
}

func setupRouter(id string) *gin.Engine {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/log")
	})

	router.GET("/log", func(c *gin.Context) {
		c.String(http.StatusOK, logString(id))
	})
	return router
}

// logString returns the formatted log string
func logString(id string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	return fmt.Sprintf("%s: %s", timestamp, id)
}
