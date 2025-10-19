package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TempFile interface {
	io.Closer
	io.Writer
	Name() string
	// ...other methods you need
}

var logger *log.Logger

var (
	COMMIT_SHA string
	COMMIT_TAG string
	// Create function variables for easier testing/mocking
	StatFunc               = os.Stat
	ReadFileFunc           = os.ReadFile
	CreateTempFunc         = func(dir, pattern string) (TempFile, error) { return os.CreateTemp(dir, pattern) }
	RemoveFunc             = os.Remove
	RenameFunc             = os.Rename
	CopyFunc               = io.Copy
	FetchImageFunc         = fetchImage
	SaveImageFunc          = saveImage
	RetryWithFibonacciFunc = retryWithFibonacci
	retryCounts            = 5
)

type App struct {
	ImagePath                  string
	BackendImageUrl            string
	ImageFetchedFromBackendAt  time.Time
	ImageLastServedAt          time.Time
	IsGracePeriodUsed          bool
	GracePeriod                time.Duration
	MaxAge                     time.Duration
	IsFetchingImageFromBackend bool
	FetchImageTimeout          time.Duration
	HeartbeatChan              chan struct{} // Channel to trigger image refetch
	mutex                      sync.RWMutex  // Mutex to protect shared resources
}

type FetchResult struct {
	ImageAvailable bool
	Path           string
	Err            error
}

func NewApp(imagePath, imageUrl string, maxAge, gracePeriod time.Duration, fetchTimeout time.Duration) *App {
	app := &App{
		ImagePath:         imagePath,
		BackendImageUrl:   imageUrl,
		MaxAge:            maxAge,
		GracePeriod:       gracePeriod,
		FetchImageTimeout: fetchTimeout,
	}

	app.HeartbeatChan = make(chan struct{}, 1) // Buffered channel to avoid blocking

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
	app.ImageFetchedFromBackendAt = info.ModTime()
	// Reset grace period usage so it can be used again, even if the image is old
	// Don't care if the grace period was used before the app restart
	app.IsGracePeriodUsed = false
	return true, nil
}

func (app *App) GetIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "DevOps with Kubernetes - Chapter 2 - Exercise 1.13",
		"body":  COMMIT_SHA + " (" + COMMIT_TAG + ")",
	})
}

func (app *App) GetImage(c *gin.Context) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Has the image ever been fetched?
	// NO --> return 503
	if app.ImageFetchedFromBackendAt.IsZero() {
		c.Writer.Header().Set("Retry-After", "10")
		c.String(http.StatusServiceUnavailable, "The image it is being fetched, please try again later")
		return
	}

	age := time.Since(app.ImageFetchedFromBackendAt)

	// Is the image being fetched?
	// YES --> check if we can serve the old image or not
	if app.IsFetchingImageFromBackend {
		// Is the image too old and is being fetched?
		if age > app.MaxAge+app.GracePeriod {
			c.Writer.Header().Set("Retry-After", "10")
			c.String(http.StatusServiceUnavailable, "Image is too old and it is being fetched, please try again later")
			return
		}

		// Is the image too old but within the grace period and is being fetched?
		if age > app.MaxAge && age <= app.MaxAge+app.GracePeriod {

			// Has the grace period been used already?
			// NO --> serve the old image and mark grace period as used
			// YES --> return 503
			if !app.IsGracePeriodUsed {
				app.IsGracePeriodUsed = true
			} else {
				c.Writer.Header().Set("Retry-After", "10")
				c.String(http.StatusServiceUnavailable, "Grace fetch already used. Image is being fetched, please try again later")
				return
			}
		}
	}

	// We are here so there should be valid image to serve
	app.ImageLastServedAt = time.Now()

	// If the image is not too old, reset the grace period usage
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

// Starts a goroutine that periodically sends to RefetchTriggerChan
// Returns the ticker so it can be manipulated (e.g. stopped) by the caller
func (app *App) StartPeriodicRefetchTrigger(ctx context.Context, wg *sync.WaitGroup) (ticker *time.Ticker) {
	wg.Add(1)
	ticker = time.NewTicker(app.MaxAge)

	go func() {

		for {
			select {
			case <-ticker.C:
				app.HeartbeatChan <- struct{}{}
			case <-ctx.Done():
				ticker.Stop()
				close(app.HeartbeatChan) // No more sends will happen
				wg.Done()
				return
			}
		}
	}()

	return ticker
}

