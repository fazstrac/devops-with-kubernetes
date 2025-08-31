package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	COMMIT_SHA string
	COMMIT_TAG string
)

type App struct {
	ImagePath         string
	ImageUrl          string
	ImageFetchedAt    time.Time
	ImageLastServedAt time.Time
	IsGracePeriodUsed bool
	GracePeriod       time.Duration
	MaxAge            time.Duration
	IsFetchingImage   bool
	FetchImageTimeout time.Duration
	fetchDoneChan     chan struct{}
	mutex             sync.RWMutex // Mutex to protect shared resources
}

func NewApp(imagePath, imageUrl string, maxAge, gracePeriod time.Duration, fetchTimeout time.Duration) *App {
	return &App{
		ImagePath:         imagePath,
		ImageUrl:          imageUrl,
		MaxAge:            maxAge,
		GracePeriod:       gracePeriod,
		FetchImageTimeout: fetchTimeout,
	}
}

// Starts a background goroutine to fetch the image periodically
// It uses a ticker to trigger the fetch operation based on MaxAge
// If a fetch is already in progress, it skips the operation
// Missing error handling for timeout and other issues
func (app *App) StartImageFetcher(ctx context.Context) {
	ticker := time.NewTicker(app.MaxAge)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.mutex.Lock()
			// Only fetch if not already fetching
			if !app.IsFetchingImage {
				app.IsFetchingImage = true
				app.fetchDoneChan = make(chan struct{}) // Create a new channel for this fetch operation
				app.mutex.Unlock()                      // Unlock before fetching the image

				err := fetchImageUnlocked(app.ImagePath, app.ImageUrl)

				app.mutex.Lock()
				app.ImageFetchedAt = time.Now()
				app.IsFetchingImage = false
				close(app.fetchDoneChan) // Signal that fetching is done
				app.mutex.Unlock()

				if err != nil {
					// Log the error, but do not terminate the fetcher
					// In real application, use a proper logging framework
					// Here we just print to stdout for simplicity
					println("Failed to fetch image:", err.Error())
				}
			} else {
				app.mutex.Unlock() // Already fetching, just unlock
			}
		case <-ctx.Done():
			return
		}
	}
}

// Initializes the app by loading the cached image if it exists
func (app *App) LoadCachedImage() error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	info, err := os.Stat(app.ImagePath)
	if err != nil {
		return err
	}

	app.ImageFetchedAt = info.ModTime()
	app.IsGracePeriodUsed = false
	return nil
}

func (app *App) GetIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.12",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func (app *App) GetImage(c *gin.Context) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	age := time.Since(app.ImageFetchedAt)

	if age > app.MaxAge+app.GracePeriod {
		c.Writer.Header().Set("Retry-After", "10")
		c.String(http.StatusServiceUnavailable, "Image is being fetched, please try again later")
		return
	}

	if age > app.MaxAge && !app.IsGracePeriodUsed {
		app.IsGracePeriodUsed = true
	} else if age <= app.MaxAge {
		app.IsGracePeriodUsed = false
	}

	imageData, err := readImageUnlocked(app.ImagePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read image: %v", err)
		return
	}

	c.Writer.Header().Set("Content-Type", "image/jpeg")
	c.Writer.Header().Set("Cache-Control", "public, max-age=10")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write([]byte(imageData))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to write image: %v", err)
	}
}

}

// Auxiliary functions
//

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
	// Create a temporary file to save the image
	//
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

	// Finally move the temp file to the final location
	// This is atomic on most operating systems
	// and ensures that we don't end up with a partial file
	// if the program crashes or is interrupted during the write
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
