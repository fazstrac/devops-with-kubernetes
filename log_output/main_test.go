package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIndex(t *testing.T) {
	os.Setenv("PORT", "8080") // Ensure PORT is set for the handler

	router := setupRouter("TestGetIndex") // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMovedPermanently, w.Code)
}

func TestGetLog(t *testing.T) {
	os.Setenv("PORT", "8080") // Ensure PORT is set for the handler

	router := setupRouter("TestGetLog") // Use the same router logic as in main.go

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/log", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
