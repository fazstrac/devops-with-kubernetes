package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func NewMockResponse(payload []byte, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(payload)),
		Header:     make(http.Header),
	}
}

type MockApp struct {
	mock.Mock
}

func (m *MockApp) SaveImage(imagePath string, resp *http.Response) error {
	args := m.Called(imagePath, resp)
	return args.Error(0)
}

func (m *MockApp) FetchImage(fname string, url string) (int, time.Duration, error) {
	args := m.Called(fname, url)
	return args.Int(0), args.Get(1).(time.Duration), args.Error(2)
}

func (m *MockApp) RetryWithFibonacci(ctx context.Context, maxRetries int, fn func() (int, time.Duration, error)) error {
	args := m.Called(ctx, maxRetries, fn)
	fn()

	return args.Error(0)
}

// ***

type MockFileReader struct {
	mock.Mock
}

func (m *MockFileReader) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

// ***

type MockFSOps struct {
	mock.Mock
}

func (m *MockFSOps) MkdirTemp(dir, pattern string) (string, error) {
	args := m.Called(dir, pattern)
	return args.String(0), args.Error(1)
}

func (m *MockFSOps) Create(imagePath string) (*os.File, error) {
	args := m.Called(imagePath)
	return args.Get(0).(*os.File), args.Error(1)
}

func (m *MockFSOps) Copy(dst io.Writer, src io.Reader) (int64, error) {
	args := m.Called(dst, src)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockFSOps) Rename(oldpath, newpath string) error {
	args := m.Called(oldpath, newpath)
	return args.Error(0)
}

func (m *MockFSOps) RemoveAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

// Mock for os.FileInfo
type MockFileInfo struct {
	mock.Mock
	modTime time.Time
}

func (m *MockFileInfo) Name() string {
	return ""
}

func (m *MockFileInfo) Size() int64 {
	return 0
}

func (m *MockFileInfo) Mode() os.FileMode {
	return 0
}

func (m *MockFileInfo) ModTime() time.Time {
	return m.modTime
}

func (m *MockFileInfo) IsDir() bool {
	return false
}

func (m *MockFileInfo) Sys() interface{} {
	return nil
}

// Mock for StatFunc
type StatMock struct {
	mock.Mock
}

func (m *StatMock) Stat(path string) (os.FileInfo, error) {
	args := m.Called(path)
	fi, _ := args.Get(0).(os.FileInfo)
	return fi, args.Error(1)
}

// Test endpoints for the application

