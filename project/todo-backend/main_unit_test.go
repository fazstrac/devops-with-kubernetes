package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Silence Gin during tests
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	m.Run()
}

func TestSetupRouter(t *testing.T) {
	s := &TodoMgr{}
	router := setupRouter(s)

	assert.NotNil(t, router)
	// Expect two routes: GET /todos and POST /todos
	routes := router.Routes()
	assert.GreaterOrEqual(t, len(routes), 2)
}

func TestGetTodos_Empty(t *testing.T) {
	s := &TodoMgr{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	s.getTodos(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// Expect empty JSON array
	assert.JSONEq(t, "[]", strings.TrimSpace(w.Body.String()))
}

func TestCreateTodo_Success(t *testing.T) {
	s := &TodoMgr{}
	router := setupRouter(s)

	payload := map[string]string{"description": "buy milk"}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/todos", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp Todo
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "buy milk", resp.Description)
	assert.NotEmpty(t, resp.UUID)

	// Ensure the todo was stored
	assert.Equal(t, 1, len(s.todosSorted))
	assert.Equal(t, resp.UUID, s.todosSorted[0].UUID)
}

func TestCreateTodo_BadRequests(t *testing.T) {
	s := &TodoMgr{}
	router := setupRouter(s)

	// Empty JSON -> missing description
	req := httptest.NewRequest(http.MethodPost, "/todos", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Blank description -> bad request
	req = httptest.NewRequest(http.MethodPost, "/todos", bytes.NewReader([]byte(`{"description":"   "}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/todos", bytes.NewReader([]byte(`notjson`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTodo_TooLongDescription(t *testing.T) {
	s := &TodoMgr{}
	router := setupRouter(s)

	// Description exceeding maximum length
	longDesc := strings.Repeat("a", TODOMAXLENGTTH+1)
	payload := map[string]string{"description": longDesc}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/todos", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteTodo_TodoExists(t *testing.T) {
	s := &TodoMgr{}
	// Pre-populate with a todo
	todo := Todo{
		UUID:        "test-uuid-123",
		Description: "Test Todo",
	}
	s.todosSorted = append(s.todosSorted, todo)

	router := setupRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/todos/"+todo.UUID, bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Ensure the todo was deleted
	assert.Equal(t, 0, len(s.todosSorted))
}

func TestDeleteTodo_TodoNotFound(t *testing.T) {
	s := &TodoMgr{}
	// Pre-populate with a todo
	todo := Todo{
		UUID:        "test-uuid-123",
		Description: "Test Todo",
	}
	s.todosSorted = append(s.todosSorted, todo)

	router := setupRouter(s)

	req := httptest.NewRequest(http.MethodDelete, "/todos/non-existent-uuid", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Ensure the todo was not deleted
	assert.Equal(t, 1, len(s.todosSorted))
}

func TestDeleteTodo_BadRequests(t *testing.T) {
	s := &TodoMgr{}
	router := setupRouter(s)

	// Empty JSON -> missing uuid
	req := httptest.NewRequest(http.MethodDelete, "/todos", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// blank uuid, garbage in body
	req = httptest.NewRequest(http.MethodDelete, "/todos", bytes.NewReader([]byte(`{"uuid":"   "}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestPatchTodo_TodoExists(t *testing.T) {
	s := &TodoMgr{}
	// Pre-populate with a todo
	todo := Todo{
		UUID:        "test-uuid-123",
		Description: "Old Description",
	}
	s.todosSorted = append(s.todosSorted, todo)

	router := setupRouter(s)

	payload := map[string]string{
		"description": "New Description",
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPatch, "/todos/"+todo.UUID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Todo
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "New Description", resp.Description)

	// Ensure the todo was updated
	assert.Equal(t, 1, len(s.todosSorted))
	assert.Equal(t, "New Description", s.todosSorted[0].Description)
}

func TestPatchTodo_TodoNotFound(t *testing.T) {
	s := &TodoMgr{}
	// Pre-populate with a todo
	todo := Todo{
		UUID:        "test-uuid-123",
		Description: "Old Description",
	}
	s.todosSorted = append(s.todosSorted, todo)

	router := setupRouter(s)

	payload := map[string]string{
		"description": "New Description",
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPatch, "/todos/"+"non-existent-uuid", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Ensure the todo was not updated
	assert.Equal(t, 1, len(s.todosSorted))
	assert.Equal(t, "Old Description", s.todosSorted[0].Description)
}

func TestPatchTodo_BadRequests(t *testing.T) {
	s := &TodoMgr{}

	router := setupRouter(s)

	// Empty JSON -> missing uuid and description
	req := httptest.NewRequest(http.MethodPatch, "/todos", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// Blank uuid -> bad request
	req = httptest.NewRequest(http.MethodPatch, "/todos", bytes.NewReader([]byte(`{"uuid":"   ","description":"New Desc"}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// Blank description -> bad request
	req = httptest.NewRequest(http.MethodPatch, "/todos/test-uuid", bytes.NewReader([]byte(`{"description":"   "}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Invalid JSON
	req = httptest.NewRequest(http.MethodPatch, "/todos/test-uuid", bytes.NewReader([]byte(`notjson`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchTodo_TooLongDescription(t *testing.T) {
	s := &TodoMgr{}
	// Pre-populate with a todo
	todo := Todo{
		UUID:        "test-uuid-123",
		Description: "Old Description",
	}
	s.todosSorted = append(s.todosSorted, todo)
	originalCount := len(s.todosSorted)

	router := setupRouter(s)

	// Description exceeding maximum length
	longDesc := strings.Repeat("a", TODOMAXLENGTTH+1)
	payload := map[string]string{
		"description": longDesc,
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPatch, "/todos/test-uuid-123", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Ensure that no new todo was created
	assert.Equal(t, len(s.todosSorted), originalCount)

	// Ensure the todo was not updated
	for _, td := range s.todosSorted {
		if td.UUID == "test-uuid-123" {
			assert.Equal(t, "Old Description", td.Description)
		}
	}

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
