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

func TestConcurrentImageRequests(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
		[]byte("This is a test image content2"),
		[]byte("This is a test image content3"),
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
				w.WriteHeader(http.StatusOK)
				w.Write(testImages[index])
			},
			initialFile:      testImages[0],
			expectedHTTPCode: http.StatusOK,
			expectErr:        false,
		},
	}

	for _, tc := range testCases {
		runIntegrationConcurrencyTest1(t, tc, appConfig, endpoint, backendServerOrchestratorChan)
	}
}

// Runs the integration test for a given test case for cases that do not test grace period logic
func runIntegrationConcurrencyTest1(t *testing.T, tc testCase, appConfig AppConfig, endpoint string, backendServerOrchestratorChan chan int) {
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
			fmt.Printf("Goroutine %s making request to %s\n", strconv.Itoa(i), endpoint)
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
