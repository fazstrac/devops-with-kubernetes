package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	COMMIT_SHA string
	COMMIT_TAG string
	// Create function variables for easier testing/mocking
	StatFunc       = os.Stat
	ReadFileFunc   = os.ReadFile
	MkdirTempFunc  = os.MkdirTemp
	CreateFunc     = os.Create
	RemoveAllFunc  = os.RemoveAll
	RenameFunc     = os.Rename
	CopyFunc       = io.Copy
	FetchImageFunc = fetchImage
	SaveImageFunc  = saveImage
	waitTimes      = []time.Duration{
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
)

type ImageFetcher func(ctx context.Context)

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
	Fetcher           ImageFetcher
	fetchDoneChan     chan struct{}
	mutex             sync.RWMutex // Mutex to protect shared resources
}

func NewApp(imagePath, imageUrl string, maxAge, gracePeriod time.Duration, fetchTimeout time.Duration) *App {
	app := &App{
		ImagePath:         imagePath,
		ImageUrl:          imageUrl,
		MaxAge:            maxAge,
		GracePeriod:       gracePeriod,
		FetchImageTimeout: fetchTimeout,
	}

	app.Fetcher = func(ctx context.Context) {
		StartBackgroundImageFetcher(ctx, app)
	}

	return app
}

// Initializes the app by loading the cached image if it exists
func (app *App) LoadCachedImage() error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Check if the image directory exists
	_, err := StatFunc(filepath.Dir(app.ImagePath))
	if err != nil {
		// The directory does not exist or it's not accessible
		// This is a non-recoverable error. Return the error and let the caller handle it
		return err
	}

	// Check if the image file exists and get its info
	info, err := StatFunc(app.ImagePath)
	if err != nil {
		if os.IsNotExist(err) {
			// The image file does not exist, nothing to load
			// This is not an error
			return nil
		} else {
			// Some other error occurred while accessing the file
			return err
		}
	}

	// If the image file exists, set the fetched time to its modification time
	// This assumes that the image was fetched at the time it was last modified
	app.ImageFetchedAt = info.ModTime()
	// Reset grace period usage so it can be used again, even if the image is old
	// Don't care if the grace period was used before the app restart
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

	if age > app.MaxAge && age <= app.MaxAge+app.GracePeriod {
		if !app.IsGracePeriodUsed {
			app.IsGracePeriodUsed = true
		} else {
			c.Writer.Header().Set("Retry-After", "10")
			c.String(http.StatusServiceUnavailable, "Image is being fetched, please try again later")
			return
		}
	}

	if age <= app.MaxAge {
		app.IsGracePeriodUsed = false
	}

	imageData, err := readImage(app.ImagePath)
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

// Auxiliary functions
//

// Starts a background goroutine to fetch the image periodically
// It uses a ticker to trigger the fetch operation based on MaxAge
// If a fetch is already in progress, it skips the operation
// Missing error handling for timeout and other issues
func StartBackgroundImageFetcher(ctx context.Context, app *App) {
	ticker := time.NewTicker(app.MaxAge)
	defer ticker.Stop()

	// Initial fetch on startup

	// Lock for possible updates
	app.mutex.Lock()

	if !app.IsFetchingImage {
		// mutex locked, can update the state
		app.IsFetchingImage = true
		app.fetchDoneChan = make(chan struct{}) // Create a new channel for this fetch operation
		// Unlock before fetching the image
		app.mutex.Unlock()

		err := fetchImage(app.ImagePath, app.ImageUrl)

		// Lock again to update the state
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

	for {
		select {
		case <-ticker.C:
			app.mutex.Lock()
			// Only fetch if not already fetching
			if !app.IsFetchingImage {
				app.IsFetchingImage = true
				app.fetchDoneChan = make(chan struct{}) // Create a new channel for this fetch operation
				app.mutex.Unlock()                      // Unlock before fetching the image

				fmt.Println("Fetching image at", time.Now().Format(time.RFC3339))
				err := fetchImage(app.ImagePath, app.ImageUrl)
				fmt.Println("Image fetched at", time.Now().Format(time.RFC3339), "Error:", err)

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

// Fetches an image from the url and saves it to the static folder
// It retries on certain HTTP errors with backoff
//
//	*** caller must ensure proper locking if needed ***
func fetchImage(fname string, url string) error {
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
			return SaveImageFunc(fname, resp)
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

// saveImage saves the image from the HTTP response to the given path
// It saves the image to a temporary file first and then moves it to the final location
// to avoid partial writes
//
// *** caller must ensure proper locking ***
func saveImage(imagePath string, resp *http.Response) error {
	// Create a temporary file to save the image
	//
	dir, err := MkdirTempFunc(os.TempDir(), "dwk-project*")
	if err != nil {
		return err
	}

	defer RemoveAllFunc(dir)    // Clean up the temporary directory after the test
	fname := dir + "/image.jpg" // hardcoded filename inside the temp dir

	// Create the temporary file
	out, err := CreateFunc(fname)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = CopyFunc(out, resp.Body)
	if err != nil {
		return err
	}

	// Finally move the temp file to the final location
	// This is atomic on most operating systems
	// and ensures that we don't end up with a partial file
	// if the program crashes or is interrupted during the write
	return RenameFunc(fname, imagePath)
}

// readImage reads the image file without locking
// caller must ensure proper locking if needed
func readImage(fname string) (string, error) {
	// Read the image file and return its content
	data, err := ReadFileFunc(fname)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
