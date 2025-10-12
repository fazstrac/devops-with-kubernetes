package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
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

type AppConfig struct {
	MaxAge       time.Duration
	GracePeriod  time.Duration
	FetchTimeout time.Duration
}

type testCase struct {
	name                   string
	backendHTTPHandlerFunc http.HandlerFunc
	initialFile            []byte
	isColdStart            bool
	expectedHTTPCode       int
	expectErr              bool
}

func setupTestServer(handler http.HandlerFunc, initialFile []byte) (*httptest.Server, string, context.Context, context.CancelFunc, *sync.WaitGroup) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	ts := httptest.NewServer(handler)
	dir, _ := os.MkdirTemp(os.TempDir(), "test_startup_*")
	if initialFile != nil {
		os.WriteFile(dir+"/image.jpg", initialFile, 0644)
	}

	return ts, dir, ctx, cancel, &wg
}

func teardownTestServer(ts *httptest.Server, app *App, dir string, cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
	ts.Close()
	os.RemoveAll(dir)
	close(app.HeartbeatChan)
}
