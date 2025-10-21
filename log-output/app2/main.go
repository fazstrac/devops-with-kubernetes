package main

import (
	"errors"
	"fmt"
	"io"
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

	pongAppSvcUrl := os.Getenv("PONGAPP_SVC_URL")
	if pongAppSvcUrl == "" {
		panic("PONGAPP_SVC_URL environment variable not set")
	}

	arglen := len(os.Args[1:])

	if arglen == 0 {
		fmt.Printf("Usage: %s <logfilename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen != 1 {
		fmt.Printf("Please give only one filename. Usage: %s <logfilename>\n", os.Args[0])
		os.Exit(1)
	}

	logFName := os.Args[1]

	fmt.Printf("Starting app2 (SHA %s) with files %s.\n", COMMIT_SHA, logFName)

	router := setupRouter(logFName)
	router.Run("0.0.0.0:" + port)
}

func setupRouter(logFName string) *gin.Engine {
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

		log_data, err3 := os.ReadFile(logFName)

		response, err4 := http.Get(os.Getenv("PONGAPP_SVC_URL") + "/pongs")

		if err3 != nil || err4 != nil {
			c.String(http.StatusInternalServerError, "Error reading file or making HTTP request: %v %v", err3, err4)
			return
		}

		buf := make([]byte, 128)

		datalength, err4 := response.Body.Read(buf)
		response.Body.Close()

		counter_data := buf[:datalength]

		if err4 != nil && !errors.Is(err4, io.EOF) {
			c.String(http.StatusInternalServerError, "Error reading counter data: %v", err4)
			return
		}

		message := string(log_data) + "Ping / Pongs: " + string(counter_data)

		c.String(http.StatusOK, message)
	})
	return router
}
