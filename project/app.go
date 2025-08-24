package main

import (
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	// COMMIT_ID is the commit ID of the code
	COMMIT_SHA string
	COMMIT_TAG string
)

type ImageState int

const (
	ImageStateNotFetched ImageState = iota
	ImageStateFresh
	ImageStateStale
	ImageStateExpired
)

type App struct {
	ImagePath         string
	ImageUrl          string
	ImageFetchedAt    time.Time
	ImageLastServedAt time.Time
	IsGracePeriodUsed bool
	GracePeriod       time.Duration
	MaxAge            time.Duration
	mu                sync.RWMutex // Mutex to protect shared resources
}

func (app *App) getIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.12",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func (app *App) getImage(c *gin.Context) {
	// Check if the image is fresh, if not, fetch it
	app.mu.Lock()
	app.ImageLastServedAt = time.Now()
	app.mu.Unlock()

	app.mu.RLock()
	state := app.getImageStateUnlocked()
	graceUsed := app.IsGracePeriodUsed
	app.mu.RUnlock()

	switch {
	case state == ImageStateNotFetched, state == ImageStateExpired:
		func() {
			// Image is not fetched or expired, fetch it

			// Use a write lock to ensure only one thread fetches the image
			// while others wait. This prevents multiple threads from fetching the
			// image simultaneously and causing a thundering herd problem

			// Reset the grace period flag just in case
			app.mu.Lock()
			defer app.mu.Unlock()
			app.IsGracePeriodUsed = false

			// Re-check the state in case it changed while waiting for the lock
			state = app.getImageStateUnlocked()

			if state == ImageStateNotFetched || state == ImageStateExpired {
				err := fetchImageUnlocked(app.ImagePath, app.ImageUrl)
				if err != nil {
					c.String(http.StatusInternalServerError, "Failed to fetch image: %v", err)
					return
				}
				app.ImageFetchedAt = time.Now()
			}
			// Else, another thread has already fetched the image
			// and we can proceed to serve it
		}()
	case state == ImageStateStale && !graceUsed:
		// Image is stale but within grace period, serve it and mark grace period used
		app.mu.Lock()
		app.IsGracePeriodUsed = true
		app.ImageLastServedAt = time.Now()
		app.mu.Unlock()
	case state == ImageStateFresh:
		// Image is fresh, serve it and reset grace period flag just in case
		app.mu.Lock()
		app.IsGracePeriodUsed = false
		app.ImageLastServedAt = time.Now()
		app.mu.Unlock()
	default:
		c.String(http.StatusInternalServerError, "Unknown image state")
		return
	}

	// At this point, the image should be available

	// Read the image file and serve it
	// Note: this error handling can't be tested easily
	// without refactoring the readImage function's os.ReadFile to be
	// replaceable for testing purposes

	// Should be safe to read the image without a lock
	imageData, err := readImageUnlocked(app.ImagePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read image: %v", err)
		return
	}
	c.Writer.Header().Set("Content-Type", "image/jpeg")
	c.Writer.Header().Set("Cache-Control", "public, max-age=10") // Cache for 10 seconds
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write([]byte(imageData))

	// Handle write error from c.Writer.Write
	// Note: This error handling is also hard to test without
	// refactoring the c.Writer to be replaceable for testing purposes
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to write image: %v", err)
		return
	}
}

// Auxiliary functions
//

func (app *App) getImageStateUnlocked() ImageState {
	if app.ImageFetchedAt.IsZero() {
		return ImageStateNotFetched
	}

	age := time.Since(app.ImageFetchedAt)

	if age < app.MaxAge {
		return ImageStateFresh
	} else if age < app.MaxAge+app.GracePeriod { // grant 10 seconds of grace period
		return ImageStateStale
	} else {
		return ImageStateExpired
	}
}

// Fetches an image from the url and saves it to the static folder
// It retries on certain HTTP errors with backoff
//
//	*** caller must ensure proper locking if needed ***
func fetchImageUnlocked(fname string, url string) error {
	waitTimes := []time.Duration{
		// Fibonacci-like backoff times
		// in total, this will wait for 128 seconds
		// before giving up on fetching the image
		1 * time.Second,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		7 * time.Second,
		12 * time.Second,
		19 * time.Second,
		31 * time.Second,
		50 * time.Second,
	}

	// This will test against the following status codes:
	// 200 OK --> save the image and return,
	//
	// 500 Internal Server Error --> sleep and retry,
	// 502 Bad Gateway --> sleep and retry,
	// 503 Service Unavailable --> sleep and retry,
	// 504 Gateway Timeout --> sleep and retry,
	//
	// Other status codes --> return an error
	for _, wait := range waitTimes {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			// Image fetched successfully, save it
			return saveImageUnlocked(fname, resp)
		case http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			resp.Body.Close() // Close the response body to avoid resource leaks
			// Sleep for a while before retrying
			time.Sleep(wait)
		default:
			return http.ErrMissingFile // Return an error if the image is not found
		}
	}
	return http.ErrHandlerTimeout // Return an error if all retries are exhausted
}

// saveImageUnlocked saves the image from the HTTP response to the given path
// It saves the image to a temporary file first and then moves it to the final location
// to avoid partial writes
//
// *** caller must ensure proper locking if needed ***
func saveImageUnlocked(imagePath string, resp *http.Response) error {
	dir, err := os.MkdirTemp(os.TempDir(), "dwk-project*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dir) // Clean up the temporary directory after the test
	fname := dir + "/image.jpg"

	// Create the temporary file
	out, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// Move the temp file to the final location
	return os.Rename(fname, imagePath)
}

// readImageUnlocked reads the image file without locking
// caller must ensure proper locking if needed
func readImageUnlocked(fname string) (string, error) {
	// Read the image file and return its content
	data, err := os.ReadFile(fname)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
