package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
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

	assert.Equal(t, 2, len(router.Routes())) // We have two routes defined
	assert.NotNil(t, router)
}

// Test app startup and initial image fetching
// Uses httptest.Server to mock backend image server, file system operations are not mocked
func TestStartupCases(t *testing.T) {
	// This test checks if the application starts correctly with various configurations.
	// It does not start the HTTP server, just initializes the App struct and calls LoadCachedImage.
	testImage := []byte("This is a test image content")

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	type testCase struct {
		name         string
		setupFunc    func() (ts *httptest.Server, dir string, ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup)
		teardownFunc func(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup)
		isColdStart  bool
		expectErr    bool
	}

	testCases := []testCase{
		{
			name: "success cold start image not present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc, *sync.WaitGroup) {
				ctx, cancel := context.WithCancel(context.Background())
				wg := &sync.WaitGroup{}

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				return ts, dir, ctx, cancel, wg
			},
			teardownFunc: func(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
				wg.Wait()
				close(app.HeartbeatChan)
			},
			isColdStart: true,
			expectErr:   false,
		},
		{
			name: "failure cold start image not present fetch timeout",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc, *sync.WaitGroup) {
				ctx, cancel := context.WithCancel(context.Background())
				wg := &sync.WaitGroup{}

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(2 * FetchImageTimeout) // Trigger a timeout
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				return ts, dir, ctx, cancel, wg
			},
			teardownFunc: func(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
				wg.Wait()
				close(app.HeartbeatChan)
			},
			isColdStart: true,
			expectErr:   true,
		},
		{
			name: "success cold start image not present first fetch timeout",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc, *sync.WaitGroup) {
				ctx, cancel := context.WithCancel(context.Background())
				wg := &sync.WaitGroup{}

				counter := 0

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if counter == 0 {
						time.Sleep(2 * FetchImageTimeout) // Trigger a timeout
						counter++
					}
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				return ts, dir, ctx, cancel, wg
			},
			teardownFunc: func(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
				wg.Wait()
				close(app.HeartbeatChan)
			},
			isColdStart: true,
			expectErr:   false,
		},
		{
			name: "success warm start image present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc, *sync.WaitGroup) {
				ctx, cancel := context.WithCancel(context.Background())
				wg := &sync.WaitGroup{}

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					t.Fatal("Backend should not be called")
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				err := os.WriteFile(dir+"/image.jpg", testImage, 0644)
				if err != nil {
					t.Fatalf("Failed to write test image: %v", err)
				}

				return ts, dir, ctx, cancel, wg
			},
			teardownFunc: func(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
				wg.Wait()
				close(app.HeartbeatChan)
			},
			isColdStart: false,
			expectErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Println("Running test case:", tc.name)
			ts, dir, ctx, cancel, wg := tc.setupFunc()

			app := NewApp(
				dir+"/image.jpg",  // Use a temporary image path for testing
				ts.URL,            // Use the test server URL as the backend
				20*time.Second,    // Set a reasonable max age for the image
				1*time.Minute,     // Grace period during which the old image can be fetched _once_
				FetchImageTimeout, // Timeout for fetching the image from the backend
			)

			fetchStatusChan := make(chan FetchResult)

			wg.Add(1)
			go app.ImageFetcher(ctx, fetchStatusChan, wg)
			defer func() {
				<-fetchStatusChan // Ensure we read the fetch result to avoid goroutine leak
				close(fetchStatusChan)
			}()

			var fetchStatus FetchResult

			if tc.isColdStart {
				// Wait for the image check result
				fetchStatus = <-fetchStatusChan
				assert.False(t, fetchStatus.ImageAvailable)
				// Wait for the first image fetch result
				fetchStatus = <-fetchStatusChan
			} else {
				// Wait for the image check result
				fetchStatus = <-fetchStatusChan
				assert.True(t, fetchStatus.ImageAvailable)
			}

			if tc.expectErr {
				assert.False(t, fetchStatus.ImageAvailable, "Should not have an image")
				assert.Error(t, fetchStatus.Err, "Expected fetch error but got none")
			} else {
				assert.True(t, fetchStatus.ImageAvailable, "Should have an image")
				assert.NoError(t, fetchStatus.Err, "Did not expect fetch error but got one")
			}

			tc.teardownFunc(ts, app, dir, cancel, wg)
		})
	}
}
