package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Test endpoints for the application

func TestGetIndexSuccess(t *testing.T) {
	app := NewApp(
		"./cache/image.jpg",
		"https://picsum.photos/1200",
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)

	w := httptest.NewRecorder()
	c, eng := gin.CreateTestContext(w)
	eng.LoadHTMLGlob("templates/*")
	assert.NotNil(t, c)
	app.getIndex(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetImageSuccess(t *testing.T) {
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
		ts.URL,           // Use the test server URL
		10*time.Minute,   // Set a reasonable max age for the image
		1*time.Minute,    // Grace period during which the old image can be fetched _once_
		30*time.Second,   // Timeout for fetching the image from the backend
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	app.getImage(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
}

// Test auxiliary functions for image handling

// FetchImage tests
func TestFetchImageSuccess(t *testing.T) {
	testImage := []byte("This is a test image content")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	imageUrl := ts.URL

	// Call the fetchImage function to fetch the image from the test server
	// and save it to the temporary directory.
	err = fetchImageUnlocked(imagePath, imageUrl)
	assert.NoError(t, err, "fetchImage should not return an error")
	assert.FileExists(t, imagePath, "Image should be fetched and saved in cache directory")

	// Verify the content of the fetched image
	content, err := os.ReadFile(imagePath)
	assert.NoError(t, err, "Failed to read fetched image file")
	assert.Equal(t, testImage, content, "Fetched image content should match the test image content")
}

func TestFetchImageSuccessWithBackoff(t *testing.T) {
	// This test checks if the fetchImage function implements the backoff logic correctly.
	// It simulates a server that returns an error for the first few requests and then succeeds.

	testImage := []byte("This is a test image content")

	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable) // Simulate a temporary failure
			attempts++
		} else {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write(testImage) // Return the test image content after a few attempts
		}
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_backoff_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	imageUrl := ts.URL

	err = fetchImageUnlocked(imagePath, imageUrl)
	assert.NoError(t, err, "fetchImage should eventually succeed after retries")
	assert.FileExists(t, imagePath, "Image should be fetched and saved in cache directory")

	content, err := os.ReadFile(imagePath)
	assert.NoError(t, err, "Failed to read fetched image file")
	assert.Equal(t, testImage, content, "Fetched image content should match the expected content")
}

func TestFetchImageFailBadResponse(t *testing.T) {
	// This test checks if the fetchImage function returns an error when the server
	// doesn't respond correctly.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_error_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	imageUrl := ts.URL // Use the test server URL that returns 403

	err = fetchImageUnlocked(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error for invalid URL")
	assert.NoFileExists(t, imagePath, "Image file should not be created for invalid URL")
}

func TestFetchImageFailBadURL(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_httpget_*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	imagePath := dir + "/image.jpg"
	invalidUrl := "http://invalid-url" // This should cause http.Get to fail

	err = fetchImageUnlocked(imagePath, invalidUrl)
	assert.Error(t, err, "fetchImage should return error for invalid URL")
	assert.NoFileExists(t, imagePath, "Image file should not be created for invalid URL")
}

func TestFetchImageFailAfterRetries(t *testing.T) {
	// This test checks if the fetchImage function returns an error after exhausting all retry attempts.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // Simulate a permanent failure
	}))
	defer ts.Close()

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_failure_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	imageUrl := ts.URL

	err = fetchImageUnlocked(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error after all retry attempts")
	assert.NoFileExists(t, imagePath, "Image file should not be created for permanent failure")
}

func TestFetchImageFailCreateFile(t *testing.T) {
	// This test checks if the fetchImage function returns an error when it fails to create the file.

	testImage := []byte("This is a test image content")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer ts.Close()

	// Use an invalid directory path to simulate file creation failure
	invalidDir := "/invalid_directory_path"
	imagePath := invalidDir + "/image.jpg"
	imageUrl := ts.URL

	err := fetchImageUnlocked(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error when failing to create file")
	assert.NoFileExists(t, imagePath, "Image file should not be created in invalid directory")
}

// readImage tests

func TestReadImageExistingFile(t *testing.T) {
	// This test checks if the readImage function reads the image file correctly.

	dir, err := os.MkdirTemp(os.TempDir(), "test_read_image1_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	content := []byte("This is a test image content")
	err = os.WriteFile(imagePath, content, 0644)
	assert.NoError(t, err, "Failed to write test image content")

	data, err := readImageUnlocked(imagePath)
	assert.NoError(t, err, "readImage should not return an error")
	assert.Equal(t, string(content), data, "readImage should return the correct image content")
}

func TestReadImageNonExistentFile(t *testing.T) {
	// This test checks if the readImage function handles non-existent files correctly.

	dir, err := os.MkdirTemp(os.TempDir(), "test_read_image2_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/non_existent_image.jpg"

	data, err := readImageUnlocked(imagePath)
	assert.Error(t, err, "readImage should return an error for non-existent file")
	assert.Empty(t, data, "readImage should return empty data for non-existent file")
}
