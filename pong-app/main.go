package main

import (
	"fmt"
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
	fname        string
)

func main() {
	counter = 0 // Initialize counter

	arglen := len(os.Args[1:])

	if arglen == 0 {
		fmt.Printf("Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen > 1 {
		fmt.Printf("Please give only one filename. Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	}

	fname = os.Args[1]

	fmt.Printf("Starting pong-app (SHA %s).\n", COMMIT_SHA)

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

	// The purpose is to pass the counter value via filesystem
	// to other pods sharing the same file system.
	// This may just work, but a major issue is that
	// there is no file locking and sharing data like this is
	// JUST DEAD WRONG to begin with.
	//
	// In production one should use something like
	// Valkey / Redis, RabbitMQ, NATS to share
	// the counter value.

	func() {
		fp, err := os.Create(fname)

		if err != nil {
			// something went really wrong, die as it's can't expect it to be a transient issue
			panic(err)
		}
		defer fp.Close()

		_, err = fp.WriteString(value)

		if err != nil {
			// Writing should not fail, so die screaming if there's an error
			panic(err)
		}
	}()
	counterMutex.Unlock()

	return "pong " + value
}
