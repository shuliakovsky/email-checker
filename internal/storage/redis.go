package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

// Redis key identifier for the task queue
const (
	TaskQueueKey = "email_checker:tasks"
)

// RedisStorage implements storage operations using Redis
type RedisStorage struct {
	client redis.UniversalClient
	cache  cache.Provider
}

// Creates new RedisStorage instance with specified Redis client
func NewRedisStorage(client redis.UniversalClient) *RedisStorage {
	return &RedisStorage{
		client: client,
		cache:  cache.NewRedisCache(client),
	}
}

// Adds task to the processing queue (LPUSH operation)
func (r *RedisStorage) EnqueueTask(task *types.Task) error {
	data, _ := json.Marshal(task)
	return r.client.LPush(context.Background(), "email_checker:tasks", data).Err()
}

// Retrieves and removes task from queue using blocking pop (BRPOP)
func (r *RedisStorage) DequeueTask() (*types.Task, error) {
	result, err := r.client.BRPop(context.Background(), 0, "email_checker:tasks").Result()
	if err != nil {
		return nil, err
	}

	var task types.Task
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// GetCacheProvider returns the cache provider instance
func (r *RedisStorage) GetCacheProvider() cache.Provider {
	return r.cache
}

// SaveTask saves a task to Redis storage with 24-hour expiration
func (r *RedisStorage) SaveTask(ctx context.Context, task *types.Task) error {
	data, err := json.Marshal(task) // Serialize task into JSON format
	if err != nil {
		return err // Return error if serialization fails
	}
	return r.client.Set(ctx, "task:"+task.ID, data, 24*time.Hour).Err() // Store the task with a 24-hour TTL
}

// GetTask retrieves a task from Redis storage by its ID
// Returns error if task not found or deserialization fails
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

// UpdateTask updates an existing task in Redis storage by overwriting it
// Uses same storage logic as SaveTask with updated data
func (r *RedisStorage) UpdateTask(ctx context.Context, task *types.Task) error {
	return r.SaveTask(ctx, task) // Reuses SaveTask method to update the task
}
