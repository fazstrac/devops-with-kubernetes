package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// COMMIT_SHA and COMMIT_TAG are set by the build system
	COMMIT_SHA string
	COMMIT_TAG string
)

func main() {
	// Set default port if not set via environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	// Verify that the PONGAPP_SVC_URL environment variable is set
	pongAppSvcUrl := os.Getenv("PONGAPP_SVC_URL")
	if pongAppSvcUrl == "" {
		panic("PONGAPP_SVC_URL environment variable not set")
	}

	// Verify that the MESSAGE environment variable is set
	env_message := os.Getenv("MESSAGE")
	if env_message == "" {
		panic("MESSAGE environment variable not set")
	}

	// Verify that the COMMON_LOGFILE_NAME environment variable is set
	logFileName := os.Getenv("COMMON_LOGFILE_NAME")
	if logFileName == "" {
		panic("COMMON_LOGFILE_NAME environment variable not set")
	}

	// Verify that the message file exists and is readable
	messageFName := "/etc/config/message.txt"
	fmsg, err := os.OpenFile(messageFName, os.O_RDONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("Error opening message file: %v", err))
	}
	fmsg.Close()

	// All good - proceed to start the web server

	// Construct the log file name with hardcoded /data/ path
	logFName := "/data/" + logFileName

	fmt.Printf("Starting app2 (SHA %s) with files %s.\n", COMMIT_SHA, logFName)

	router := setupRouter(logFName, messageFName, pongAppSvcUrl)
	router.Run("0.0.0.0:" + port)
}

func setupRouter(logFName string, messageFName string, pongAppUrl string) *gin.Engine {
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

		// Read input data from three sources: log file, message file, and pong app
		// Read everything on every request to allow seeing updates without restarting
		// Another option would be to watch the files for changes
		log_data, err3 := os.ReadFile(logFName)
		message1_data, err4 := os.ReadFile(messageFName)
		message2_data := os.Getenv("MESSAGE")
		response, err5 := http.Get(pongAppUrl)

		if err3 != nil || err4 != nil || err5 != nil {
			c.String(http.StatusInternalServerError, "Error reading file or making HTTP request: %v %v %v", err3, err4, err5)
			return
		}

		// Expecting a short response, so even 128 bytes is a bit of an overkill
		buf := make([]byte, 128)

		datalength, err4 := response.Body.Read(buf)
		response.Body.Close()

		counter_data := buf[:datalength]

		if err4 != nil && !errors.Is(err4, io.EOF) {
			c.String(http.StatusInternalServerError, "Error reading counter data: %v", err4)
			return
		}

		message := fmt.Sprintf(
			"%s\nfile content: %s\n env variable: %s\nPing / Pongs: %s\n",
			strings.TrimSpace(string(log_data)),
			strings.TrimSpace(string(message1_data)),
			strings.TrimSpace(message2_data),
			strings.TrimSpace(string(counter_data)),
		)

		c.String(http.StatusOK, message)
	})
	return router
}
