package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestMain does global setup for tests.
// Per-test files and routers are created by helpers to ensure isolation.
func TestMain(m *testing.M) {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	os.Setenv("PORT", "8080")

	os.Exit(m.Run())
}

// setupTestRouter creates a temporary counter file and returns a router
// that uses that file. The temp file is removed automatically when the
// test finishes via t.Cleanup.
func setupTestRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	dir := t.TempDir()
	fname := filepath.Join(dir, "counter.txt")
	// ensure file exists (initCounter expects a path)
	if err := os.WriteFile(fname, []byte("0"), 0644); err != nil {
		t.Fatalf("failed to create counter file: %v", err)
	}

	router := setupRouter(fname)

	// ensure global counter is reset to the file's value for isolation
	counter = initCounter(fname)

	return router, fname
}

func TestInitCounterNoFile(t *testing.T) {
	// Test the counter initialization functionality
	fname := filepath.Join(t.TempDir(), "noexist.txt")
	// Ensure the file does not exist
	if _, err := os.Stat(fname); err == nil {
		os.Remove(fname)
	}

	// initCounter should return 0 when file is missing
	counter = initCounter(fname)
	assert.Equal(t, 0, counter)
}

func TestInitCounterWithFile(t *testing.T) {
	// Test the counter initialization functionality with an existing file
	fname := filepath.Join(t.TempDir(), "counter.txt")
	// Create a file with a specific counter value
	err := os.WriteFile(fname, []byte("5"), 0644)
	assert.NoError(t, err)

	// Check if the counter is initialized to the value in the file
	data, err := os.ReadFile(fname)
	assert.NoError(t, err)
	assert.Equal(t, "5", string(data))

	counter = initCounter(fname)
	assert.Equal(t, 5, counter)
}

func TestInitCounterWithInvalidFile(t *testing.T) {
	// Test the counter initialization functionality with an invalid file
	// Note that this test is expected to reset the counter to 0
	fname := filepath.Join(t.TempDir(), "counter.txt")
	// Create a file with invalid content
	err := os.WriteFile(fname, []byte("invalid"), 0644)
	assert.NoError(t, err)

	// Check if the counter is reset to 0 due to invalid content
	counter = initCounter(fname)
	assert.Equal(t, 0, counter)
}

func TestIncrCounter(t *testing.T) {
	// Test the counter increment functionality
	// Use a temp file for isolation
	fname := filepath.Join(t.TempDir(), "counter.txt")

	counterMutex.Lock()
	counter = 0
	counterMutex.Unlock()

	result := incrCounter(fname)
	assert.Equal(t, "pong 1", result)

	result = incrCounter(fname)
	assert.Equal(t, "pong 2", result)

	// Check if the file was created and contains the correct value
	data, err := os.ReadFile(fname)
	assert.NoError(t, err)
	assert.Equal(t, "2", string(data))
}

// Integration tests for the HTTP endpoints

func TestIncrCounterEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pingpong", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong 1", w.Body.String())

}

func TestPongsNoPingEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pongs", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "0", w.Body.String())
}

func TestPongsOnePingEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pongs", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "0", w.Body.String())

	// Now send a ping to increment the counter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/pingpong", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong 1", w.Body.String())

	// Now check the pongs endpoint again
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/pongs", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "1", w.Body.String())
}
