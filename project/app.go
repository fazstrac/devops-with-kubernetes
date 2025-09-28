package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	COMMIT_SHA string
	COMMIT_TAG string
	// Create function variables for easier testing/mocking
	StatFunc               = os.Stat
	ReadFileFunc           = os.ReadFile
	MkdirTempFunc          = os.MkdirTemp
	CreateFunc             = os.Create
	RemoveAllFunc          = os.RemoveAll
	RenameFunc             = os.Rename
	CopyFunc               = io.Copy
	FetchImageFunc         = fetchImage
	SaveImageFunc          = saveImage
	RetryWithFibonacciFunc = retryWithFibonacci
	retryCounts            = 5
)

type ImageFetcher func(ctx context.Context, out chan<- FetchResult)

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

type FetchResult struct {
	ImageAvailable bool
	Path           string
	Err            error
}

func NewApp(imagePath, imageUrl string, maxAge, gracePeriod time.Duration, fetchTimeout time.Duration) *App {
	app := &App{
		ImagePath:         imagePath,
		ImageUrl:          imageUrl,
		MaxAge:            maxAge,
		GracePeriod:       gracePeriod,
		FetchImageTimeout: fetchTimeout,
	}

	app.Fetcher = func(ctx context.Context, out chan<- FetchResult) {
		StartBackgroundImageFetcher(ctx, out, app)
	}

	return app
}

