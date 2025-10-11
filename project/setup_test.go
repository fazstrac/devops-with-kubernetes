package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// global setup (e.g., logger)
	logger := setupLogger()
	_ = logger // use as needed

	code := m.Run()
	// global teardown

	os.Exit(code)
}

func TestSetupLogger(t *testing.T) {
	logger := setupLogger()
	if logger == nil {
		t.Fatal("Expected logger to be initialized")
	}
}
