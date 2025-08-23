package main

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
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

	app := &App{
		ImagePath: "./cache/image.jpg",
		ImageUrl:  "https://picsum.photos/1200",
		MaxAge:    10 * time.Minute,
	}
	router := setupRouter(app)

	assert.Equal(t, 2, len(router.Routes())) // We have two routes defined
	assert.NotNil(t, router)
}

func TestEndpointGetIndex(t *testing.T) {
	app := &App{
		ImagePath: "./cache/image.jpg", // Use a temporary image path for testing
		MaxAge:    10 * time.Minute,    // Set a reasonable max age for the image
	}

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

	app := &App{
		ImagePath: dir + "/image.jpg", // Use a temporary image path for testing
		ImageUrl:  ts.URL,             // Use an invalid URL to simulate fetch error
		MaxAge:    10 * time.Minute,   // Set a reasonable max age for the image
	}
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/image.jpg", nil)
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

	app := &App{
		ImagePath: dir + "/image.jpg", // Use a temporary image path for testing
		ImageUrl:  ts.URL,             // Use an invalid URL to simulate fetch error
		MaxAge:    10 * time.Minute,   // Set a reasonable max age for the image
	}
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEndpointGetImageFailBadURL(t *testing.T) {
	// This test checks if the getImage handler returns an error when fetching the image fails.
	// It uses a temporary directory for the image and simulates a fetch error.

	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_error_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := &App{
		ImagePath: dir + "/image.jpg",    // Use a temporary image path for testing
		ImageUrl:  "http://invalid-url/", // Use an invalid URL to simulate fetch error
		MaxAge:    10 * time.Minute,      // Set a reasonable max age for the image
	}
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
