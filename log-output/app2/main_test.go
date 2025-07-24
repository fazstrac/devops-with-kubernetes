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
	fp, err := os.CreateTemp("/tmp", "log_output_app2_test_*.log")
	if err != nil {
		panic("Failed to create temporary log file: " + err.Error())
	}
	defer os.Remove(fp.Name())

	os.Setenv("PORT", "8080")
	testRouter = setupRouter(fp.Name())
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
