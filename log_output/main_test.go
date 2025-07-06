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
	os.Setenv("PORT", "8080")
	testRouter = setupRouter("TestMain")
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
