package cache

import (
	"sync"
	"time"
)

// Provider defines the interface for a cache provider
type Provider interface {
	Get(key string) (interface{}, bool)                   // Retrieve a value by key, returns false if not found or expired
	Set(key string, value interface{}, ttl time.Duration) // Store a value with a key and a time-to-live (TTL)
	Flush()                                               // Clear all items from the cache
}

// InMemoryCache is an in-memory implementation of the cache provider
type InMemoryCache struct {
	mu    sync.RWMutex         // Mutex for managing concurrent access to the cache
	items map[string]cacheItem // Map to store cache items
}

// cacheItem represents an individual item in the cache
type cacheItem struct {
	value    interface{} // The actual value of the cache item
	expireAt time.Time   // Expiration time of the cache item
}

// NewInMemoryCache creates and returns a new instance of InMemoryCache
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		items: make(map[string]cacheItem), // Initialize the map to store items
	}
}

// Get retrieves a cache item by its key. If the key doesn't exist or the item has expired, it returns false.
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()         // Acquire a read lock
	defer c.mu.RUnlock() // Release the read lock when the function exits

	item, ok := c.items[key] // Check if the key exists in the cache
	if !ok || time.Now().After(item.expireAt) {
		return nil, false // Return nil and false if the item doesn't exist or has expired
	}
	return item.value, true // Return the item's value and true if found and valid
}

// Set stores a new cache item with the specified key, value, and TTL
func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()         // Acquire a write lock
	defer c.mu.Unlock() // Release the write lock when the function exits

	c.items[key] = cacheItem{
		value:    value,               // Store the provided value
		expireAt: time.Now().Add(ttl), // Set the expiration time based on the TTL
	}
}

// Flush removes all items from the cache
func (c *InMemoryCache) Flush() {
	c.mu.Lock()         // Acquire a write lock
	defer c.mu.Unlock() // Release the write lock when the function exits

	c.items = make(map[string]cacheItem) // Clear all items in the cache
}
