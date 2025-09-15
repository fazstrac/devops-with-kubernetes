package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// Test endpoints for the application

func TestGetIndexSuccess(t *testing.T) {
	app := NewApp(
		"./cache/image.jpg",
		"https://picsum.photos/1200",
		10*time.Minute,
		1*time.Minute,
		30*time.Second,
	)

	w := httptest.NewRecorder()
	c, eng := gin.CreateTestContext(w)
	eng.LoadHTMLGlob("templates/*")
	assert.NotNil(t, c)
	app.GetIndex(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetImageSuccess(t *testing.T) {
	testImage := []byte("This is a test image content")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer ts.Close()

	// This test checks if the getImage handler works correctly.
	// It creates a temporary directory for the image and checks if the response is correct.
	dir, err := os.MkdirTemp(os.TempDir(), "test_get_image_*")
	assert.NoError(t, err, "Failed to create temporary directory for test")
	defer os.RemoveAll(dir) // Clean up the temporary directory after the test

	app := NewApp(
		dir+"/image.jpg", // Use a temporary image path for testing
		ts.URL,           // Use the test server URL
		10*time.Minute,   // Set a reasonable max age for the image
		1*time.Minute,    // Grace period during which the old image can be fetched _once_
		30*time.Second,   // Timeout for fetching the image from the backend
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	app.GetImage(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, testImage, w.Body.Bytes(), "Response body should match the test image content")
}

// Test auxiliary functions for image handling

func TestFetchImageCases(t *testing.T) {
	testImage := []byte("This is a test image content")
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
	testImage := []byte("This is a test image content")

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
	testImage := []byte("This is a test image content")

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
