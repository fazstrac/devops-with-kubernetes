package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test application's endpoints. Mock only the backend server
// Uses httptest.Server to mock backend image server, file system operations are not mocked

// This test check for successes in the initial image fetch and
// that the backend is not called if there is a fresh image on startup
// TODO: Add failures
func TestIntegrationGetImageCases1(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
	}

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	endpoint := "/images/image.jpg"

	type testCase struct {
		name         string
		setupFunc    func() (ts *httptest.Server, dir string, ctx context.Context, cancel context.CancelFunc)
		teardownFunc func(ts *httptest.Server, dir string, cancel context.CancelFunc)
		expectdCode  int
		isColdStart  bool
		expectErr    bool
	}

	testCases := []testCase{
		{
			name: "success cold start image not present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImages[0])
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				return ts, dir, ctx, cancel
			},
			teardownFunc: func(ts *httptest.Server, dir string, cancel context.CancelFunc) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
			},
			expectdCode: http.StatusOK,
			isColdStart: true,
			expectErr:   false,
		},
		{
			name: "success warm start image present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					t.Fatal("Backend should not be called")
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				err := os.WriteFile(dir+"/image.jpg", testImages[0], 0644)
				if err != nil {
					t.Fatalf("Failed to write test image: %v", err)
				}

				return ts, dir, ctx, cancel
			},
			teardownFunc: func(ts *httptest.Server, dir string, cancel context.CancelFunc) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
			},
			isColdStart: false,
			expectErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts, dir, ctx, cancel := tc.setupFunc()

			app := NewApp(
				dir+"/image.jpg",  // Use a temporary image path for testing
				ts.URL,            // Use the test server URL as the backend
				20*time.Second,    // Set a reasonable max age for the image
				1*time.Minute,     // Grace period during which the old image can be fetched _once_
				FetchImageTimeout, // Timeout for fetching the image from the backend
			)

			fetchStatusChan := make(chan FetchResult)
			go app.Fetcher(ctx, fetchStatusChan)
			defer func() {
				<-fetchStatusChan // Ensure we read the fetch result to avoid goroutine leak
				close(fetchStatusChan)
			}()

			var fetchStatus FetchResult

			if tc.isColdStart {
				// Wait for the image check result
				fetchStatus = <-fetchStatusChan
				assert.False(t, fetchStatus.ImageAvailable)
				// Wait for the first image fetch resul
				fetchStatus = <-fetchStatusChan
			} else {
				// Wait for the image check result
				fetchStatus = <-fetchStatusChan
				assert.True(t, fetchStatus.ImageAvailable)
			}
			assert.Equal(t, tc.expectErr, fetchStatus.Err != nil, "Fetch status error mismatch")

			router := setupRouter(app)
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			body := w.Body.Bytes()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, testImages[0], body)

			tc.teardownFunc(ts, dir, cancel)
		})
	}
}

// This test tests that the image does get automatically refreshed
// It deliberately waits for the next image, and does not care about the
// grace period.
// TODO: Add failures
func TestIntegrationGetImageCases2(t *testing.T) {
	testImages := [][]byte{
		[]byte("This is a test image content1"),
		[]byte("This is a test image content2"),
		[]byte("This is a test image content3"),
	}

	FetchImageTimeout := 1 * time.Second // Set a short timeout for testing

	endpoint := "/images/image.jpg"

	type testCase struct {
		name         string
		setupFunc    func() (ts *httptest.Server, dir string, ctx context.Context, cancel context.CancelFunc)
		teardownFunc func(ts *httptest.Server, dir string, cancel context.CancelFunc)
		expectdCode  int
		isColdStart  bool
		expectErr    bool
	}

	testCases := []testCase{
		{
			name: "success cold start image not present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())

				// Serve different images on subsequent calls
				counter := 0
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImages[counter])
					counter++
				}))

				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				return ts, dir, ctx, cancel
			},
			teardownFunc: func(ts *httptest.Server, dir string, cancel context.CancelFunc) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
			},
			expectdCode: http.StatusOK,
			isColdStart: true,
			expectErr:   false,
		},
		{
			name: "success warm start image present",
			setupFunc: func() (*httptest.Server, string, context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())

				counter := 1
				// Serve different images on subsequent calls
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if counter == 0 {
						w.WriteHeader(http.StatusForbidden)
						t.Fatal("Backend should not be called on first request")
					} else {
						w.Header().Set("Content-Type", "image/jpeg")
						w.WriteHeader(http.StatusOK)
						w.Write(testImages[counter])
					}
					counter++
				}))
				dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
				err := os.WriteFile(dir+"/image.jpg", testImages[0], 0644)
				if err != nil {
					t.Fatalf("Failed to write test image: %v", err)
				}

				return ts, dir, ctx, cancel
			},
			teardownFunc: func(ts *httptest.Server, dir string, cancel context.CancelFunc) {
				cancel()
				ts.Close()
				os.RemoveAll(dir)
			},
			isColdStart: false,
			expectErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts, dir, ctx, cancel := tc.setupFunc()

			app := NewApp(
				dir+"/image.jpg",  // Use a temporary image path for testing
				ts.URL,            // Use the test server URL as the backend
				5*time.Second,     // Set a reasonable max age for the image
				1*time.Minute,     // Grace period during which the old image can be fetched _once_
				FetchImageTimeout, // Timeout for fetching the image from the backend
			)

			fetchStatusChan := make(chan FetchResult)
			go app.Fetcher(ctx, fetchStatusChan)
			defer func() {
				<-fetchStatusChan // Ensure we read the fetch result to avoid goroutine leak
				close(fetchStatusChan)
			}()

			var fetchStatus FetchResult

			assert.Equal(t, tc.expectErr, fetchStatus.Err != nil, "Fetch status error mismatch")

			router := setupRouter(app)

			// Wait for the initial image file check
			fetchStatus = <-fetchStatusChan

			for i := range len(testImages) {
				// If image wasn't available in previous check, wait for it
				if !fetchStatus.ImageAvailable {
					fetchStatus = <-fetchStatusChan
				}

				req := httptest.NewRequest("GET", endpoint, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				resp := w.Result()
				body := w.Body.Bytes()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, testImages[i], body)

				// Mark image not available to force waiting for fresh image
				fetchStatus.ImageAvailable = false
			}

			tc.teardownFunc(ts, dir, cancel)
		})
	}
}
