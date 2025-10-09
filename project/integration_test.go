package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type AppConfig struct {
	MaxAge       time.Duration
	GracePeriod  time.Duration
	FetchTimeout time.Duration
}

type testCase struct {
	name                   string
	backendHTTPHandlerFunc http.HandlerFunc
	initialFile            []byte
	expectedHTTPCode       int
	expectErr              bool
}

// Test application's endpoints. Mock only the backend server
// Uses httptest.Server to mock backend image server, file system operations are not mocked

// ** TestIntegrationGetImageCases1 **
// This test check for successes in the initial image fetch and
// that the backend is not called if there is a fresh image on startup.
// Tests using a single image, does not test automatic refresh.
// TODO: Add failures
func TestIntegrationGetImageCases1(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
	}

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	appConfig := AppConfig{
		MaxAge:       20 * time.Second,
		GracePeriod:  1 * time.Minute,
		FetchTimeout: FetchImageTimeout,
	}

	endpoint := "/images/image.jpg"

	testCases := []testCase{
		{
			name: "success cold start image not present",
			backendHTTPHandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				w.WriteHeader(http.StatusOK)
				w.Write(testImages[0])
			}),
			initialFile:      nil,
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
		{
			name: "success warm start image present",
			backendHTTPHandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				t.Fatal("Backend should not be called")
			}),
			expectedHTTPCode: http.StatusOK,
			initialFile:      testImages[0],
			expectErr:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runIntegrationTest1(t, tc, appConfig, testImages, endpoint)
		})
	}
}

// ** TestIntegrationGetImageCases2 **
//
// This test tests that the image does get automatically refreshed
// Tests using multiple images, tests automatic refresh. It does not test the grace period logic.
func TestIntegrationGetImageCases2(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
		[]byte("This is a test image content2"),
		[]byte("This is a test image content3"),
	}

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	appConfig := AppConfig{
		MaxAge:       5 * time.Second,
		GracePeriod:  2 * time.Second,
		FetchTimeout: FetchImageTimeout,
	}

	endpoint := "/images/image.jpg"

	testCases := []testCase{
		{
			name: "success cold start image not present",
			backendHTTPHandlerFunc: func() http.HandlerFunc {
				counter := 0

				// Serve different images on subsequent calls
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImages[counter])
					counter++
				})
			}(),
			initialFile:      nil,
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
		{
			name: "success warm start image present",
			backendHTTPHandlerFunc: func() http.HandlerFunc {
				counter := 1

				// Serve different images on subsequent calls
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if counter == 0 {
						w.WriteHeader(http.StatusForbidden)
						t.Fatal("Backend should not be called on first request")
					} else {
						w.Header().Set("Content-Type", "image/jpeg")
						w.WriteHeader(http.StatusOK)
						w.Write(testImages[counter])
					}
					counter++
				})
			}(),
			initialFile:      testImages[0],
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runIntegrationTest1(t, tc, appConfig, testImages, endpoint)
		})
	}
}

// ** TestIntegrationGetImageCases3 **
//
// This test tests the grace period logic using interleaved image fetches.
func TestIntegrationGetImageCases3(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
		[]byte("This is a test image content2"),
		[]byte("This is a test image content3"),
	}

	FetchImageTimeout := 2 * time.Second // Set a short timeout for testing

	appConfig := AppConfig{
		MaxAge:       5 * time.Second,
		GracePeriod:  2 * time.Second,
		FetchTimeout: FetchImageTimeout,
	}

	endpoint := "/images/image.jpg"

	serverOrchestrationChannel := make(chan int, 1)

	testCases := []testCase{
		{
			name: "success cold start image not present",
			backendHTTPHandlerFunc: func() http.HandlerFunc {
				var index int

				// Serve different images on subsequent calls
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					index = <-serverOrchestrationChannel // Wait for signal to proceed

					if index >= len(testImages) {
						w.WriteHeader(http.StatusNotFound)
						t.Fatal("Backend should not be called more than the number of test images")
						return
					}
					fmt.Println("Backend called, serving image", index)
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImages[index])
				})
			}(),
			initialFile:      nil,
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
		// {
		// 	name: "success warm start image present",
		// 	httpHandlerFunc: func() http.HandlerFunc {
		// 		counter := 1

		// 		// Serve different images on subsequent calls
		// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 			if counter == 0 {
		// 				w.WriteHeader(http.StatusForbidden)
		// 				t.Fatal("Backend should not be called on first request")
		// 			} else {
		// 				w.Header().Set("Content-Type", "image/jpeg")
		// 				w.WriteHeader(http.StatusOK)
		// 				w.Write(testImages[counter])
		// 			}
		// 			counter++
		// 		})
		// 	}(),
		// 	initialFile:      testImages[0],
		// 	expectedHTTPCode: http.StatusOK,
		// 	expectErr:        false,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runIntegrationTest2(t, tc, appConfig, testImages, endpoint, serverOrchestrationChannel)
		})
	}

	close(serverOrchestrationChannel)
}

func setupTestServer(handler http.HandlerFunc, initialFile []byte) (*httptest.Server, string, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ts := httptest.NewServer(handler)
	dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
	if initialFile != nil {
		os.WriteFile(dir+"/image.jpg", initialFile, 0644)
	}
	return ts, dir, ctx, cancel
}

func teardownTestServer(ts *httptest.Server, dir string, cancel context.CancelFunc) {
	cancel()
	ts.Close()
	os.RemoveAll(dir)
}

