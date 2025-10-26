package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var testRouter *gin.Engine

func TestMain(m *testing.M) {
	fp, err := os.CreateTemp("/tmp", "log_output_app2_test_*.file1")
	if err != nil {
		panic("File1: Failed to create temporary file: " + err.Error())
	}
	defer func() {
		os.Remove(fp.Name())
	}()

	fp.WriteString("Test log content\n")
	fp.Sync()

	// Set up a mock server to server counter value
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("42"))
	}))
	defer mockServer.Close()

	os.Setenv("PORT", "8080")
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	testRouter = setupRouter(fp.Name(), mockServer.URL)
	os.Exit(m.Run())
}

func TestGetIndex(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	testRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMovedPermanently, w.Code)
}

func TestGetLog(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/log", nil)
	testRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Test log content\nPing / Pongs: 42")
}
