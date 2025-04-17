package mx

import (
	"net"  // Provides network-related utilities
	"sync" // Synchronization primitives
)

var cache struct {
	sync.RWMutex                      // Read-write mutex for concurrency
	records      map[string][]*net.MX // Cache for MX records
}

func init() {
	cache.records = make(map[string][]*net.MX) // Initialize cache
}

func GetMXRecords(domain string) ([]*net.MX, error) {
	cache.RLock()                       // Acquire read lock
	cached, ok := cache.records[domain] // Check if domain is in cache
	cache.RUnlock()                     // Release read lock

	if ok {
		return cached, nil // Return cached records
	}

	records, err := net.LookupMX(domain) // Perform MX lookup
	if err != nil {
		return nil, err // Return error if lookup fails
	}

	cache.Lock()                    // Acquire write lock
	cache.records[domain] = records // Store records in cache
	cache.Unlock()                  // Release write lock

	return records, nil // Return retrieved records
}
