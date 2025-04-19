package server

import (
	"encoding/json"
	"fmt"
	"github.com/shuliakovsky/email-checker/internal/logger"
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"sync"
	"time"

	_ "github.com/shuliakovsky/email-checker/docs"
	"github.com/shuliakovsky/email-checker/internal/checker"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

type Task struct {
	ID        string              // Unique identifier for the task
	Status    string              // Current status of the task ("pending", "processing", "completed")
	Emails    []string            // List of emails to be processed
	Results   []types.EmailReport // Results of the email processing
	CreatedAt time.Time           // Timestamp when the task was created
}

type Server struct {
	mu    sync.RWMutex     // Mutex for protecting concurrent access to tasks
	tasks map[string]*Task // Map of tasks with their unique IDs
	port  string           // Port on which the server listens
}
type TaskStatusResponse struct {
	Status  string              `json:"status"`
	Results []types.EmailReport `json:"results,omitempty"`
}

// NewServer creates and returns a new Server instance with the specified port
func NewServer(port string) *Server {
	return &Server{
		tasks: make(map[string]*Task),
		port:  port,
	}
}

// generateID generates a unique ID for a new task based on the current time
func (s *Server) generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Start initializes the server and begins listening for HTTP requests

func (s *Server) Start() error {
	router := http.NewServeMux()                            // Create a new HTTP request router
	router.HandleFunc("/tasks", s.handleTasks)              // Route for task creation
	router.HandleFunc("/tasks/", s.handleTaskStatus)        // Route for checking task status
	router.HandleFunc("/swagger/", httpSwagger.WrapHandler) // Route for swagger

	loggedRouter := loggingMiddleware(router) // Wrap the router with logging middleware

	return http.ListenAndServe(":"+s.port, loggedRouter) // Start the server
}

// handleTasks handles task creation requests
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Parse the incoming JSON request
		var request struct {
			Emails []string `json:"emails"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest) // Respond with 400 if the request is invalid
			return
		}

		// Generate a new task ID and create a new task
		taskID := s.generateID()
		task := &Task{
			ID:        taskID,
			Status:    "pending",      // Initial status of the task
			Emails:    request.Emails, // Emails to be processed
			CreatedAt: time.Now(),     // Task creation timestamp
		}

		// Add the task to the server's task map
		s.mu.Lock()
		s.tasks[taskID] = task
		s.mu.Unlock()

		// Start processing the task asynchronously
		go s.processTask(task)

		// Respond with the new task ID
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed) // Respond with 405 for unsupported HTTP methods
}

// handleTaskStatus handles requests for checking task status
func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	// Extract the task ID from the URL path
	taskID := r.URL.Path[len("/tasks/"):]

	// Find the task in the server's task map
	s.mu.RLock()
	task, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound) // Respond with 404 if the task does not exist
		return
	}

	// Prepare the response with the task's status and results (if available)
	response := struct {
		Status  string              `json:"status"`
		Results []types.EmailReport `json:"results,omitempty"`
	}{
		Status:  task.Status,
		Results: task.Results,
	}

	// Respond with the task information as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// processTask processes the task asynchronously
func (s *Server) processTask(task *Task) {
	// Update the task status to "processing"
	s.mu.Lock()
	task.Status = "processing"
	s.mu.Unlock()

	// Process the emails and retrieve the results
	results := checker.ProcessEmails(task.Emails)

	// Update the task status to "completed" and store the results
	s.mu.Lock()
	task.Status = "completed"
	task.Results = results
	s.mu.Unlock()
}

// loggingMiddleware is a middleware that logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Log(fmt.Sprintf("%s %s", r.Method, r.URL.Path)) // Log the HTTP method and URL path
		next.ServeHTTP(w, r)                                   // Pass the request to the next handler
	})
}
