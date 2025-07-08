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
	testRouter = setupRouter()
	os.Exit(m.Run())
}

func TestIncrCounter(t *testing.T) {
	counterMutex.Lock()
	counter = 0
	counterMutex.Unlock()
	expected := "pong 1"
	result := incrCounter()
	assert.Equal(t, expected, result)
}

func TestIncrCounterEndpoint(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/pingpong", nil)
	testRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
