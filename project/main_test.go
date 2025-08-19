package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test endpoints for the application

func TestGetIndex(t *testing.T) {
	app := &App{
		ImagePath: "./cache/image.jpg", // Use a temporary image path for testing
	}
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetImage(t *testing.T) {
	// This test checks if the getImage handler works correctly.
	// It creates a temporary directory for the image and checks if the response is correct.
	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := &App{
		ImagePath: dir + "/image.jpg", // Use a temporary image path for testing
	}
	router := setupRouter(app) // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/image.jpg", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
}

// Test auxiliary functions for image handling

func TestFetchImage(t *testing.T) {
	// This test assumes that the fetchImage function is implemented correctly
	// and that it can be called without any side effects.

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	err = fetchImage(imagePath)
	assert.NoError(t, err, "fetchImage should not return an error")
	assert.FileExists(t, imagePath, "Image should be fetched and saved in cache directory")
}

func TestIsImageFreshNewImage(t *testing.T) {
	// This test checks if the image freshness logic works correctly.
	// It creates a temporary file and checks if it is considered fresh.

	dir, err := os.MkdirTemp(os.TempDir(), "test_is_image_fresh1_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	_, err = os.Create(imagePath)
	assert.NoError(t, err, "Failed to create image file for freshness test")

	fresh := isImageFresh(imagePath, 10*time.Minute)
	assert.True(t, fresh, "Newly created image should be considered fresh")
}

func TestIsImageFreshOldImage(t *testing.T) {
	// This test checks if the image freshness logic works correctly.
	// It creates a temporary file and checks if it is considered fresh.

	dir, err := os.MkdirTemp(os.TempDir(), "test_is_image_fresh2_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	_, err = os.Create(imagePath)
	assert.NoError(t, err, "Failed to create image file for freshness test")

	// Simulate an old image by modifying its modification time
	err = os.Chtimes(imagePath, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour))
	assert.NoError(t, err, "Failed to set modification time for image file")

	fresh := isImageFresh(imagePath, 10*time.Minute)
	assert.False(t, fresh, "Old image should not be considered fresh")
}

func TestIsImageFreshNonExistentImage(t *testing.T) {
	// This test checks if the image freshness logic works correctly for a non-existent image.

	dir, err := os.MkdirTemp(os.TempDir(), "test_is_image_fresh3_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/non_existent_image.jpg"

	fresh := isImageFresh(imagePath, 10*time.Minute)
	assert.False(t, fresh, "Non-existent image should not be considered fresh")
}

func TestReadImageExistingFile(t *testing.T) {
	// This test checks if the readImage function reads the image file correctly.

	dir, err := os.MkdirTemp(os.TempDir(), "test_read_image1_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	content := []byte("This is a test image content")
	err = os.WriteFile(imagePath, content, 0644)
	assert.NoError(t, err, "Failed to write test image content")

	data, err := readImage(imagePath)
	assert.NoError(t, err, "readImage should not return an error")
	assert.Equal(t, string(content), data, "readImage should return the correct image content")
}

func TestReadImageNonExistentFile(t *testing.T) {
	// This test checks if the readImage function handles non-existent files correctly.

	dir, err := os.MkdirTemp(os.TempDir(), "test_read_image2_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/non_existent_image.jpg"

	data, err := readImage(imagePath)
	assert.Error(t, err, "readImage should return an error for non-existent file")
	assert.Empty(t, data, "readImage should return empty data for non-existent file")
}
