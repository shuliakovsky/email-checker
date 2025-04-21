package server

import (
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/shuliakovsky/email-checker/docs"
	"github.com/shuliakovsky/email-checker/internal/storage"
)

// Represents task status information for API responses
type TaskStatusResponse struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"total_results"`
	CreatedAt    time.Time `json:"created_at"`
	TotalPages   int       `json:"total_pages,omitempty"`
}

// Core server structure holding dependencies and configuration
type Server struct {
	storage     storage.Storage
	redisClient redis.UniversalClient
	port        string
	maxWorkers  int
	clusterMode bool
}

// response writer
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}
