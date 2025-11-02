package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TODOMAXLENGTTH = 140

// Todo represents a single todo item.
type Todo struct {
	UUID        string    `json:"uuid"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ChangedAt   time.Time `json:"changed_at,omitempty"`
}

// TodoMgr holds in-memory todos and a mutex for concurrency.
type TodoMgr struct {
	mu          sync.RWMutex
	todosSorted []Todo
}

func main() {
	s := &TodoMgr{}
	r := setupRouter(s)

	// Default port if not set via environment variable
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8080")
	}

	log.Println("Starting todo-backend on port", os.Getenv("PORT"))
	if err := r.Run("0.0.0.0:" + os.Getenv("PORT")); err != nil {
		log.Fatalf("Todo-backend failed to start: %v", err)
	}
}

func setupRouter(s *TodoMgr) *gin.Engine {
	r := gin.Default()
	r.GET("/todos", s.getTodos)
	r.POST("/todos", s.createTodo)
	r.DELETE("/todos/:uuid", s.deleteTodo)
	r.PATCH("/todos/:uuid", s.patchTodo)
	// Disable unsupported methods
	r.DELETE("/todos", func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "DELETE /todos is not allowed"})
	})
	r.PATCH("/todos", func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "PATCH /todos is not allowed"})
	})
	return r
}

// getTodos handles retrieval of all todo items.
// @success 200 {array} Todo
func (s *TodoMgr) getTodos(c *gin.Context) {
	if len(s.todosSorted) == 0 {
		c.JSON(http.StatusOK, []Todo{})
		return
	}

	s.mu.RLock()
	// return a copy to avoid racey access by callers
	out := make([]Todo, len(s.todosSorted))
	copy(out, s.todosSorted)
	s.mu.RUnlock()

	c.JSON(http.StatusOK, out)
}

// createTodo handles the creation of a new todo item.
// @param description body string true "Description of the todo"
// @success 201 {object} Todo
// @failure 400 {object} map[string]string
func (s *TodoMgr) createTodo(c *gin.Context) {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request" + err.Error()})
		return
	}

	req.Description = strings.TrimSpace(req.Description)
	if req.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description is required"})
		return
	}

	if len(req.Description) > TODOMAXLENGTTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description exceeds maximum length"})
		return
	}

	// Create new todo

	t := Todo{
		UUID:        uuid.New().String(),
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
		ChangedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.todosSorted = append(s.todosSorted, t)
	s.mu.Unlock()

	c.JSON(http.StatusCreated, t)
}

// deleteTodo handles deletion of a todo by UUID.
// @param uuid path string true "UUID of the todo to delete"
// @success 200 {object} map[string]string
// @failure 400 {object} map[string]string
// @failure 404 {object} map[string]string
func (s *TodoMgr) deleteTodo(c *gin.Context) {
	UUID := c.Param("uuid")

	if UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	UUID = strings.TrimSpace(UUID)
	if UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todosSorted {
		if t.UUID == UUID {
			// Remove the todo from the slice
			s.todosSorted = append(s.todosSorted[:i], s.todosSorted[i+1:]...)
			c.JSON(http.StatusOK, gin.H{"message": "todo deleted"})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
}

// patchTodo handles partial updates to a todo's description.
// @param uuid path string true "UUID of the todo to update"
// @param description body string true "New description for the todo"
// @success 200 {object} Todo
// @failure 400 {object} map[string]string
// @failure 404 {object} map[string]string
func (s *TodoMgr) patchTodo(c *gin.Context) {
	UUID := c.Param("uuid")
	if UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	UUID = strings.TrimSpace(UUID)
	req.Description = strings.TrimSpace(req.Description)

	if req.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new description is required"})
		return
	}

	if len(req.Description) > TODOMAXLENGTTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description exceeds maximum length"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// For proper patch, we should check which fields are provided and different, and only update those.
	for i, t := range s.todosSorted {
		if t.UUID == UUID {
			s.todosSorted[i].Description = req.Description
			s.todosSorted[i].ChangedAt = time.Now().UTC()
			c.JSON(http.StatusOK, s.todosSorted[i])
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
}