// Starts the backend image fetcher goroutine. It loads the cached image, if it exists and then listens for refetch triggers
//
// Design choice 1: this is triggered via the HeartbeatChan, not immediately or direcly periodically. The
// caller has to start the periodic trigger separately and can also trigger it manually if needed.
// This allows for more flexibility for testing and better control over when to fetch
//
// Design choice 2: we do not use a separate channel for errors, we send them via the same channel as results
// This simplifies the design and makes it easier to handle errors in the caller
//
// Design choice 3: The fetch cannot be cancelled mid-way, it has to timeout or complete
func (app *App) StartBackgroundImageFetcher(ctx context.Context, wg *sync.WaitGroup) (initialFetchResult FetchResult, fetchResultChan chan FetchResult) {
	// Communicate the result of the cache load and image fetch via channel
	// Design choice: we do not panic here, we let the caller decide what to do
	// This is more flexible and allows for better error handling in the caller

	imageAvailable, err := app.LoadCachedImage()

	// If error occurred while loading the cached image, return the error, do not start the fetcher
	if err != nil {
		return FetchResult{ImageAvailable: false, Path: "", Err: err}, nil
	}

	fetchResultChan = make(chan FetchResult, 1) // Buffered channel to avoid blocking the sender

	// Everything ok so far, start the fetcher goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Listen for refetch triggers
		// This will run until the context is cancelled
		// or the HeartbeatChan is closed
		for {
			select {
			case <-app.HeartbeatChan:
				logger.Println("Received heartbeat, triggering image fetch from backend")

				app.mutex.Lock()
				app.IsFetchingImageFromBackend = true
				app.mutex.Unlock()

				err = tryFetchImageFromBackend(ctx, app)

				result := FetchResult{ImageAvailable: err == nil, Path: app.ImagePath, Err: err}

				// Send the result to the channel, but do not block if the channel is full
				select {
				case fetchResultChan <- result:
					// sent successfully
				default:
					// channel full, drop or log
					// This is normal in production as there is no-one waiting for the result
					// during normal operation. The channel is mainly for the initial fetch
					// and for testing purposes. In production, the channel will be full most of the time.
					// Using channel for notifying the caller of the result is a design choice, and should be
					// replaced with pub/sub or similar mechanism in a real-world application.
					// logger.Println("fetchResultChan full, dropping result")
				}

				// Design choice 4: if fetch failed even after retries, reuse the old image until next fetch
				// This prevents constant retries if the image URL is down for a long time
				// The grace period will be used if needed. However it does not take cold start into account
				// as the app should not start if it cannot load the cached image and cannot fetch a new one.
				//
				// This is a non-recoverable error and should be handled by the caller (e.g. exit the app)
				// If the image was never fetched successfully, the app will return 503 until it can fetch it
				// successfully
				app.mutex.Lock()
				app.IsFetchingImageFromBackend = false
				app.ImageFetchedFromBackendAt = time.Now()
				app.IsGracePeriodUsed = false
				app.mutex.Unlock()

				if err != nil {
					logger.Println("Image fetch from backend failed:", err)
				} else {
					logger.Println("Image fetch from backend succeeded")
				}
			case <-ctx.Done():
				logger.Println("Background image fetcher exiting due to context cancellation")
				close(fetchResultChan)
				return
			}
		}
	}()

	// Return the result of the cache load
	return FetchResult{ImageAvailable: imageAvailable, Path: app.ImagePath, Err: err}, fetchResultChan
}

// *** Auxiliary functions ***
//

func setupLogger() *log.Logger {
	logger = log.New(os.Stdout, "[DwK-Project] ", log.Ldate|log.Ltime|log.Lshortfile)

	return logger
}

// Retries the given function with Fibonacci backoff
// TODO: Add argument to cap the maximum wait time
func retryWithFibonacci(ctx context.Context, maxRetries int, fn func() (int, time.Duration, error)) error {
	fib := [3]time.Duration{0, time.Second, time.Second} // Start with 0s, 1s

	var lastErr error

	for i := range maxRetries {
		logger.Printf("Image fetch attempt %d/%d\n", i+1, maxRetries)

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

		logger.Printf("Waiting for %v before next retry\n", wait)
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
// Does not lock the app mutex, caller must ensure proper locking
func tryFetchImageFromBackend(ctx context.Context, app *App) error {
	err := RetryWithFibonacciFunc(ctx, retryCounts, func() (int, time.Duration, error) {
		return FetchImageFunc(app.ImagePath, app.BackendImageUrl, app.FetchImageTimeout)
	})
	return err
}

// Fetches an image from the url and saves it as the fname
// This handles the response based on the status code
// Special cases:
//
// 200 OK
//   - Save the image
//   - File error is the result of SaveImageFunc
//
// 429 Too Many Requests
// 503 Service Unavailable
//   - Parse Retry-After header if present and return it
//   - File error is http.ErrMissingFile
//   - Default wait is 0 --> caller to handle backoff
//
// TODO: Implement proper response for 202 Accepted: extract Location header
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

	// Split the imagePath into directory and filename
	dir, fname := filepath.Dir(imagePath), filepath.Base(imagePath)

	// Create a temporary file to save the image
	tempFile, err := CreateTempFunc(dir, fname+".tmp.*")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	defer RemoveFunc(tempFile.Name()) // Clean up the temp file on any error

	// Write the body to file
	_, err = CopyFunc(tempFile, resp.Body)
	if err != nil {
		return err
	}

	// Finally rename the temp file to the actual image.
	// This is atomic on most operating systems, assuming the source
	// and destination are on the same filesystem.
	err = RenameFunc(tempFile.Name(), imagePath)
	if err != nil {
		return err
	}

	return nil
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
