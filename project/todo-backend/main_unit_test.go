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
	assert.NotEmpty(t, resp.ID)

	// Ensure the todo was stored
	assert.Equal(t, 1, len(s.todos))
	assert.Equal(t, resp.ID, s.todos[0].ID)
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
