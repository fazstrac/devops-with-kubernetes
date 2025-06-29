package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	router := setupRouter()
	print("Server started in port %s", os.Getenv("PORT"))
	router.Run("0.0.0.0:" + port)
}

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/", getIndex)
	// Add more routes here as needed
	return router
}

func getIndex(c *gin.Context) {
	c.String(http.StatusOK, "Server started in port %s", os.Getenv("PORT"))
}
