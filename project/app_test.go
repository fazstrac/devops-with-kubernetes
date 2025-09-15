package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func NewMockResponse(payload []byte, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(payload)),
		Header:     make(http.Header),
	}
}

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
	app.GetIndex(c)

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
	app.GetImage(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
}

// Test auxiliary functions for image handling

// FetchImage tests
func TestFetchImageSuccess(t *testing.T) {
	// Save original functions to restore after test
	origSaveImage := SaveImageFunc

	// Restore original functions after the test
	defer func() {
		SaveImageFunc = origSaveImage
	}()

	testImage := []byte("This is a test image content")

	SaveImageFunc = func(imagePath string, resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		assert.Equal(t, testImage, body, "Image content should match the test image content")
		return nil
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer ts.Close()

	imagePath := "mockimage.jpg"
	imageUrl := ts.URL

	// Call the fetchImage function to fetch the image from the test server
	// and save it to the temporary directory.
	err := fetchImage(imagePath, imageUrl)
	assert.NoError(t, err, "fetchImage should not return an error")
}

func TestFetchImageSuccessWithBackoff(t *testing.T) {
	// This test checks if the fetchImage function implements the backoff logic correctly.
	// It simulates a server that returns an error for the first few requests and then succeeds.
	// Save original functions to restore after test
	origSaveImage := SaveImageFunc

	// Restore original functions after the test
	defer func() {
		SaveImageFunc = origSaveImage
	}()

	testImage := []byte("This is a test image content")

	SaveImageFunc = func(imagePath string, resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		assert.Equal(t, testImage, body, "Image content should match the test image content")
		return nil
	}

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

	imagePath := "mockimage.jpg"
	imageUrl := ts.URL

	err := fetchImage(imagePath, imageUrl)
	assert.NoError(t, err, "fetchImage should eventually succeed after retries")
}

func TestFetchImageFailBadResponse(t *testing.T) {
	// This test checks if the fetchImage function returns an error when the server
	// doesn't respond correctly.

	// Save original functions to restore after test
	origSaveImage := SaveImageFunc

	// Restore original functions after the test
	defer func() {
		SaveImageFunc = origSaveImage
	}()

	timesCalled := 0

	SaveImageFunc = func(imagePath string, resp *http.Response) error {
		timesCalled++

		return nil
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	imagePath := "mockimage.jpg"
	imageUrl := ts.URL // Use the test server URL that returns 403

	err := fetchImage(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error for invalid URL")
	assert.Equal(t, 0, timesCalled, "SaveImageFunc should not be called for bad response")
}

func TestFetchImageFailBadURL(t *testing.T) {
	// Save original functions to restore after test
	origSaveImage := SaveImageFunc

	// Restore original functions after the test
	defer func() {
		SaveImageFunc = origSaveImage
	}()

	timesCalled := 0

	SaveImageFunc = func(imagePath string, resp *http.Response) error {
		timesCalled++

		return nil
	}

	imagePath := "mockimage.jpg"
	invalidUrl := "http://invalid-url" // This should cause http.Get to fail

	err := fetchImage(imagePath, invalidUrl)
	assert.Error(t, err, "fetchImage should return error for invalid URL")
	assert.Equal(t, 0, timesCalled, "SaveImageFunc should not be called for invalid URL")
}

func TestFetchImageFailAfterRetries(t *testing.T) {
	// This test checks if the fetchImage function returns an error after exhausting all retry attempts.

	origWaitTimes := waitTimes
	waitTimes = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}

	// Save original functions to restore after test
	origSaveImage := SaveImageFunc

	// Restore original functions after the test
	defer func() {
		SaveImageFunc = origSaveImage
		waitTimes = origWaitTimes
	}()

	timesCalled := 0

	SaveImageFunc = func(imagePath string, resp *http.Response) error {
		timesCalled++

		return nil
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // Simulate a permanent failure
	}))
	defer ts.Close()

	imagePath := "mockimage.jpg"
	imageUrl := ts.URL

	err := fetchImage(imagePath, imageUrl)
	assert.Error(t, err, "fetchImage should return an error after all retry attempts")
	assert.Equal(t, 0, timesCalled, "SaveImageFunc should not be called if all attempts fail")
}

// readImage tests

func TestReadImageExistingFile(t *testing.T) {
	// This test checks if the readImage function reads the image file correctly.

	// Save original functions to restore after test
	origReadImage := ReadFileFunc

	// Restore original functions after the test
	defer func() {
		ReadFileFunc = origReadImage
	}()

	testImage := []byte("This is a test image content")

	ReadFileFunc = func(imagePath string) ([]byte, error) {
		return testImage, nil
	}

	imagePath := "mockimage.jpg"
	data, err := readImage(imagePath)
	assert.NoError(t, err, "readImage should not return an error for existing file")
	assert.Equal(t, string(testImage), data, "readImage should return the correct image content")
}

