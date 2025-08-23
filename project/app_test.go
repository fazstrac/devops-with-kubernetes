package main

import (
	"fmt"
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
	app := &App{
		ImagePath: "./cache/image.jpg",
		ImageUrl:  "https://picsum.photos/1200",
		MaxAge:    10 * time.Minute,
	}

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

	app := &App{
		ImagePath: dir + "/image.jpg", // Use a temporary image path for testing
		ImageUrl:  ts.URL,             // Use the test server URL
		MaxAge:    10 * time.Minute,   // Set a reasonable max age for the image
	}

	fmt.Println("Image URL:", app.ImageUrl) // Debugging output to verify the URL

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	app.getImage(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
}

// func TestGetImageFail(t *testing.T) {
// 	// This test checks if the getImage handler returns an error when fetching the image fails.
// 	// It uses a temporary directory for the image and simulates a fetch error.
// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Content-Type", "image/jpeg")
// 		w.WriteHeader(http.StatusNotFound)
// 	}))
// 	defer ts.Close()

// 	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_error_*")
// 	assert.NoError(t, err, "Failed to create temporary directory for test")
// 	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

// 	app := &App{
// 		ImagePath: dir + "/image.jpg", // Use a temporary image path for testing
// 		ImageUrl:  ts.URL,             // Use an invalid URL to simulate fetch error
// 		MaxAge:    10 * time.Minute,   // Set a reasonable max age for the image
// 	}

// 	w := httptest.NewRecorder()
// 	c, _ := gin.CreateTestContext(w)
// 	app.getImage(c)

// 	assert.Equal(t, http.StatusInternalServerError, w.Code)
// }

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

	// This test assumes that the fetchImage function is implemented correctly
	// and that it can be called without any side effects.

	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	imagePath := dir + "/image.jpg"
	imageUrl := ts.URL

	// Call the fetchImage function to fetch the image from the test server
	// and save it to the temporary directory.
	err = fetchImage(imagePath, imageUrl)
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

	err = fetchImage(imagePath, imageUrl)
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

	err = fetchImage(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error for invalid URL")
	assert.NoFileExists(t, imagePath, "Image file should not be created for invalid URL")
}

func TestFetchImageFailBadURL(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "test_fetch_image_httpget_*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	imagePath := dir + "/image.jpg"
	invalidUrl := "http://invalid-url" // This should cause http.Get to fail

	err = fetchImage(imagePath, invalidUrl)
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

	err = fetchImage(imagePath, imageUrl)
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

	err := fetchImage(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error when failing to create file")
	assert.NoFileExists(t, imagePath, "Image file should not be created in invalid directory")
}

// isImageFresh tests

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
