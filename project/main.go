package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var (
	// COMMIT_ID is the commit ID of the code
	COMMIT_ID string
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
	// Add more routes here as needed
	return router
}

func getIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.5",
		"body":  COMMIT_ID,
	})
}
