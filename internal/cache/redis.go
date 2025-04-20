package cache

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"time"

	"github.com/shuliakovsky/email-checker/pkg/types" // Package containing custom types, like EmailReport
)

// RedisCache is a cache provider using Redis as the backend storage
type RedisCache struct {
	client *redis.Client // Redis client instance for communicating with the Redis server
}

// NewRedisCache initializes and returns a new RedisCache instance
func NewRedisCache(addr, password string, db int) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Address of the Redis server
		Password: password, // Password for Redis authentication
		DB:       db,       // Redis database number
	})
	return &RedisCache{client: client}
}

// Get retrieves a cached value by its key. Returns false if the key doesn't exist or deserialization fails.
func (r *RedisCache) Get(key string) (interface{}, bool) {
	ctx := context.Background()                 // Create a context for the Redis operation
	val, err := r.client.Get(ctx, key).Result() // Retrieve the value associated with the key
	if err == redis.Nil {                       // If the key doesn't exist in Redis
		return nil, false
	}

	// Deserialize the cached value into the EmailReport struct
	var report types.EmailReport
	if err := json.Unmarshal([]byte(val), &report); err != nil { // Handle deserialization errors
		return nil, false
	}
	return report, true // Return the deserialized object and a success flag
}

// Set stores a value in the Redis cache with the specified key and TTL (time-to-live)
func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) {
	ctx := context.Background()       // Create a context for the Redis operation
	data, _ := json.Marshal(value)    // Serialize the value into JSON format
	r.client.Set(ctx, key, data, ttl) // Store the serialized value in Redis with a TTL
}

// Flush clears all data in the Redis database
func (r *RedisCache) Flush() {
	ctx := context.Background() // Create a context for the Redis operation
	r.client.FlushDB(ctx)       // Execute the Redis flush command to clear the database
}
