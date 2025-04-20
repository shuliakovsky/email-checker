package server

import (
	"context"
	"encoding/json"
	"fmt"
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"strconv"
	"time"

	_ "github.com/shuliakovsky/email-checker/docs"           // Auto-generated Swagger documentation
	"github.com/shuliakovsky/email-checker/internal/checker" // Email processing logic
	"github.com/shuliakovsky/email-checker/internal/logger"  // Custom logger
	"github.com/shuliakovsky/email-checker/internal/storage" // Task and results storage
	"github.com/shuliakovsky/email-checker/pkg/types"        // Custom types like Task and EmailReport
)

// Server is the main HTTP server for handling tasks and results
type Server struct {
	storage    storage.Storage // Handles task persistence and caching
	port       string          // Port to run the server
	maxWorkers int             // Maximum number of workers for concurrent processing
}

// TaskStatusResponse represents the response for task status queries
type TaskStatusResponse struct {
	Status       string    `json:"status"`                // Current task status (e.g., "pending", "completed")
	TotalResults int       `json:"total_results"`         // Total number of processed results
	CreatedAt    time.Time `json:"created_at"`            // Timestamp when the task was created
	TotalPages   int       `json:"total_pages,omitempty"` // Optional: Total pages of results
}

// NewServer initializes a new Server instance
func NewServer(port string, storage storage.Storage, maxWorkers int) *Server {
	return &Server{
		storage:    storage,    // Assign provided storage
		port:       port,       // Assign provided port
		maxWorkers: maxWorkers, // Assign max workers
	}
}

// generateID generates a unique task ID using the current timestamp
func (s *Server) generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()) // Use nanoseconds for uniqueness
}

// Start begins the HTTP server and registers route handlers
func (s *Server) Start() error {
	router := http.NewServeMux()                              // Create a new HTTP request multiplexer
	router.HandleFunc("/tasks", s.handleTasks)                // Route to create tasks
	router.HandleFunc("/tasks/", s.handleTaskStatus)          // Route to check task status
	router.HandleFunc("/tasks-results/", s.handleTaskResults) // Route to fetch task results
	router.HandleFunc("/swagger/", httpSwagger.WrapHandler)   // Route for Swagger documentation
	loggedRouter := loggingMiddleware(router)                 // Wrap routes with logging middleware
	return http.ListenAndServe(":"+s.port, loggedRouter)      // Start the server on the specified port
}

// handleTasks processes task creation requests
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request struct {
			Emails []string `json:"emails"` // List of emails to process
		}

		// Parse JSON request body
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Create a new task with a unique ID
		taskID := s.generateID()
		task := &types.Task{
			ID:        taskID,
			Status:    "pending", // Initial status
			Emails:    request.Emails,
			CreatedAt: time.Now(),
		}

		// Save the task to storage
		if err := s.storage.SaveTask(r.Context(), task); err != nil {
			http.Error(w, "Failed to save task", http.StatusInternalServerError)
			return
		}

		// Process the task asynchronously
		go s.processTask(task)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"task_id": taskID}) // Respond with the task ID
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed) // Reject non-POST methods
}

// handleTaskStatus handles requests to retrieve the status of a task
func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/tasks/"):] // Extract task ID from URL

	// Fetch task details from storage
	task, err := s.storage.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Calculate total pages of results
	var totalPages int
	if task.Status == "completed" {
		totalPages = (len(task.Results) + 99) / 100 // Round up for pages
	}

	// Build the response
	response := TaskStatusResponse{
		Status:       task.Status,
		TotalResults: len(task.Results),
		CreatedAt:    task.CreatedAt,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) // Send response as JSON
}

// handleTaskResults handles requests to fetch paginated results of a task
func (s *Server) handleTaskResults(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/tasks-results/"):]             // Extract task ID from URL
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))        // Parse page number from query
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page")) // Parse per-page count from query

	// Default values if not provided
	if perPage <= 0 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}

	// Fetch task details from storage
	task, err := s.storage.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Calculate pagination boundaries
	start := (page - 1) * perPage
	if start < 0 || start >= len(task.Results) {
		start = 0
	}
	end := start + perPage
	if end > len(task.Results) {
		end = len(task.Results)
	}

	// Build the response
	response := struct {
		Data  []types.EmailReport `json:"data"`  // Paginated email reports
		Page  int                 `json:"page"`  // Current page number
		Total int                 `json:"total"` // Total number of results
	}{
		Data:  task.Results[start:end],
		Page:  page,
		Total: len(task.Results),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response) // Send response as JSON
}

// processTask processes an email task and updates its status
func (s *Server) processTask(task *types.Task) {
	ctx := context.Background()
	task.Status = "processing"          // Update status to "processing"
	_ = s.storage.UpdateTask(ctx, task) // Save the updated task status

	// Configure the checker with storage-provided cache
	cfg := checker.Config{
		MaxWorkers:     s.maxWorkers,
		CacheProvider:  s.storage.GetCacheProvider(),
		DomainCacheTTL: 24 * time.Hour,
		ExistTTL:       30 * 24 * time.Hour, // Cache existing emails for 30 days
		NotExistTTL:    24 * time.Hour,      // Cache non-existing emails for 24 hours
	}

	// Process emails and store results
	results := checker.ProcessEmailsWithConfig(task.Emails, cfg)
	task.Status = "completed" // Update status to "completed"
	task.Results = results
	_ = s.storage.UpdateTask(ctx, task) // Save completed task
}

// handleFlushCache clears the server-side cache
func (s *Server) handleFlushCache(w http.ResponseWriter, r *http.Request) {
	s.storage.GetCacheProvider().Flush() // Clear the cache
	w.WriteHeader(http.StatusOK)         // Respond with a success status
}

// loggingMiddleware adds logging for incoming HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Log(fmt.Sprintf("%s %s", r.Method, r.URL.Path)) // Log the method and path
		next.ServeHTTP(w, r)                                   // Call the next handler
	})
}
