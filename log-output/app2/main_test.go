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
	fp_log, err_log := os.CreateTemp("/tmp", "log_output_app2_test_*.file1")
	fp_msg, err_msg := os.CreateTemp("/tmp", "log_output_app2_test_*.file2")

	if err_log != nil || err_msg != nil {
		panic("File1: Failed to create temporary file: " + err_log.Error() + " File2: Failed to create temporary file: " + err_msg.Error())
	}
	defer func() {
		os.Remove(fp_log.Name())
		os.Remove(fp_msg.Name())
	}()

	fp_log.WriteString("Test log content\n")
	fp_log.Sync()

	fp_msg.WriteString("Test file message content\n")
	fp_msg.Sync()

	os.Setenv("COMMON_LOGFILE_NAME", fp_log.Name()[6:]) // strip /data/ prefix
	os.Setenv("MESSAGE", "Test environment message content")

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

	testRouter = setupRouter(fp_log.Name(), fp_msg.Name(), mockServer.URL)
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
	assert.Contains(t, w.Body.String(), "Test log content\nfile content: Test file message content\n env variable: Test environment message content\nPing / Pongs: 42")
}