// Runs the integration test for a given test case for cases that do not test grace period logic
func runIntegrationTest1(t *testing.T, tc testCase, appConfig AppConfig, testImages [][]byte, endpoint string) {
	ts, dir, ctx, cancel := setupTestServer(tc.backendHTTPHandlerFunc, tc.initialFile)
	app := NewApp(
		dir+"/image.jpg",
		ts.URL,
		appConfig.MaxAge,
		appConfig.GracePeriod,
		appConfig.FetchTimeout,
	)

	fetchStatusChan := make(chan FetchResult)
	go app.ImageFetcher(ctx, fetchStatusChan)
	defer func() {
		<-fetchStatusChan
		close(fetchStatusChan)
	}()

	var fetchStatus FetchResult

	// Block until the cache load is complete
	fetchStatus = <-fetchStatusChan

	// Check image cache status
	// On cold start, image should not be available initially
	// On warm start, image should be available immediately
	if tc.initialFile == nil { // Cold start
		assert.False(t, fetchStatus.ImageAvailable)
	} else { // Warm start
		assert.True(t, fetchStatus.ImageAvailable)
	}

	router := setupRouter(app)

	// helper variable to track if we need to wait for image fetch

	imageAvailable := fetchStatus.ImageAvailable

	// A bit complicated logic to test refetching the images
	// without testing the grace period logic.
	// The first iteration will use the initial fetch status
	// Subsequent iterations will wait for a new fetch to complete
	// before making the next request.
	for imageIndex := range testImages {
		// Block until the image is available
		if !imageAvailable {
			fetchStatus = <-fetchStatusChan
			assert.True(t, fetchStatus.ImageAvailable)
		}

		// Make HTTP request to the application
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		body := w.Body.Bytes()
		assert.Equal(t, tc.expectedHTTPCode, resp.StatusCode)
		assert.Equal(t, testImages[imageIndex], body)

		// This forces a wait for the next image fetch, but
		// not for the last iteration --> avoids overfetching
		imageAvailable = false
	}

	teardownTestServer(ts, dir, cancel)
}

// Runs the integration test for a given test case for cases that test grace period logic
func runIntegrationTest2(t *testing.T, tc testCase, appConfig AppConfig, testImages [][]byte, endpoint string, serverOrchestrationChannel chan int) {
	ts, dir, ctx, cancel := setupTestServer(tc.backendHTTPHandlerFunc, tc.initialFile)
	app := NewApp(
		dir+"/image.jpg",
		ts.URL,
		appConfig.MaxAge,
		appConfig.GracePeriod,
		appConfig.FetchTimeout,
	)

	fetchStatusChan := make(chan FetchResult)
	go app.ImageFetcher(ctx, fetchStatusChan)
	defer func() {
		<-fetchStatusChan
		close(fetchStatusChan)
	}()

	var fetchStatus FetchResult

	serverOrchestrationChannel <- 0 // Allow the first fetch to proceed

	// Block until the cache load is complete
	fetchStatus = <-fetchStatusChan

	// Check image cache status
	// On cold start, image should not be available initially
	// On warm start, image should be available immediately
	if tc.initialFile == nil { // Cold start
		assert.False(t, fetchStatus.ImageAvailable)
	} else { // Warm start
		assert.True(t, fetchStatus.ImageAvailable)
	}

	router := setupRouter(app)

	// helper variable to track if we need to wait for image fetch
	imageAvailable := fetchStatus.ImageAvailable

	// A bit complicated logic to test refetching the images
	// without testing the grace period logic.
	// The first iteration will use the initial fetch status
	// Subsequent iterations will wait for a new fetch to complete
	// before making the next request.
	for imageIndex := 0; imageIndex < len(testImages)-1; imageIndex++ {
		// Block until the image is available
		if !imageAvailable {
			fetchStatus = <-fetchStatusChan
			assert.True(t, fetchStatus.ImageAvailable)
		}

		// RUN 1: Initial fetch or fetch after image became stale
		// Elapsed time: 0
		// Make HTTP request to the application
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		body := w.Body.Bytes()
		assert.Equal(t, tc.expectedHTTPCode, resp.StatusCode)
		assert.Equal(t, testImages[imageIndex], body)

		// RUN 2: Fetch while image is still fresh
		// Elapsed time: appConfig.GracePeriod / 2
		// Let's fetch the image again immediately to ensure that we do not call the backend again
		// TODO should somehow check that the backend was not called?
		time.Sleep(appConfig.GracePeriod / 2)
		req = httptest.NewRequest("GET", endpoint, nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp = w.Result()
		body = w.Body.Bytes()
		assert.Equal(t, tc.expectedHTTPCode, resp.StatusCode)
		assert.Equal(t, testImages[imageIndex], body)

		// RUN 3: Fetch after image became stale but within grace period
		// Elapsed time: appConfig.MaxAge + (appConfig.GracePeriod / 2) --> within grace period
		time.Sleep(appConfig.MaxAge) // We should now be within the grace period
		req = httptest.NewRequest("GET", endpoint, nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp = w.Result()
		body = w.Body.Bytes()
		assert.Equal(t, tc.expectedHTTPCode, resp.StatusCode)
		assert.Equal(t, testImages[imageIndex], body)

		// RUN 4: Fetch after grace period has been used
		// Elapsed time: appConfig.MaxAge + appConfig.GracePeriod / 2 --> grace period used
		// No reason to wait because the grace period has been used
		req = httptest.NewRequest("GET", endpoint, nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp = w.Result()
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		// This forces a wait for the next image fetch, but
		// not for the last iteration --> avoids overfetching
		imageAvailable = false
		serverOrchestrationChannel <- imageIndex + 1 // Allow the next fetch to proceed

		// Now the timing is off so we should synchronize with the fetcher
		fetchStatus = <-fetchStatusChan
		imageAvailable = fetchStatus.ImageAvailable
	}

	teardownTestServer(ts, dir, cancel)
}
