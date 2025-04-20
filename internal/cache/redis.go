package cache

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"time"

	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/pkg/types"
)

// RedisCache implements cache.Provider interface using Redis as backend
type RedisCache struct {
	client redis.UniversalClient
}

// Creates new Redis-based cache instance with specified Redis client
func NewRedisCache(client redis.UniversalClient) *RedisCache {
	return &RedisCache{client: client}
}

// Retrieves cached value by key and unmarshals it into EmailReport struct
// Returns (value, true) on success or (nil, false) for missing/invalid entries
func (r *RedisCache) Get(key string) (interface{}, bool) {
	ctx := context.Background()
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false
	}

	var report types.EmailReport
	if err := json.Unmarshal([]byte(val), &report); err != nil {
		return nil, false
	}
	return report, true
}

// Stores value in Redis with JSON serialization and specified TTL expiration
// Uses best-effort approach - errors during marshaling/insertion are ignored
func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) {
	ctx := context.Background()
	data, _ := json.Marshal(value)
	r.client.Set(ctx, key, data, ttl)
}

// Clears all entries in Redis database using FLUSHDB command
// Logs operation but doesn't return success/failure status
func (r *RedisCache) Flush() {
	ctx := context.Background()
	logger.Log("Flushing Redis cache")
	r.client.FlushDB(ctx)
}

// Returns basic cache statistics (item count only)
// Memory usage and hit/miss metrics not implemented for Redis
func (r *RedisCache) GetStats() Stats {
	ctx := context.Background()

	size, _ := r.client.DBSize(ctx).Result()
	return Stats{
		Items:  int(size), // Total keys in database
		Memory: -1,        // Memory stats require Redis MEMORY USAGE command
		Hits:   0,         // Hit tracking needs Lua script implementation
		Misses: 0,         // Miss tracking needs Lua script implementation
	}
}
