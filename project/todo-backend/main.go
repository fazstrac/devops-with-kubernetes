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

// Todo represents a single todo item.
type Todo struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// TodoMgr holds in-memory todos and a mutex for concurrency.
type TodoMgr struct {
	mu    sync.RWMutex
	todos []Todo
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
	return r
}

func (s *TodoMgr) getTodos(c *gin.Context) {
	if len(s.todos) == 0 {
		c.JSON(http.StatusOK, []Todo{})
		return
	}

	s.mu.RLock()
	// return a copy to avoid racey access by callers
	out := make([]Todo, len(s.todos))
	copy(out, s.todos)
	s.mu.RUnlock()

	c.JSON(http.StatusOK, out)
}

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

	t := Todo{
		ID:          uuid.New().String(),
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.todos = append(s.todos, t)
	s.mu.Unlock()

	c.JSON(http.StatusCreated, t)
}