func TestReadImageNonExistentFile(t *testing.T) {
	// This test checks if the readImage function handles non-existent files correctly.

	// Save original functions to restore after test
	origReadImage := ReadFileFunc

	// Restore original functions after the test
	defer func() {
		ReadFileFunc = origReadImage
	}()

	ReadFileFunc = func(imagePath string) ([]byte, error) {
		return []byte{}, os.ErrNotExist
	}

	imagePath := "mockimage.jpg"

	data, err := readImage(imagePath)
	assert.Error(t, err, "readImage should return an error for non-existent file")
	assert.Empty(t, data, "readImage should return empty data for non-existent file")
}

func TestSaveImageSuccess(t *testing.T) {
	// This test checks if the saveImage function saves the image correctly.

	// Save original functions to restore after test
	origCreate := CreateFunc
	origMkdirTemp := MkdirTempFunc
	origRename := RenameFunc
	origRemoveAll := RemoveAllFunc
	origCopy := CopyFunc

	// Restore original functions after the test
	defer func() {
		CreateFunc = origCreate
		MkdirTempFunc = origMkdirTemp
		RenameFunc = origRename
		RemoveAllFunc = origRemoveAll
		CopyFunc = origCopy
	}()

	testImage := []byte("This is a test image content")

	mkdirTempFuncCalledTimes := 0
	MkdirTempFunc = func(dir, pattern string) (string, error) {
		mkdirTempFuncCalledTimes++
		return "tempdir", nil
	}

	createFuncCalledTimes := 0
	CreateFunc = func(imagePath string) (*os.File, error) {
		createFuncCalledTimes++
		assert.Equal(t, "tempdir/image.jpg", imagePath, "CreateFunc should be called with the correct temporary file path")
		return nil, nil
	}

	renameFuncCalledTimes := 0
	RenameFunc = func(oldpath, newpath string) error {
		renameFuncCalledTimes++
		assert.Equal(t, "tempdir/image.jpg", oldpath, "RenameFunc should be called with the correct old path")
		assert.Equal(t, "mockimage.jpg", newpath, "RenameFunc should be called with the correct new path")
		return nil
	}

	removeAllFuncCalledTimes := 0
	RemoveAllFunc = func(path string) error {
		removeAllFuncCalledTimes++
		assert.Equal(t, "tempdir", path, "RemoveAllFunc should be called with the correct temporary directory path")
		return nil
	}

	copyFuncCalledTimes := 0
	CopyFunc = func(dst io.Writer, src io.Reader) (int64, error) {
		copyFuncCalledTimes++

		data, err := io.ReadAll(src)
		if err != nil {
			return 0, err
		}
		assert.Equal(t, testImage, data, "Copied data should match the test image content")
		return int64(len(data)), nil
	}

	// Create a mock HTTP response with the test image content
	resp := NewMockResponse(testImage, http.StatusOK)
	imagePath := "mockimage.jpg"

	err := saveImage(imagePath, resp)
	assert.NoError(t, err, "saveImage should not return an error")
	assert.Equal(t, 1, mkdirTempFuncCalledTimes, "MkdirTempFunc should be called once")
	assert.Equal(t, 1, createFuncCalledTimes, "CreateFunc should be called once")
	assert.Equal(t, 1, renameFuncCalledTimes, "RenameFunc should be called once")
	assert.Equal(t, 1, removeAllFuncCalledTimes, "RemoveAllFunc should be called once")
	assert.Equal(t, 1, copyFuncCalledTimes, "CopyFunc should be called once")
}

func TestSaveImageFailMkdirTemp(t *testing.T) {
	// This test checks if the saveImage function handles MkdirTemp failure correctly.

	// Save original functions to restore after test
	origMkdirTemp := MkdirTempFunc

	// Restore original functions after the test
	defer func() {
		MkdirTempFunc = origMkdirTemp
	}()

	MkdirTempFunc = func(dir, pattern string) (string, error) {
		return "", os.ErrPermission
	}

	testImage := []byte("This is a test image content")
	resp := NewMockResponse(testImage, http.StatusOK)
	imagePath := "mockimage.jpg"

	err := saveImage(imagePath, resp)
	assert.Error(t, err, "saveImage should return an error if MkdirTemp fails")
}

