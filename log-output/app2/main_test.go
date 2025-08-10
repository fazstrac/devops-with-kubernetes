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
	fp, err := os.CreateTemp("/tmp", "log_output_app2_test_*.file1")
	if err != nil {
		panic("File1: Failed to create temporary file: " + err.Error())
	}
	defer os.Remove(fp.Name())

	fp2, err2 := os.CreateTemp("/tmp", "log_output_app2_test_*.file2")
	if err2 != nil {
		panic("File2: Failed to create temporary file: " + err2.Error())
	}
	defer os.Remove(fp2.Name())

	os.Setenv("PORT", "8080")
	testRouter = setupRouter(fp.Name(), fp2.Name())
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
}
