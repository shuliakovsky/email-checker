package cache

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"time"

	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

// RedisCache is a cache provider using Redis for backend storage
type RedisCache struct {
	client *redis.Client // Redis client instance used for communication with the Redis server
}

// NewRedisCache creates and initializes a new RedisCache instance
func NewRedisCache(addr, password string, db int) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis server address
		Password: password, // Authentication password for the Redis server
		DB:       db,       // Redis database number to use
	})
	return &RedisCache{client: client}
}

// Get retrieves a cached value using its key. If the key does not exist or deserialization fails, it returns false.
func (r *RedisCache) Get(key string) (interface{}, bool) {
	ctx := context.Background()                 // Create a context for executing the Redis operation
	val, err := r.client.Get(ctx, key).Result() // Retrieve the value associated with the key from Redis
	if err == redis.Nil {                       // Check if the key does not exist in Redis
		return nil, false
	}

	// Attempt to deserialize the cached value into the EmailReport struct
	var report types.EmailReport
	if err := json.Unmarshal([]byte(val), &report); err != nil { // Handle deserialization errors
		return nil, false
	}
	return report, true // Return the deserialized object and a success flag
}

// Set stores a value in the Redis cache with the specified key and TTL (time-to-live)
func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) {
	ctx := context.Background()       // Create a context for executing the Redis operation
	data, _ := json.Marshal(value)    // Serialize the value into JSON format for Redis storage
	r.client.Set(ctx, key, data, ttl) // Store the serialized data in Redis with a time-to-live (TTL) setting
}

// Flush clears all data stored in the Redis database
func (r *RedisCache) Flush() {
	ctx := context.Background()        // Create a context for executing the Redis flush operation
	logger.Log("Flushing Redis cache") // Log a message indicating the cache is being cleared
	r.client.FlushDB(ctx)              // Execute the Redis command to clear all entries in the database
}

// GetStats retrieves statistics about the current state of the Redis cache
func (r *RedisCache) GetStats() Stats {
	ctx := context.Background() // Create a context for retrieving the cache statistics

	size, _ := r.client.DBSize(ctx).Result() // Get the number of keys in the Redis database
	// For Redis, precise memory usage information is not directly available without external tools
	return Stats{
		Items:  int(size), // The number of items stored in the Redis database
		Memory: -1,        // Memory usage is marked as unavailable (-1)
		Hits:   0,         // Hits tracking requires implementation via Lua scripts
		Misses: 0,         // Misses tracking requires implementation via Lua scripts
	}
}
