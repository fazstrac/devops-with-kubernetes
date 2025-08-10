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
)

func main() {
	arglen := len(os.Args[1:])

	if arglen == 0 {
		fmt.Printf("Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	} else if arglen > 1 {
		fmt.Printf("Please give only one filename. Usage: %s <filename>\n", os.Args[0])
		os.Exit(1)
	}

	fmt.Printf("Starting pong-app (SHA %s).\n", COMMIT_SHA)

	counter = initCounter(os.Args[1])

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		os.Setenv("PORT", port)
	}

	router := setupRouter(os.Args[1])
	router.Run("0.0.0.0:" + port)
}

func setupRouter(fname string) *gin.Engine {
	router := gin.Default()

	router.GET("/pingpong", func(c *gin.Context) {
		c.String(http.StatusOK, incrCounter(fname))
	})
	return router
}

func initCounter(fname string) int {
	// Lock the mutex to ensure that only one goroutine can access the counter at a time
	// It should not happen as this is done in the main goroutine
	// but we do it anyway to be safe.
	counterMutex.Lock()
	defer counterMutex.Unlock()

	// The counter is initialized from the file if it exists.
	// If the file does not exist, it is created with a counter value of 0.
	value := "0"

	// Try to read the file to get the current counter value
	// If the file does not exist, we will create it later
	if _, err := os.Stat(fname); err == nil {
		// File exists, read the counter value
		data, err := os.ReadFile(fname)
		if err != nil {
			counter = 0
			fmt.Printf("Error reading counter file '%s': %v. Resetting counter to 0.\n", fname, err)
			return counter
		}
		// Otherwise, cast the byte slice to a string
		// and continue trying to parse it as an integer
		value = string(data)
	}

	// The contents should be parseable as an integer
	// This is a toy app so we will just set it to zero if it is not
	// an integer
	counter, err := strconv.Atoi(value)
	if err != nil {
		counter = 0
		fmt.Printf("Counter value in file '%s' is not an integer, resetting to 0.\n", fname)
	}

	return counter
}

func incrCounter(fname string) string {
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
		// os.Create creates or truncates the file
		// and returns a file pointer.
		// If the file already exists, it will be truncated to zero length.
		fp, err := os.Create(fname)

		if err != nil {
			// something went really wrong, die as this is completely unexpected
			panic(err)
		}
		defer fp.Close()

		_, err = fp.WriteString(value)

		if err != nil {
			// Writing should not fail in this case, so die screaming if there's an error
			// Of course can be running out of space
			panic(err)
		}
	}()
	counterMutex.Unlock()

	return "pong " + value
}
