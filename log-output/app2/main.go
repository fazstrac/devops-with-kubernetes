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
		fmt.Printf("Usage: %s <logfilename> <counterfilename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen != 2 {
		fmt.Printf("Please give only two filenames. Usage: %s <logfilename> <counterfilename>\n", os.Args[0])
		os.Exit(1)
	}

	logFName := os.Args[1]
	counterFName := os.Args[2]

	fmt.Printf("Starting app2 (SHA %s) with files %s and %s.\n", COMMIT_SHA, logFName, counterFName)

	router := setupRouter(logFName, counterFName)
	router.Run("0.0.0.0:" + port)
}

func setupRouter(logFName string, counterFName string) *gin.Engine {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/log")
	})

	router.GET("/log", func(c *gin.Context) {
		fp, err := os.OpenFile(logFName, os.O_RDONLY, 0644)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error opening log file: %v", err)
			return
		}
		defer fp.Close()

		counterFP, err2 := os.OpenFile(counterFName, os.O_RDONLY, 0644)

		// This can mean the the pong app has not been run yet. Not actually an error.
		// It should be handled gracefully, but for now let's just return an error message.
		if err2 != nil {
			c.String(http.StatusInternalServerError, "Error opening counter file: %v", err2)
			return
		}
		defer counterFP.Close()

		log_data, err3 := os.ReadFile(logFName)
		counter_data, err4 := os.ReadFile(counterFName)

		if err3 != nil || err4 != nil {
			c.String(http.StatusInternalServerError, "Error reading file: %v %v", err3, err4)
			return
		}

		message := string(log_data) + "Ping / Pongs: " + string(counter_data)

		c.String(http.StatusOK, message)
	})
	return router
}
