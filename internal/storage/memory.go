package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/shuliakovsky/email-checker/internal/cache" // Cache provider interface
	"github.com/shuliakovsky/email-checker/pkg/types"      // Custom types for tasks and other entities
)

// MemoryStorage is an in-memory implementation of the Storage interface
type MemoryStorage struct {
	mu    sync.RWMutex           // Read-write mutex to ensure thread-safe access
	tasks map[string]*types.Task // Map for storing tasks by their unique IDs
	queue []*types.Task          // Task queue for local processing mode
	cache cache.Provider         // Cache provider instance for secondary caching
}

// NewMemoryStorage creates a new instance of MemoryStorage
func NewMemoryStorage(cache cache.Provider) *MemoryStorage {
	return &MemoryStorage{
		tasks: make(map[string]*types.Task), // Initialize the task map
		cache: cache,                        // Assign the provided cache provider
	}
}

// GetCacheProvider returns the cache provider instance
func (m *MemoryStorage) GetCacheProvider() cache.Provider {
	return m.cache
}

// SaveTask stores a task in memory, overwriting any existing task with the same ID
func (m *MemoryStorage) SaveTask(ctx context.Context, task *types.Task) error {
	m.mu.Lock()             // Acquire write lock for thread-safe access
	defer m.mu.Unlock()     // Release lock after operation
	m.tasks[task.ID] = task // Save or update the task in the map
	return nil              // Return nil to indicate successful storage
}

// GetTask retrieves a task by ID from memory
func (m *MemoryStorage) GetTask(ctx context.Context, id string) (*types.Task, error) {
	m.mu.RLock()                // Acquire read lock for thread-safe access
	defer m.mu.RUnlock()        // Release lock after operation
	task, exists := m.tasks[id] // Check if the task exists in the map
	if !exists {
		return nil, fmt.Errorf("task not found") // Return error if the task is not found
	}
	return task, nil // Return the retrieved task
}

// UpdateTask updates an existing task in memory by overwriting it
func (m *MemoryStorage) UpdateTask(ctx context.Context, task *types.Task) error {
	return m.SaveTask(ctx, task) // Use SaveTask for updating logic
}

// DequeueTask removes and returns the first task from the in-memory queue
func (m *MemoryStorage) DequeueTask() (*types.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		return nil, fmt.Errorf("no tasks in queue")
	}

	task := m.queue[0]
	m.queue = m.queue[1:]
	return task, nil
}

// EnqueueTask adds a task to the end of the in-memory queue
func (m *MemoryStorage) EnqueueTask(task *types.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, task)
	return nil
}
