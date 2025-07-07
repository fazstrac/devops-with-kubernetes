package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	// COMMIT_SHA and COMMIT_TAG are set by the build system
	COMMIT_SHA   string
	COMMIT_TAG   string
	counter      int
	counterMutex sync.Mutex // Mutex to protect counter access
)

func main() {
	counter = 0 // Initialize counter

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	router := setupRouter()
	router.Run("0.0.0.0:" + port)
}

func setupRouter() *gin.Engine {
	router := gin.Default()

	router.GET("/pingpong", func(c *gin.Context) {
		c.String(http.StatusOK, incrCounter())
	})
	return router
}

func incrCounter() string {
	counterMutex.Lock()
	counter++
	value := strconv.Itoa(counter)
	counterMutex.Unlock()

	return "pong " + value
}