// Initializes the app by loading the cached image if it exists
func (app *App) LoadCachedImage() (imageAvailable bool, err error) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Check if the image directory exists
	_, err = StatFunc(filepath.Dir(app.ImagePath))
	if err != nil {
		// The directory does not exist or it's not accessible
		// This is a non-recoverable error. Return the error and let the caller handle it
		return false, err
	}

	// Check if the image file exists and get its info
	info, err := StatFunc(app.ImagePath)
	if err != nil {
		if os.IsNotExist(err) {
			// The image file does not exist, nothing to load
			// This is not an error
			return false, nil
		} else {
			// Some other error occurred while accessing the file
			return false, err
		}
	}

	// If the image file exists, set the fetched time to its modification time
	// This assumes that the image was fetched at the time it was last modified
	app.ImageFetchedAt = info.ModTime()
	// Reset grace period usage so it can be used again, even if the image is old
	// Don't care if the grace period was used before the app restart
	app.IsGracePeriodUsed = false
	return true, nil
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
		c.String(http.StatusServiceUnavailable, "Image is way too old and it is being fetched, please try again later")
		return
	}

	if age > app.MaxAge && age <= app.MaxAge+app.GracePeriod {
		if !app.IsGracePeriodUsed {
			app.IsGracePeriodUsed = true
		} else {
			c.Writer.Header().Set("Retry-After", "10")
			c.String(http.StatusServiceUnavailable, "Grace period already used. Image is being fetched, please try again later")
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
// It tries to fetch the image immediately with a timeout,
// then it starts a
func StartBackgroundImageFetcher(ctx context.Context, out chan<- FetchResult, app *App) {
	var ticker *time.Ticker

	// Communicate the result of the cache load and image fetch via channel
	// This allows the caller to know if the initial fetch succeeded or failed
	// and act accordingly (e.g., panic if it failed)
	//
	// Design choice: we do not panic here, we let the caller decide what to do
	// This is more flexible and allows for better error handling in the caller

	imageAvailable, err := app.LoadCachedImage()
	out <- FetchResult{ImageAvailable: imageAvailable, Path: app.ImagePath, Err: err}

	// Calculate the time to wait until the next fetch
	wait := max(app.MaxAge-time.Since(app.ImageFetchedAt), 200*time.Millisecond)
	fmt.Printf("Initial wait before first fetch: %v\n", wait)

	timer := time.NewTimer(wait)
	defer timer.Stop()

	// Wait for the initial wait duration or context cancellation
	select {
	case <-timer.C: // Initial wait is over, start the periodic fetch
		fmt.Println("Starting background image fetcher with interval:", app.MaxAge)
		ticker = time.NewTicker(app.MaxAge)
		defer ticker.Stop()

		err = tryFetchImage(ctx, app)
		if err != nil {
			fmt.Printf("Background image fetch failed: %v\n", err)
		}
		out <- FetchResult{ImageAvailable: err != nil, Path: app.ImagePath, Err: err}
	case <-ctx.Done(): // Context cancelled before the initial wait is over
		out <- FetchResult{Path: "", Err: ctx.Err()}
		return
	}

	// Actual periodic fetch loop
	// This runs until the context is cancelled
	// It fetches the image every MaxAge duration
	for {
		select {
		case <-ticker.C:
			err = tryFetchImage(ctx, app)
			if err != nil {
				fmt.Printf("Background image fetch failed: %v\n", err)
			}
			out <- FetchResult{Path: app.ImagePath, Err: err}

			// Design choice: if fetch failed even after retries, reuse the old image until next fetch
			// This prevents constant retries if the image URL is down for a long time
			// The grace period will be used if needed
			app.ImageFetchedAt = time.Now()
			app.IsGracePeriodUsed = false
		case <-ctx.Done():
			out <- FetchResult{Path: "", Err: ctx.Err()}

			return
		}
	}
}

func retryWithFibonacci(ctx context.Context, maxRetries int, fn func() (int, time.Duration, error)) error {
	fib := [3]time.Duration{0, time.Second, time.Second} // Start with 0s, 1s

	var lastErr error

	for range maxRetries {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		status, waitDuration, err := fn()

		switch status {
		case http.StatusOK:
			return nil
		case http.StatusTooManyRequests:
		case http.StatusServiceUnavailable:
			lastErr = err
		case 666:
		default:
			// Other errors are considered non-retryable
			return err
		}

		// Calculate the wait duration
		// Use the maximum of the error's suggested wait time and the Fibonacci backoff
		// This ensures we respect server's Retry-After header if provided
		// and also implement our own backoff strategy

		wait := max(waitDuration, fib[2])

		// Wait using Fibonacci backoff
		select {
		case <-time.After(wait): // We waited long enough
			// Continue to next retry
			fib[2] = fib[0] + fib[1]
			fib[0] = fib[1]
			fib[1] = fib[2]
		case <-ctx.Done(): // Context cancelled or timed out
			return ctx.Err()
		}
	}

	// All retries exhausted
	return fmt.Errorf("all retries failed: %w", lastErr)
}

// Attempts to fetch the image with retries and timeout
func tryFetchImage(ctx context.Context, app *App) error {
	app.mutex.Lock()

	if !app.IsFetchingImage {
		app.IsFetchingImage = true
		app.fetchDoneChan = make(chan struct{}) // Create a new channel for this fetch operation
		// Unlock before fetching the image
		app.mutex.Unlock()

		err := RetryWithFibonacciFunc(ctx, retryCounts, func() (int, time.Duration, error) {
			return FetchImageFunc(app.ImagePath, app.ImageUrl, app.FetchImageTimeout)
		})

		// Lock again to update the state
		app.mutex.Lock()
		app.ImageFetchedAt = time.Now()
		app.IsFetchingImage = false
		close(app.fetchDoneChan) // Signal that fetching is done
		app.mutex.Unlock()

		return err
	} else {
		app.mutex.Unlock()
		return nil
	}
}

// Fetches an image from the url and saves it as the fname
// This handles the response based on the status code
// Special cases:
//
//		200 OK
//	  - Save the image
//	  - File error is the result of SaveImageFunc
//		429 Too Many Requests or 503 Service Unavailable:
//	  - Parse Retry-After header if present and return it
//	  - File error is http.ErrMissingFile
//	  - Default wait is 0 --> caller to handle backoff
//
// FIXME: Implement proper response for 202 Accepted: extract Location header
func fetchImage(fname string, url string, timeOut time.Duration) (status int, wait time.Duration, err error) {
	client := http.Client{
		Timeout: timeOut,
	}
	resp, err := client.Get(url)

	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusServiceUnavailable, time.Duration(0), err
	} else if err != nil {
		return 666, time.Duration(0), err // 666 is a custom code for other network errors
	}

	defer resp.Body.Close()

	wait = time.Duration(0)

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.StatusCode, wait, SaveImageFunc(fname, resp)
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		retryAfter := resp.Header.Get("Retry-After")

		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				// Retry after this many seconds
				wait = time.Duration(seconds) * time.Second
			} else if t, err := http.ParseTime(retryAfter); err == nil {
				// Retry after this duration
				wait = time.Until(t).Round(time.Second)
			}
		}
		return resp.StatusCode, wait, http.ErrMissingFile
	default:
		return resp.StatusCode, wait, http.ErrMissingFile
	}
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
