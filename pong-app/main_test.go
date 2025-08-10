package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var testRouter *gin.Engine

func TestMain(m *testing.M) {
	fp, err := os.CreateTemp("/tmp", "pingpong_app_test1_*")
	if err != nil {
		panic("TestMain: Failed to create temporary file: " + err.Error())
	}
	defer os.Remove(fp.Name())

	os.Setenv("PORT", "8080")
	testRouter = setupRouter(fp.Name())
	os.Exit(m.Run())
}

func TestInitCounterNoFile(t *testing.T) {
	// Test the counter initialization functionality

	fname := "thisfileshouldnotexist.txt"
	// Ensure the file does not exist before the test
	if _, err := os.Stat(fname); err == nil {
		os.Remove(fname)
	}

	counter = initCounter(fname)
	assert.Equal(t, 0, counter)

	// Check if the counter is initialized to 0
	assert.Equal(t, 0, counter)
}

func TestInitCounterWithFile(t *testing.T) {
	// Test the counter initialization functionality with an existing file
	fp, err := os.CreateTemp("/tmp", "pingpong_app_test2_*")
	if err != nil {
		panic("Test2: Failed to create temporary file: " + err.Error())
	}
	defer os.Remove(fp.Name())

	fname := fp.Name()

	// Create a file with a specific counter value
	err = os.WriteFile(fname, []byte("5"), 0644)
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
	fp, err := os.CreateTemp("/tmp", "pingpong_app_test3_*")
	if err != nil {
		panic("Test3: Failed to create temporary file: " + err.Error())
	}
	defer os.Remove(fp.Name())

	fname := fp.Name()

	// Create a file with invalid content
	err = os.WriteFile(fname, []byte("invalid"), 0644)
	assert.NoError(t, err)

	// Check if the counter is reset to 0 due to invalid content
	counter = initCounter(fname)
	assert.Equal(t, 0, counter)
}

func TestIncrCounter(t *testing.T) {
	// Test the counter increment functionality
	counterMutex.Lock()
	counter = 0
	counterMutex.Unlock()

	fname := "testfile.txt"
	result := incrCounter(fname)
	defer os.Remove(fname) // Clean up the file after the test

	assert.Equal(t, "pong 1", result)

	result = incrCounter(fname)
	assert.Equal(t, "pong 2", result)

	// Check if the file was created and contains the correct value
	data, err := os.ReadFile(fname)
	assert.NoError(t, err)
	assert.Equal(t, "2", string(data))
}

func TestIncrCounterEndpoint(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pingpong", nil)
	testRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
