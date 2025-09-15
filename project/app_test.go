package main

import (
	"bytes"
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

type MockSaveImage struct {
	mock.Mock
}

func (m *MockSaveImage) SaveImage(imagePath string, resp *http.Response) error {
	args := m.Called(imagePath, resp)
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
		setupMocks  func(m *MockSaveImage)
		setupServer func() (ts *httptest.Server)
		expectErr   bool
		assertions  func(t *testing.T, m *MockSaveImage)
	}

	cases := []testCase{
		{
			name: "success",
			setupMocks: func(m *MockSaveImage) {
				m.On("SaveImage", imagePath, mock.Anything).Return(nil)
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 1)
			},
			expectErr: false,
		},
		{
			name: "success after retries",
			setupMocks: func(m *MockSaveImage) {
				m.On("SaveImage", imagePath, mock.Anything).Return(nil)
			},
			setupServer: func() (ts *httptest.Server) {
				attempts := 0
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if attempts < 3 {
						w.WriteHeader(http.StatusServiceUnavailable) // Simulate a temporary failure
						attempts++
					} else {
						w.Header().Set("Content-Type", "image/jpeg")
						w.WriteHeader(http.StatusOK)
						w.Write(testImage) // Return the test image content after a few attempts
					}
				}))
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 1)
			},
			expectErr: false,
		},
		{
			name: "fail with bad url",
			setupMocks: func(m *MockSaveImage) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return nil // No server needed for invalid URL
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 0)
			},
			expectErr: true,
		},
		{
			name: "fail bad response",
			setupMocks: func(m *MockSaveImage) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusForbidden) // Simulate a permanent failure
				}))
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 0)
			},
			expectErr: true,
		},
		{
			name: "fail after retries",
			setupMocks: func(m *MockSaveImage) {
				// No calls expected
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable) // Simulate a permanent failure
				}))
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 0)
			},
			expectErr: true,
		},
		{
			name: "fail save image",
			setupMocks: func(m *MockSaveImage) {
				m.On("SaveImage", imagePath, mock.Anything).Return(os.ErrPermission)
			},
			setupServer: func() (ts *httptest.Server) {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write(testImage)
				}))
			},
			assertions: func(t *testing.T, m *MockSaveImage) {
				m.AssertNumberOfCalls(t, "SaveImage", 1)
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSave := new(MockSaveImage)
			tc.setupMocks(mockSave)

			origWaitTimes := waitTimes
			waitTimes = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond}

			origSaveImageFunc := SaveImageFunc
			SaveImageFunc = mockSave.SaveImage
			defer func() {
				SaveImageFunc = origSaveImageFunc
				waitTimes = origWaitTimes
			}()

			var imageUrl string

			if tc.name != "fail with bad url" {
				ts := tc.setupServer()
				defer ts.Close()
				imageUrl = ts.URL
			} else {
				imageUrl = "http://invalid-url"
			}

			err := fetchImage(imagePath, imageUrl)
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
		{
			name: "fail removeall",
			setupMocks: func(m *MockFSOps) {
				m.On("MkdirTemp", mock.Anything, mock.Anything).Return("tempdir", nil)
				m.On("Create", mock.Anything).Return(&os.File{}, nil)
				m.On("Copy", mock.Anything, mock.Anything).Return(int64(len(testImage)), nil)
				m.On("Rename", mock.Anything, mock.Anything).Return(nil)
				m.On("RemoveAll", mock.Anything).Return(os.ErrPermission)
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
