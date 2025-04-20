package storage

import (
	"context"

	"github.com/shuliakovsky/email-checker/internal/cache" // Cache provider interface
	"github.com/shuliakovsky/email-checker/pkg/types"      // Custom types for tasks and other entities
)

// Storage defines the interface for persistence operations related to tasks
type Storage interface {
	// Saves a task to persistent storage
	SaveTask(ctx context.Context, task *types.Task) error

	// Retrieves a task by its unique identifier
	GetTask(ctx context.Context, id string) (*types.Task, error)

	// Updates an existing task in storage
	UpdateTask(ctx context.Context, task *types.Task) error

	// Provides access to the cache layer instance
	GetCacheProvider() cache.Provider

	// Adds task to processing queue (local mode without context)
	EnqueueTask(task *types.Task) error

	// Retrieves and removes task from queue (local mode blocking pop)
	DequeueTask() (*types.Task, error)
}
