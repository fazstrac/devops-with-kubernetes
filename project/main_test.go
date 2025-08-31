package main

import (
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

func TestEndpointGetIndex(t *testing.T) {
	app := NewApp(
		"./cache/image.jpg",
		"https://picsum.photos/1200",
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)

	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndpointGetImageSuccess(t *testing.T) {
	testImage := []byte("This is a test image content")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer ts.Close()

	// This test checks if the getImage handler works correctly.
	// It creates a temporary directory for the image and checks if the response is correct.
	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(
		dir+"/image.jpg", // Use a temporary image path for testing
		ts.URL,           // Use an invalid URL to simulate fetch error
		10*time.Minute,   // Set a reasonable max age for the image
		1*time.Minute,    // Grace period during which the old image can be fetched _once_
		30*time.Second,   // Timeout for fetching the image from the backend
	)

	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/images/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
}

func TestEndpointGetImageFailBadResponse(t *testing.T) {
	// This test checks if the getImage handler returns an error when fetching the image fails.
	// It uses a temporary directory for the image and simulates a fetch error.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_error_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(
		dir+"/image.jpg",
		ts.URL,
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)

	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/images/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEndpointGetImageFailBadURL(t *testing.T) {
	// This test checks if the getImage handler returns an error when fetching the image fails.
	// It uses a temporary directory for the image and simulates a fetch error.

	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_error_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(
		dir+"/image.jpg",
		"http://invalid-url/",
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/images/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEndPointGetImageConcurrentSuccess(t *testing.T) {
	fetchTimeout := 15 * time.Second
	serveWait := max(fetchTimeout-5*time.Second, 1*time.Second) // Ensure serveWait is positive

	var wg sync.WaitGroup

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serveWait) // Simulate a long fetch time
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is a test image content"))
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_timeout_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(dir+"/image.jpg", ts.URL, 10*time.Minute, 1*time.Nanosecond, fetchTimeout)
	router := setupRouter(app) // Use the same router logic as in main.go

	// Start first request (will block for serveWait seconds
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/images/image.jpg", nil)
		router.ServeHTTP(w, req)

		if err != nil {
			t.Errorf("First request failed: %v", err)
			return
		}

		assert.Equal(t, http.StatusOK, w.Code)
	}()

	// Wait a moment to ensure first request grabs the lock
	time.Sleep(1 * time.Second)

	// Start second request (should timeout after 30s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/images/image.jpg", nil)
		router.ServeHTTP(w, req)

		if err != nil {
			t.Errorf("Second request failed: %v", err)
			return
		}

		assert.Equal(t, http.StatusOK, w.Code)
	}()

	wg.Wait()
}

func TestEndPointGetImageConcurrentFailTimeout(t *testing.T) {
	fetchTimeout := 15 * time.Second
	serveWait := fetchTimeout + 5*time.Second

	var wg sync.WaitGroup

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serveWait) // Simulate a long fetch time
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is a test image content"))
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_timeout_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(dir+"/image.jpg", ts.URL, 10*time.Minute, 1*time.Nanosecond, fetchTimeout)
	router := setupRouter(app) // Use the same router logic as in main.go

	// Start first request (will block for 35s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/images/image.jpg", nil)
		router.ServeHTTP(w, req)

		if err != nil {
			t.Errorf("First request failed: %v", err)
			return
		}

		assert.Equal(t, http.StatusOK, w.Code)
	}()

	// Wait a moment to ensure first request grabs the lock
	time.Sleep(1 * time.Second)

	// Start second request (should timeout after 30s)
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/images/image.jpg", nil)
		router.ServeHTTP(w, req)

		if err != nil {
			t.Errorf("Second request failed: %v", err)
			return
		}
		duration := time.Since(start)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		if duration < fetchTimeout {
			t.Errorf("Expected timeout after ~%v, got %v", fetchTimeout, duration)
		}
	}()

	wg.Wait()
}
