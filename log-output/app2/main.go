package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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

	arglen := len(os.Args[1:])

	if arglen == 0 {
		fmt.Printf("Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen > 1 {
		fmt.Printf("Please give only one filename. Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	}

	fname := os.Args[1]

	fmt.Printf("Starting app2 (SHA %s) with file %s\n", COMMIT_SHA, fname)

	router := setupRouter(fname)
	router.Run("0.0.0.0:" + port)
}

func setupRouter(fname string) *gin.Engine {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/log")
	})

	router.GET("/log", func(c *gin.Context) {
		fp, err := os.OpenFile(fname, os.O_RDONLY, 0644)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error opening log file: %v", err)
			return
		}
		defer fp.Close()

		log_data, err := os.ReadFile(fname)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading log file: %v", err)
			return
		}

		c.String(http.StatusOK, string(log_data))
	})
	return router
}