func TestGetIndexSuccess(t *testing.T) {
	app := &App{}

	w := httptest.NewRecorder()
	c, eng := gin.CreateTestContext(w)
	eng.LoadHTMLGlob("templates/*")
	assert.NotNil(t, c)
	app.GetIndex(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetImageCases(t *testing.T) {
	testImage := []byte("Test image contents")

	app := &App{
		ImagePath:         "mockimage.jpg",
		MaxAge:            10 * time.Minute,
		GracePeriod:       1 * time.Minute,
		IsGracePeriodUsed: false,
		mutex:             sync.RWMutex{},
	}

	type testCase struct {
		name                 string
		setupMocks           func(m *MockFileReader)
		imageFetchedAt       time.Time
		isGracePeriodUsed    bool
		expectErr            bool
		assertions           func(t *testing.T, m *MockFileReader)
		expectHTTPStatusCode int
	}

	// Setup the test cases
	cases := []testCase{
		{
			name: "success",
			setupMocks: func(m *MockFileReader) {
				m.On("ReadFile", "mockimage.jpg").Return(testImage, nil)
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				m.AssertNumberOfCalls(t, "ReadFile", 1)
			},
			imageFetchedAt:       time.Now(),
			isGracePeriodUsed:    false,
			expectHTTPStatusCode: http.StatusOK,
			expectErr:            false,
		},
		{
			name: "success image in grace period",
			setupMocks: func(m *MockFileReader) {
				m.On("ReadFile", "mockimage.jpg").Return(testImage, nil)
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				m.AssertNumberOfCalls(t, "ReadFile", 1)
			},
			imageFetchedAt:       time.Now().Add(+1*time.Second - app.MaxAge - app.GracePeriod),
			isGracePeriodUsed:    false,
			expectHTTPStatusCode: http.StatusOK,
			expectErr:            false,
		},
		{
			name: "fail image being refreshed",
			setupMocks: func(m *MockFileReader) {
				// No calls expected
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				// No calls expected
			},
			imageFetchedAt:       time.Now().Add(-1*time.Second - app.MaxAge - app.GracePeriod),
			isGracePeriodUsed:    true,
			expectHTTPStatusCode: http.StatusServiceUnavailable,
			expectErr:            true,
		},
		{
			name: "fail image grace period already used",
			setupMocks: func(m *MockFileReader) {
				// No calls expected
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				// No calls expected
			},
			imageFetchedAt:       time.Now().Add(+1*time.Second - app.MaxAge - app.GracePeriod),
			isGracePeriodUsed:    true,
			expectHTTPStatusCode: http.StatusServiceUnavailable,
			expectErr:            true,
		},
		{
			name: "fail read image",
			setupMocks: func(m *MockFileReader) {
				m.On("ReadFile", "mockimage.jpg").Return([]byte{}, os.ErrNotExist)
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				m.AssertNumberOfCalls(t, "ReadFile", 1)
			},
			imageFetchedAt:       time.Now(),
			isGracePeriodUsed:    false,
			expectHTTPStatusCode: http.StatusInternalServerError,
			expectErr:            true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockReader := new(MockFileReader)
			tc.setupMocks(mockReader)

			origReadFile := ReadFileFunc
			ReadFileFunc = mockReader.ReadFile
			defer func() { ReadFileFunc = origReadFile }()
			app.ImageFetchedAt = tc.imageFetchedAt // Ensure the image is fresh

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			app.GetImage(c)

			assert.Equal(t, tc.expectHTTPStatusCode, w.Code, "GetImage should return the expected HTTP status code")

			// Check if grace period usage is updated correctly
			if !tc.isGracePeriodUsed {
				if tc.imageFetchedAt.Before(time.Now().Add(-app.MaxAge)) {
					assert.True(t, app.IsGracePeriodUsed, "Grace period should be marked as used")
				} else {
					assert.False(t, app.IsGracePeriodUsed, "Grace period should not be marked as used")
				}
			}

			if !tc.expectErr {
				assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
				assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
			}
			tc.assertions(t, mockReader)
			mockReader.AssertExpectations(t)
		})
	}
}

func TestLoadCachedImageCases(t *testing.T) {
	now := time.Now()
	imagePath := "/tmp/test/image.jpg"
	dirPath := filepath.Dir(imagePath)

	type testCase struct {
		name          string
		statResponses map[string]struct {
			fi  os.FileInfo
			err error
		}
		expectErr     bool
		expectFetched bool
		expectModTime time.Time
	}

	cases := []testCase{
		{
			name: "directory missing",
			statResponses: map[string]struct {
				fi  os.FileInfo
				err error
			}{
				dirPath: {nil, os.ErrNotExist},
			},
			expectErr: true,
		},
		{
			name: "file missing",
			statResponses: map[string]struct {
				fi  os.FileInfo
				err error
			}{
				dirPath:   {&MockFileInfo{}, nil},
				imagePath: {nil, os.ErrNotExist},
			},
			expectErr:     false,
			expectFetched: false,
		},
		{
			name: "file exists",
			statResponses: map[string]struct {
				fi  os.FileInfo
				err error
			}{
				dirPath:   {&MockFileInfo{}, nil},
				imagePath: {&MockFileInfo{modTime: now}, nil},
			},
			expectErr:     false,
			expectFetched: true,
			expectModTime: now,
		},
		{
			name: "file stat error",
			statResponses: map[string]struct {
				fi  os.FileInfo
				err error
			}{
				dirPath:   {&MockFileInfo{}, nil},
				imagePath: {nil, errors.New("stat error")},
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup StatMock
			statMock := &StatMock{}
			for path, resp := range tc.statResponses {
				statMock.On("Stat", path).Return(resp.fi, resp.err)
			}

			// Patch StatFunc
			origStatFunc := StatFunc
			StatFunc = statMock.Stat
			defer func() { StatFunc = origStatFunc }()

			app := &App{ImagePath: imagePath}
			err := app.LoadCachedImage()

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.expectFetched {
				assert.WithinDuration(t, tc.expectModTime, app.ImageFetchedAt, time.Second)
			} else {
				assert.True(t, app.ImageFetchedAt.IsZero())
			}
			statMock.AssertExpectations(t)
		})
	}
}

func TestFetchImageCases(t *testing.T) {
	testImage := []byte("Test image contents")
	imagePath := "mockimage.jpg"

	type testCase struct {
		name        string
		setupMocks  func(m *MockApp)
		setupServer func() (ts *httptest.Server)
		expectErr   bool
		assertions  func(t *testing.T, m *MockApp)
	}

	cases := []testCase{
		{
			name: "success",
			setupMocks: func(m *MockApp) {
				m.On("SaveImage", imagePath, mock.Anything).Return(nil)
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertNumberOfCalls(t, "SaveImage", 1)
			},
			expectErr: false,
		},
		{
			name: "fail retry-later 1",
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gmt := time.FixedZone("GMT", 0)

					w.Header().Set("Retry-After", time.Now().Add(25*time.Second).In(gmt).Format(time.RFC1123))
					w.WriteHeader(http.StatusServiceUnavailable) // Simulate a temporary failure
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				// No calls expected
			},
			expectErr: true,
		},
		{
			name: "fail retry-later 2",
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "120")
					w.WriteHeader(http.StatusServiceUnavailable) // Simulate a temporary failure
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				// No calls expected
			},
			expectErr: true,
		},
		{
			name: "fail retry-later 3",
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Format the date deliberately wrong by missing the timezone .In(gmt)
					w.Header().Set("Retry-After", time.Now().Add(25*time.Second).Format(time.RFC1123))
					w.WriteHeader(http.StatusServiceUnavailable) // Simulate a temporary failure
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				// No calls expected
			},
			expectErr: true,
		},
		{
			name: "fail with bad url",
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return nil // No server needed for invalid URL
			},
			assertions: func(t *testing.T, m *MockApp) {
				// No calls expected
			},
			expectErr: true,
		},
		{
			name: "fail bad response",
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusForbidden) // Simulate a permanent failure
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				// No calls expected
			},
			expectErr: true,
		},
		{
			name: "fail save image",
			setupMocks: func(m *MockApp) {
				m.On("SaveImage", imagePath, mock.Anything).Return(os.ErrPermission)
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertNumberOfCalls(t, "SaveImage", 1)
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSave := new(MockApp)
			tc.setupMocks(mockSave)

			origSaveImageFunc := SaveImageFunc
			SaveImageFunc = mockSave.SaveImage
			defer func() {
				SaveImageFunc = origSaveImageFunc
			}()

			var imageUrl string

			if tc.name != "fail with bad url" {
				ts := tc.setupServer()
				defer ts.Close()
				imageUrl = ts.URL
			} else {
				imageUrl = "http://invalid-url"
			}

			status, waitDuration, err := fetchImage(imagePath, imageUrl)

			switch tc.name {
			case "fail retry-later 1":
				diff := waitDuration - 25*time.Second
				if diff < 0 {
					diff = -diff
				}
				assert.LessOrEqual(t, diff, 2*time.Second)
				assert.Equal(t, http.StatusServiceUnavailable, status)
				assert.Equal(t, http.ErrMissingFile, err)
			case "fail retry-later 2":
				assert.Equal(t, 120*time.Second, waitDuration)
				assert.Equal(t, http.StatusServiceUnavailable, status)
				assert.Equal(t, http.ErrMissingFile, err)
			case "fail retry-later 3":
				assert.Equal(t, time.Duration(0), waitDuration)
				assert.Equal(t, http.StatusServiceUnavailable, status)
				assert.Equal(t, http.ErrMissingFile, err)
			case "fail with bad url":
				assert.Equal(t, time.Duration(0), waitDuration)
				assert.Equal(t, 666, status)
				assert.Error(t, err)
			case "fail bad response":
				assert.Equal(t, time.Duration(0), waitDuration)
				assert.Equal(t, http.StatusForbidden, status)
				assert.Equal(t, http.ErrMissingFile, err)
			case "fail save image":
				assert.Equal(t, time.Duration(0), waitDuration)
				assert.Equal(t, http.StatusOK, status)
				assert.Equal(t, os.ErrPermission, err)
			case "success":
				assert.Equal(t, time.Duration(0), waitDuration)
				assert.Equal(t, http.StatusOK, status)
				assert.NoError(t, err)
			}

			if tc.expectErr {
				assert.Error(t, err, "fetchImage should return an error")
			} else {
				assert.NoError(t, err, "fetchImage should not return an error")
			}

			tc.assertions(t, mockSave)
			mockSave.AssertExpectations(t)
		})
	}
}

