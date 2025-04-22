// Package domains manages domain rotation for SMTP HELO/EHLO commands
package domains

import (
	"context"
	"sync/atomic"

	"github.com/go-redis/redis/v8"
)

// Counter interface for sequence generation
type Counter interface {
	Next() (uint64, error)
}

// In-memory counter using atomic operations
type MemoryCounter struct {
	value uint64 // Stores counter state with atomic access
}

// Atomically increment and return counter value
func (c *MemoryCounter) Next() (uint64, error) {
	return atomic.AddUint64(&c.value, 1), nil
}

// Redis-based distributed counter
type RedisCounter struct {
	client redis.UniversalClient // Redis client connection
	key    string                // Redis key for counter storage
}

// Increment Redis counter atomically
func (c *RedisCounter) Next() (uint64, error) {
	return c.client.Incr(context.Background(), c.key).Uint64()
}

var (
	// Predefined list of HELO domains for rotation
	domainsList = []string{
		"rover.info",
		"mailto.plus",
		"fexpost.com",
		"chitthi.in",
		"fextemp.com",
		"any.pink",
		"merepost.com",
	}

	// Active counter implementation (memory or Redis)
	counter Counter
)

// Initialize counter based on deployment mode
func Init(isClusterMode bool, redisClient redis.UniversalClient) {
	if isClusterMode && redisClient != nil {
		// Use Redis counter for clustered deployments
		counter = &RedisCounter{
			client: redisClient,
			key:    "helo_domain_counter", // Shared Redis key for coordination
		}
	} else {
		// Use in-memory counter for single instance
		counter = &MemoryCounter{}
	}
}

// Get next rotated domain using modulo distribution
func GetNext() (string, error) {
	n, err := counter.Next() // Get sequence number
	if err != nil {
		return "", err // Propagate counter errors
	}

	// Rotate through domains using modulus
	return domainsList[n%uint64(len(domainsList))], nil
}