func TestSaveImageFailCreate(t *testing.T) {
	// This test checks if the saveImage function handles Create failure correctly.

	// Save original functions to restore after test
	origCreate := CreateFunc
	origMkdirTemp := MkdirTempFunc
	origRemoveAll := RemoveAllFunc

	// Restore original functions after the test
	defer func() {
		CreateFunc = origCreate
		MkdirTempFunc = origMkdirTemp
		RemoveAllFunc = origRemoveAll
	}()

	MkdirTempFunc = func(dir, pattern string) (string, error) {
		return "tempdir", nil
	}

	CreateFunc = func(imagePath string) (*os.File, error) {
		return nil, os.ErrPermission
	}

	removeAllFuncCalledTimes := 0
	RemoveAllFunc = func(path string) error {
		removeAllFuncCalledTimes++
		assert.Equal(t, "tempdir", path, "RemoveAllFunc should be called with the correct temporary directory path")
		return nil
	}

	testImage := []byte("This is a test image content")
	resp := NewMockResponse(testImage, http.StatusOK)
	imagePath := "mockimage.jpg"

	err := saveImage(imagePath, resp)
	assert.Error(t, err, "saveImage should return an error if Create fails")
	assert.Equal(t, 1, removeAllFuncCalledTimes, "RemoveAllFunc should be called once to clean up the temp directory")
}

func TestSaveImageFailCopy(t *testing.T) {
	// This test checks if the saveImage function handles Copy failure correctly.

	// Save original functions to restore after test
	origCreate := CreateFunc
	origMkdirTemp := MkdirTempFunc
	origRemoveAll := RemoveAllFunc
	origCopy := CopyFunc

	// Restore original functions after the test
	defer func() {
		CreateFunc = origCreate
		MkdirTempFunc = origMkdirTemp
		RemoveAllFunc = origRemoveAll
		CopyFunc = origCopy
	}()

	MkdirTempFunc = func(dir, pattern string) (string, error) {
		return "tempdir", nil
	}

	CreateFunc = func(imagePath string) (*os.File, error) {
		return nil, nil
	}

	copyFuncCalledTimes := 0
	CopyFunc = func(dst io.Writer, src io.Reader) (int64, error) {
		copyFuncCalledTimes++
		return 0, os.ErrInvalid
	}

	removeAllFuncCalledTimes := 0
	RemoveAllFunc = func(path string) error {
		removeAllFuncCalledTimes++
		assert.Equal(t, "tempdir", path, "RemoveAllFunc should be called with the correct temporary directory path")
		return nil
	}

	testImage := []byte("This is a test image content")
	resp := NewMockResponse(testImage, http.StatusOK)
	imagePath := "mockimage.jpg"

	err := saveImage(imagePath, resp)
	assert.Error(t, err, "saveImage should return an error if Copy fails")
	assert.Equal(t, 1, copyFuncCalledTimes, "CopyFunc should be called once")
	assert.Equal(t, 1, removeAllFuncCalledTimes, "RemoveAllFunc should be called once to clean up the temp directory")
}

func TestSaveImageFailRename(t *testing.T) {
	// This test checks if the saveImage function handles Rename failure correctly.

	// Save original functions to restore after test
	origCreate := CreateFunc
	origMkdirTemp := MkdirTempFunc
	origRemoveAll := RemoveAllFunc
	origRename := RenameFunc
	origCopy := CopyFunc

	// Restore original functions after the test
	defer func() {
		CreateFunc = origCreate
		MkdirTempFunc = origMkdirTemp
		RemoveAllFunc = origRemoveAll
		RenameFunc = origRename
		CopyFunc = origCopy
	}()

	MkdirTempFunc = func(dir, pattern string) (string, error) {
		return "tempdir", nil
	}

	CreateFunc = func(imagePath string) (*os.File, error) {
		return nil, nil
	}

	RenameFunc = func(oldpath, newpath string) error {
		return os.ErrInvalid
	}

	copyFuncCalledTimes := 0
	CopyFunc = func(dst io.Writer, src io.Reader) (int64, error) {
		copyFuncCalledTimes++
		return int64(len("This is a test image content")), nil
	}

	removeAllFuncCalledTimes := 0
	RemoveAllFunc = func(path string) error {
		removeAllFuncCalledTimes++
		assert.Equal(t, "tempdir", path, "RemoveAllFunc should be called with the correct temporary directory path")
		return nil
	}

	testImage := []byte("This is a test image content")
	resp := NewMockResponse(testImage, http.StatusOK)
	imagePath := "mockimage.jpg"

	err := saveImage(imagePath, resp)
	assert.Error(t, err, "saveImage should return an error if Rename fails")
	assert.Equal(t, 1, copyFuncCalledTimes, "CopyFunc should be called once")
	assert.Equal(t, 1, removeAllFuncCalledTimes, "RemoveAllFunc should be called once to clean up the temp directory")
}