func TestRetryWithFibonacciCases(t *testing.T) {
	type testCase struct {
		name       string
		maxRetries int
		setupApp   func() *App
		fn         func() (int, time.Duration, error)
		expectErr  bool
		assertions func(t *testing.T, m *MockApp)
	}

	app := &App{
		ImagePath:         "mockimage.jpg",
		ImageUrl:          "http://mockurl/image.jpg",
		MaxAge:            10 * time.Minute,
		GracePeriod:       1 * time.Minute,
		FetchImageTimeout: 1 * time.Minute,
		IsGracePeriodUsed: false,
		mutex:             sync.RWMutex{},
	}

	cases := []testCase{
		{
			name:       "success first try",
			maxRetries: 5,
			setupApp: func() *App {
				return app
			},
			fn: func() (int, time.Duration, error) {
				return http.StatusOK, 0, nil
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: false,
		},
		{
			name:       "success after retries",
			maxRetries: 5,
			setupApp: func() *App {
				return app
			},
			fn: func() func() (int, time.Duration, error) {
				staticCounter := 0

				return func() (int, time.Duration, error) {
					staticCounter++
					if staticCounter < 3 {
						return http.StatusServiceUnavailable, 1 * time.Second, http.ErrMissingFile
					}
					return http.StatusOK, 0, nil
				}
			}(),
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: false,
		},
		{
			name:       "fail all retries with ServiceUnavailable",
			maxRetries: 3,
			setupApp: func() *App {
				return app
			},
			fn: func() (int, time.Duration, error) {
				return http.StatusServiceUnavailable, 1 * time.Second, http.ErrMissingFile
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: true,
		},
		{
			name:       "fail all retries with TooManyRequests",
			maxRetries: 3,
			setupApp: func() *App {
				return app
			},
			fn: func() (int, time.Duration, error) {
				return http.StatusTooManyRequests, 1 * time.Second, http.ErrMissingFile
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: true,
		},
		{
			name:       "fail non-retryable error",
			maxRetries: 5,
			setupApp: func() *App {
				return app
			},
			fn: func() (int, time.Duration, error) {
				return 666, 0, errors.New("non-retryable error")
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: true,
		},
		{
			name:       "fail context timeout",
			maxRetries: 3,
			setupApp: func() *App {
				app.FetchImageTimeout = 1 * time.Second
				return app
			},
			fn: func() (int, time.Duration, error) {
				time.Sleep(3 * time.Second)
				return http.StatusServiceUnavailable, 1 * time.Second, http.ErrMissingFile
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertExpectations(t)
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockFcn := new(MockApp)

			app := tc.setupApp()

			ctx, cancel := context.WithTimeout(context.Background(), app.FetchImageTimeout)
			defer cancel()

			err := retryWithFibonacci(ctx, tc.maxRetries, tc.fn)
			if tc.expectErr {
				assert.Error(t, err, "retryWithFibonacci should return an error")
			} else {
				assert.NoError(t, err, "retryWithFibonacci should not return an error")
			}
			tc.assertions(t, mockFcn)
			mockFcn.AssertExpectations(t)
		})
	}
}

func TestTryFetchImageCases(t *testing.T) {
	type testCase struct {
		name       string
		setupApp   func() *App
		setupMocks func(m *MockApp)
		expectErr  bool
		assertions func(t *testing.T, m *MockApp)
	}

	app := &App{
		ImagePath:         "mockimage.jpg",
		ImageUrl:          "http://mockurl/image.jpg",
		MaxAge:            10 * time.Minute,
		GracePeriod:       1 * time.Minute,
		IsGracePeriodUsed: false,
		mutex:             sync.RWMutex{},
	}

	cases := []testCase{
		{
			name: "success fetch",
			setupApp: func() *App {
				app.IsFetchingImage = false
				app.FetchImageTimeout = 20 * time.Second
				return app
			},
			setupMocks: func(m *MockApp) {
				m.On("FetchImage", mock.Anything, mock.Anything).Return(http.StatusOK, time.Duration(0), nil)
				m.On("RetryWithFibonacci", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertNumberOfCalls(t, "FetchImage", 1)
				m.AssertNumberOfCalls(t, "RetryWithFibonacci", 1)
				m.AssertExpectations(t)
				assert.False(t, app.IsFetchingImage, "IsFetchingImage should be reset to false after fetch")
			},
			expectErr: false,
		},
		{
			name: "success already fetching",
			setupApp: func() *App {
				app.IsFetchingImage = true
				app.FetchImageTimeout = 20 * time.Second
				return app
			},
			setupMocks: func(m *MockApp) {
				// No calls expected
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertNumberOfCalls(t, "FetchImage", 0)
				m.AssertNumberOfCalls(t, "RetryWithFibonacci", 0)
				m.AssertExpectations(t)
				assert.True(t, app.IsFetchingImage, "IsFetchingImage should remain true")
			},
			expectErr: false,
		},
		{
			name: "fail fetch",
			setupApp: func() *App {
				app.IsFetchingImage = false
				app.FetchImageTimeout = 20 * time.Second
				return app
			},
			setupMocks: func(m *MockApp) {
				m.On("FetchImage", mock.Anything, mock.Anything).Return(http.StatusServiceUnavailable, 15*time.Second, http.ErrMissingFile)
				m.On("RetryWithFibonacci", mock.Anything, mock.Anything, mock.Anything).Return(http.ErrMissingFile)
			},
			assertions: func(t *testing.T, m *MockApp) {
				m.AssertNumberOfCalls(t, "FetchImage", 1)
				m.AssertNumberOfCalls(t, "RetryWithFibonacci", 1)
				m.AssertExpectations(t)
				assert.False(t, app.IsFetchingImage, "IsFetchingImage should be reset to false after fetch")
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockFcn := new(MockApp)
			tc.setupMocks(mockFcn)

			origFetchImageFunc := FetchImageFunc
			FetchImageFunc = mockFcn.FetchImage
			origRetryWithFibonacci := RetryWithFibonacciFunc
			RetryWithFibonacciFunc = mockFcn.RetryWithFibonacci

			defer func() {
				FetchImageFunc = origFetchImageFunc
				RetryWithFibonacciFunc = origRetryWithFibonacci
			}()

			ctx := context.Background()

			err := tryFetchImage(ctx, tc.setupApp())
			if tc.expectErr {
				assert.Error(t, err, "tryFetchImage should return an error")
			} else {
				assert.NoError(t, err, "tryFetchImage should not return an error")
			}
			tc.assertions(t, mockFcn)
		})
	}
}

func TestReadImageCases(t *testing.T) {
	testImage := []byte("Test image contents")

	type testCase struct {
		name       string
		setupMocks func(m *MockFileReader)
		expectErr  bool
		assertions func(t *testing.T, m *MockFileReader)
	}

	cases := []testCase{
		{
			name: "success",
			setupMocks: func(m *MockFileReader) {
				m.On("ReadFile", "mockimage.jpg").Return(testImage, nil)
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				m.AssertNumberOfCalls(t, "ReadFile", 1)
			},
			expectErr: false,
		},
		{
			name: "success",
			setupMocks: func(m *MockFileReader) {
				m.On("ReadFile", "mockimage.jpg").Return([]byte{}, os.ErrNotExist)
			},
			assertions: func(t *testing.T, m *MockFileReader) {
				m.AssertNumberOfCalls(t, "ReadFile", 1)
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockReader := new(MockFileReader)
			tc.setupMocks(mockReader)

			origReadFileFunc := ReadFileFunc
			ReadFileFunc = mockReader.ReadFile
			defer func() { ReadFileFunc = origReadFileFunc }()

			data, err := readImage("mockimage.jpg")
			if tc.expectErr {
				assert.Error(t, err, "readImage should return an error")
			} else {
				assert.NoError(t, err, "readImage should not return an error")
				assert.Equal(t, string(testImage), data, "readImage should return the correct image content")
			}

			tc.assertions(t, mockReader)
			mockReader.AssertExpectations(t)
		})
	}
}

// saveImage tests
func TestSaveImageCases(t *testing.T) {
	testImage := []byte("Test image content")

	type testCase struct {
		name       string
		setupMocks func(m *MockFSOps)
		expectErr  bool
		assertions func(t *testing.T, m *MockFSOps)
	}

	cases := []testCase{
		{
			name: "success",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("tempdir", nil)
				m.On("Create", mock.Anything).Return(&os.File{}, nil)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), nil)
				m.On("Rename", mock.Anything, mock.Anything).Return(nil)
				m.On("RemoveAll", mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockFSOps) {
				m.AssertNumberOfCalls(t, "MkdirTemp", 1)
				m.AssertNumberOfCalls(t, "Create", 1)
				m.AssertNumberOfCalls(t, "Copy", 1)
				m.AssertNumberOfCalls(t, "Rename", 1)
				m.AssertNumberOfCalls(t, "RemoveAll", 1)
			},
		},
		{
			name: "fail mkdirTemp",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("", os.ErrPermission)
				m.On("Create", mock.Anything).Return(&os.File{}, nil)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), nil)
				m.On("Rename", mock.Anything, mock.Anything).Return(nil)
				m.On("RemoveAll", mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockFSOps) {
				m.AssertNumberOfCalls(t, "MkdirTemp", 1)
				m.AssertNumberOfCalls(t, "Create", 0)
				m.AssertNumberOfCalls(t, "Copy", 0)
				m.AssertNumberOfCalls(t, "Rename", 0)
				m.AssertNumberOfCalls(t, "RemoveAll", 0)
			},
			expectErr: true,
		},
		{
			name: "fail create",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("tempdir", nil)
				m.On("Create", mock.Anything).Return(&os.File{}, os.ErrPermission)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), nil)
				m.On("Rename", mock.Anything, mock.Anything).Return(nil)
				m.On("RemoveAll", mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockFSOps) {
				m.AssertNumberOfCalls(t, "MkdirTemp", 1)
				m.AssertNumberOfCalls(t, "Create", 1)
				m.AssertNumberOfCalls(t, "Copy", 0)
				m.AssertNumberOfCalls(t, "Rename", 0)
				m.AssertNumberOfCalls(t, "RemoveAll", 1)
			},
			expectErr: true,
		},
		{
			name: "fail copy",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("tempdir", nil)
				m.On("Create", mock.Anything).Return(&os.File{}, nil)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), os.ErrClosed)
				m.On("Rename", mock.Anything, mock.Anything).Return(nil)
				m.On("RemoveAll", mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockFSOps) {
				m.AssertNumberOfCalls(t, "MkdirTemp", 1)
				m.AssertNumberOfCalls(t, "Create", 1)
				m.AssertNumberOfCalls(t, "Copy", 1)
				m.AssertNumberOfCalls(t, "Rename", 0)
				m.AssertNumberOfCalls(t, "RemoveAll", 1)
			},
			expectErr: true,
		},
		{
			name: "fail rename",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("tempdir", nil)
				m.On("Create", mock.Anything).Return(&os.File{}, nil)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), nil)
				m.On("Rename", mock.Anything, mock.Anything).Return(os.ErrPermission)
				m.On("RemoveAll", mock.Anything).Return(nil)
			},
			assertions: func(t *testing.T, m *MockFSOps) {
				m.AssertNumberOfCalls(t, "MkdirTemp", 1)
				m.AssertNumberOfCalls(t, "Create", 1)
				m.AssertNumberOfCalls(t, "Copy", 1)
				m.AssertNumberOfCalls(t, "Rename", 1)
				m.AssertNumberOfCalls(t, "RemoveAll", 1)
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockFS := new(MockFSOps)
			tc.setupMocks(mockFS)

			origMkdirTemp := MkdirTempFunc
			origCreate := CreateFunc
			origCopy := CopyFunc
			origRename := RenameFunc
			origRemoveAll := RemoveAllFunc

			defer func() {
				MkdirTempFunc = origMkdirTemp
				CreateFunc = origCreate
				CopyFunc = origCopy
				RenameFunc = origRename
				RemoveAllFunc = origRemoveAll
			}()

			// Inject mocks into your saveImage logic
			MkdirTempFunc = mockFS.MkdirTemp
			CreateFunc = mockFS.Create
			CopyFunc = mockFS.Copy
			RenameFunc = mockFS.Rename
			RemoveAllFunc = mockFS.RemoveAll

			resp := NewMockResponse(testImage, http.StatusOK)
			imagePath := "mockimage.jpg"

			err := saveImage(imagePath, resp)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.assertions != nil {
				tc.assertions(t, mockFS)
			}
		})
	}
}
