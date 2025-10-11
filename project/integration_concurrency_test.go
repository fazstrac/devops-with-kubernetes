package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type concurrencyTestRunner func(t *testing.T, tc testCase, appConfig AppConfig, testImages [][]byte, endpoint string, backendServerOrchestratorChan chan int)

func TestConcurrentImageRequests(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
		[]byte("This is a test image content2"),
		[]byte("This is a test image content3"),
	}

	testRunners := []concurrencyTestRunner{
		runIntegrationConcurrencyTest1,
		runIntegrationConcurrencyTest2,
	}

	appConfig := AppConfig{
		MaxAge:       20 * time.Second,
		GracePeriod:  60 * time.Second,
		FetchTimeout: 10 * time.Second,
	}

	backendServerOrchestratorChan := make(chan int)

	endpoint := "/images/image.jpg"

	testCases := []testCase{
		{
			name: "concurrent requests with cold start",
			backendHTTPHandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				index := <-backendServerOrchestratorChan
				time.Sleep(250 * time.Millisecond) // Simulate some delay
				// Serve the selected test image
				w.WriteHeader(http.StatusOK)
				w.Write(testImages[index])

			},
			initialFile:      nil,
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
		{
			name: "concurrent requests with warm start",
			backendHTTPHandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				index := <-backendServerOrchestratorChan
				time.Sleep(250 * time.Millisecond) // Simulate some delay
				w.WriteHeader(http.StatusOK)
				w.Write(testImages[index])
			},
			initialFile:      testImages[0],
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
	}

	for _, tc := range testCases {
		for i, runTest := range testRunners {
			t.Run(tc.name+" test runner "+strconv.Itoa(i), func(t *testing.T) {
				runTest(t, tc, appConfig, testImages, endpoint, backendServerOrchestratorChan)
			})
		}
	}
}

// Runs the integration test for a given test case for cases that do not test grace period logic
func runIntegrationConcurrencyTest1(t *testing.T, tc testCase, appConfig AppConfig, _ [][]byte, endpoint string, backendServerOrchestratorChan chan int) {
	ts, dir, ctx, cancel, wg := setupTestServer(tc.backendHTTPHandlerFunc, tc.initialFile)

	app := NewApp(
		dir+"/image.jpg",
		ts.URL,
		appConfig.MaxAge,
		appConfig.GracePeriod,
		appConfig.FetchTimeout,
	)

	fetchStatus, fetchStatusChan := app.StartBackgroundImageFetcher(ctx, wg)

	assert.NoError(t, fetchStatus.Err)

	// *** Setup phase ***

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

	startGoRoutinesChan := make(chan struct{}, 1)

	var request_wg sync.WaitGroup
	numParallelRequests := 10

	// Start multiple goroutines to make concurrent requests
	for i := 0; i < numParallelRequests; i++ {
		request_wg.Add(1)
		go func() {
			defer request_wg.Done()
			// wait until signaled to start
			<-startGoRoutinesChan
			// random short delay to better simulate real-world concurrent requests
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			// Make the request
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	// *** Execution phase ***
	// Prepare the system start
	imageIndex := 0

	if !imageAvailable { // Cold start, need to fetch the first image
		// Trigger image fetch from backend
		app.HeartbeatChan <- struct{}{}

		// Trigger backend server to serve the next image
		backendServerOrchestratorChan <- imageIndex

		// Wait for the image fetch result
		fetchStatus = <-fetchStatusChan
		assert.True(t, fetchStatus.ImageAvailable)
	}
	// Release the hounds of war
	close(startGoRoutinesChan)

	// Wait for all requests to complete
	request_wg.Wait()

	// *** Teardown phase ***
	teardownTestServer(ts, app, dir, cancel, wg)
}

// Run the concurrency test for cases that test grace period logic
// Only one goroutine should receive the old image, others should receive the new image
func runIntegrationConcurrencyTest2(t *testing.T, tc testCase, appConfig AppConfig, testImages [][]byte, endpoint string, backendServerOrchestratorChan chan int) {
	ts, dir, ctx, cancel, wg := setupTestServer(tc.backendHTTPHandlerFunc, tc.initialFile)

	type fetchedImageResult struct {
		ImageData   []byte
		HTTPStatus  int
		GoRoutineID int
	}

	app := NewApp(
		dir+"/image.jpg",
		ts.URL,
		appConfig.MaxAge,
		appConfig.GracePeriod,
		appConfig.FetchTimeout,
	)

	fetchStatus, fetchStatusChan := app.StartBackgroundImageFetcher(ctx, wg)

	assert.NoError(t, fetchStatus.Err)

	// *** Setup phase ***

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

	var request_wg sync.WaitGroup
	numParallelRequests := 10

	// Channel to signal goroutines to start
	startGoRoutinesChan := make(chan struct{}, 1)
	// Channel to collect fetched image results
	fetchedImageResultsChan := make(chan fetchedImageResult, numParallelRequests)

	// Start multiple goroutines to make concurrent requests
	for i := 0; i < numParallelRequests; i++ {
		request_wg.Add(1)
		go func() {
			defer request_wg.Done()
			// wait until signaled to start
			<-startGoRoutinesChan
			// random short delay to better simulate real-world concurrent requests
			time.Sleep(time.Duration(200+rand.Intn(200)) * time.Millisecond)
			// Make the request
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// assert.Equal(t, http.StatusOK, w.Code)
			fetchedImageResultsChan <- fetchedImageResult{
				ImageData:   w.Body.Bytes(),
				GoRoutineID: i,
				HTTPStatus:  w.Code,
			}
		}()
	}

	// *** Execution phase ***
	// Prepare the system start
	imageIndex := 0
	if !imageAvailable { // Cold start, need to fetch the first image
		// Trigger image fetch from backend
		app.HeartbeatChan <- struct{}{}

		// Trigger backend server to serve the next image
		backendServerOrchestratorChan <- imageIndex

		// Wait for the image fetch result
		fetchStatus = <-fetchStatusChan
		assert.True(t, fetchStatus.ImageAvailable)
	} else {
		// do nothing
	}

	// make image stale
	app.ImageFetchedFromBackendAt = time.Now().Add(-app.MaxAge).Add(-1 * time.Second)
	// Release the hounds of war
	close(startGoRoutinesChan)
	// Trigger image fetch from backend
	app.HeartbeatChan <- struct{}{}

	// Trigger backend server to serve the next image
	imageIndex = (imageIndex + 1) % len(testImages)
	backendServerOrchestratorChan <- imageIndex

	for range numParallelRequests {
		fetchResult := <-fetchedImageResultsChan
		assert.NotNil(t, fetchResult.ImageData)
		fmt.Println("Goroutine ", fetchResult.GoRoutineID, " fetched image ", string(fetchResult.ImageData))
	}

	// Wait for all requests to complete
	request_wg.Wait()
	close(fetchedImageResultsChan)

	// TODO assert that only one goroutine received the old image, others received the new image

	// *** Teardown phase ***
	teardownTestServer(ts, app, dir, cancel, wg)
}
