package cache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/shuliakovsky/email-checker/internal/logger"
)

// Provider defines the interface for a cache provider
type Provider interface {
	Get(key string) (interface{}, bool)                   // Retrieve a value by key; returns false if the key is not found or the item has expired
	Set(key string, value interface{}, ttl time.Duration) // Store a value with a specific key and a time-to-live (TTL)
	Flush()                                               // Remove all items from the cache
	GetStats() Stats                                      // Retrieve statistics about the current state of the cache
}

// Stats contains statistical data about the cache
type Stats struct {
	Items  int   // Number of items currently stored in the cache
	Memory int64 // Total memory used by the cache (in bytes)
	Hits   int64 // Number of successful retrievals (cache hits)
	Misses int64 // Number of failed retrievals (cache misses)
}

// InMemoryCache is an in-memory implementation of the cache provider
type InMemoryCache struct {
	mu          sync.RWMutex         // Mutex for thread-safe access to the cache
	items       map[string]cacheItem // Map holding the cache items
	statsHits   int64                // Counter for successful cache hits
	statsMisses int64                // Counter for failed cache misses
}

// cacheItem represents an individual cache entry
type cacheItem struct {
	value    interface{} // The stored value of the cache item
	expireAt time.Time   // The expiration time for the cache item
}

// NewInMemoryCache creates and initializes a new instance of InMemoryCache
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		items: make(map[string]cacheItem), // Initialize the map to store cache entries
	}
}

// Get retrieves a cache item by its key. Returns false if the item does not exist or has expired.
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()         // Acquire a read lock
	defer c.mu.RUnlock() // Release the read lock when the function exits

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expireAt) {
		atomic.AddInt64(&c.statsMisses, 1) // Increment the miss counter
		return nil, false
	}

	atomic.AddInt64(&c.statsHits, 1) // Increment the hit counter
	return item.value, true
}

// Set adds a new cache item or updates an existing one with the given key, value, and TTL
func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()         // Acquire a write lock
	defer c.mu.Unlock() // Release the write lock when the function exits

	c.items[key] = cacheItem{
		value:    value,               // Store the provided value
		expireAt: time.Now().Add(ttl), // Calculate and set the expiration time based on the provided TTL
	}
}

// Flush clears all items from the cache
func (c *InMemoryCache) Flush() {
	c.mu.Lock()                                                      // Acquire a write lock
	defer c.mu.Unlock()                                              // Release the write lock when the function exits
	logger.Log(fmt.Sprintf("Flushing %d cache items", len(c.items))) // Log the number of items being flushed
	c.items = make(map[string]cacheItem)                             // Reset the map to clear all cache entries
}

// GetStats retrieves the current statistics of the cache
func (c *InMemoryCache) GetStats() Stats {
	c.mu.RLock()         // Acquire a read lock
	defer c.mu.RUnlock() // Release the read lock when the function exits

	return Stats{
		Items:  len(c.items),                     // Count the number of items in the cache
		Memory: int64(unsafe.Sizeof(c.items)),    // Calculate the memory usage of the cache (approximation)
		Hits:   atomic.LoadInt64(&c.statsHits),   // Retrieve the number of successful cache hits
		Misses: atomic.LoadInt64(&c.statsMisses), // Retrieve the number of cache misses
	}
}
