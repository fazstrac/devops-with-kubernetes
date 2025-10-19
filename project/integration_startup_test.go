package main

import (
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test application's endpoints and router setup

func TestSetupRouter(t *testing.T) {
	port := strconv.Itoa(rand.Intn(9000) + 1000)
	os.Setenv("PORT", port)

	app := NewApp(
		"./cache/image.jpg",
		"https://picsum.photos/1200",
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)
	router := setupRouter(app)

	assert.Equal(t, 4, len(router.Routes())) // We have four routes defined
	assert.NotNil(t, router)
}

// Test app startup and initial image fetching
// Uses httptest.Server to mock backend image server, file system operations are not mocked
func TestStartupCases(t *testing.T) {
	// This test checks if the application starts correctly with various configurations.
	// It does not start the HTTP server, just initializes the App struct and calls LoadCachedImage.
	testImage := []byte("This is a test image content")

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	testCases := []testCase{
		{
			name: "success cold start image not present",
			backendHTTPHandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				w.WriteHeader(http.StatusOK)
				w.Write(testImage)
			}),
			isColdStart: true,
			initialFile: nil,
			expectErr:   false,
		},
		{
			name: "success warm start image present",
			backendHTTPHandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				t.Fatal("Backend should not be called")
			}),
			isColdStart: false,
			initialFile: testImage,
			expectErr:   false,
		},
		{
			name: "success cold start image not present first fetch timeout",
			backendHTTPHandlerFunc: func() http.HandlerFunc {
				counter := 0 // closure to keep state
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if counter == 0 {
						time.Sleep(2 * FetchImageTimeout) // Trigger a timeout
						counter++
					}
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				})
			}(),
			isColdStart: true,
			initialFile: nil,
			expectErr:   false,
		},
		{
			name: "failure cold start image not present fetch timeout",
			backendHTTPHandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * FetchImageTimeout) // Trigger a timeout
				w.Header().Set("Content-Type", "image/jpeg")
				w.WriteHeader(http.StatusOK)
				w.Write(testImage)
			}),
			isColdStart: true,
			initialFile: nil,
			expectErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runIntegrationStartupTest1(t, tc, FetchImageTimeout)
		})
	}
}

func runIntegrationStartupTest1(t *testing.T, tc testCase, fetchTimeout time.Duration) {
	ts, dir, ctx, cancel, wg := setupTestServer(tc.backendHTTPHandlerFunc, tc.initialFile)

	app := NewApp(
		dir+"/image.jpg", // Use a temporary image path for testing
		ts.URL,           // Use the test server URL as the backend
		20*time.Second,   // Set a reasonable max age for the image
		1*time.Minute,    // Grace period during which the old image can be fetched _once_
		fetchTimeout,     // Timeout for fetching the image from the backend
	)

	fetchStatus, fetchStatusChan := app.StartBackgroundImageFetcher(ctx, wg)
	assert.NoError(t, fetchStatus.Err)

	if tc.isColdStart {
		assert.False(t, fetchStatus.ImageAvailable)

		// Trigger the heartbeat to start the fetch process
		app.HeartbeatChan <- struct{}{}
		// Wait for the first image fetch result
		fetchStatus = <-fetchStatusChan
	}

	if tc.expectErr {
		assert.False(t, fetchStatus.ImageAvailable, "Should not have an image")
		assert.Error(t, fetchStatus.Err, "Expected fetch error but got none")
	} else {
		assert.True(t, fetchStatus.ImageAvailable, "Should have an image")
		assert.NoError(t, fetchStatus.Err, "Did not expect fetch error but got one")
	}

	teardownTestServer(ts, app, dir, cancel, wg)
}
