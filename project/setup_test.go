package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
	"io"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	// global setup: run Gin in release mode and silence the package logger to reduce
	// noisy test output. Tests still validate behavior but will be quieter.
	gin.SetMode(gin.ReleaseMode)
	// assign to package-level logger (do not shadow)
	logger = setupLogger()
	// Silence logger output during tests; individual tests may re-enable if needed
	logger.SetOutput(io.Discard)
	// Also silence Gin's default writers to prevent request logging during tests
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	// Ensure package-level logger variable is set to a discarded-output logger so
	// any later calls that recreate or reference it are quiet.
	pkgLogger := log.New(io.Discard, "[DwK-Project] ", log.Ldate|log.Ltime|log.Lshortfile)
	logger = pkgLogger
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
