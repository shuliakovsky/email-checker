package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache" // Cache provider interface
	"github.com/shuliakovsky/email-checker/pkg/types"      // Defines custom types like Task
)

// RedisStorage implements a storage mechanism backed by Redis
type RedisStorage struct {
	client *redis.Client  // Redis client instance
	cache  cache.Provider // Cache provider interface for secondary caching
}

// NewRedisStorage initializes a RedisStorage instance with connection details
func NewRedisStorage(addr, password string, db int, cacheProvider cache.Provider) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis server address
		Password: password, // Redis server password
		DB:       db,       // Redis database number
	})

	// Test connection to Redis
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // Set timeout for connection
	defer cancel()
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("error connecting to Redis: %w", err) // Return error if connection fails
	}

	return &RedisStorage{
		client: client,        // Assign Redis client instance
		cache:  cacheProvider, // Assign cache provider instance
	}, nil
}

// GetCacheProvider returns the cache provider instance
func (r *RedisStorage) GetCacheProvider() cache.Provider {
	return r.cache
}

// SaveTask saves a task to Redis storage
func (r *RedisStorage) SaveTask(ctx context.Context, task *types.Task) error {
	data, err := json.Marshal(task) // Serialize task into JSON format
	if err != nil {
		return err // Return error if serialization fails
	}
	return r.client.Set(ctx, "task:"+task.ID, data, 24*time.Hour).Err() // Store the task with a 24-hour TTL
}

// GetTask retrieves a task from Redis storage by its ID
func (r *RedisStorage) GetTask(ctx context.Context, id string) (*types.Task, error) {
	data, err := r.client.Get(ctx, "task:"+id).Bytes() // Fetch task data by key
	if err == redis.Nil {
		return nil, fmt.Errorf("task not found") // Return error if key is not found
	} else if err != nil {
		return nil, err // Return other Redis-related errors
	}

	var task types.Task
	if err := json.Unmarshal(data, &task); err != nil { // Deserialize JSON data into a Task struct
		return nil, err
	}
	return &task, nil // Return the deserialized task
}

// UpdateTask updates an existing task in Redis storage
func (r *RedisStorage) UpdateTask(ctx context.Context, task *types.Task) error {
	return r.SaveTask(ctx, task) // Reuses SaveTask method to update the task
}
