package mx

import (
	"context" // Defines the Context type
	"fmt"     // Formatted output
	"net"     // Provides network-related utilities
	"sync"    // Synchronization primitives
	"time"    // For handling time durations and delays
)

var (
	cache struct {
		sync.RWMutex                      // Read-write mutex for concurrency
		records      map[string][]*net.MX // Cache for MX records
	}
	resolver *net.Resolver // Custom DNS resolver
)

func init() {
	cache.records = make(map[string][]*net.MX) // Initialize cache
}

// InitResolver Initialise a custom DNS resolver
func InitResolver(dnsServer string) {
	// Create a new resolver instance with custom settings.
	resolver = &net.Resolver{
		PreferGo: true, // Prefer the Go implementation of DNS resolution instead of the system one !!!
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Configure a dialer with a timeout of 2 seconds for making connections.
			d := net.Dialer{
				Timeout: 2 * time.Second,
			}

			// List of protocols to attempt for the connection: UDP and TCP.
			protocols := []string{"udp", "tcp"} // Loop through the protocols (UDP and TCP).
			for _, proto := range protocols {
				conn, err := d.DialContext(ctx, proto, net.JoinHostPort(dnsServer, "53"))
				if err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("failed to connect via UDP and TCP") // If both UDP and TCP connections fail, return an error.
		},
	}
}

func GetMXRecords(domain string) ([]*net.MX, error) {
	cache.RLock()                       // Acquire read lock
	cached, ok := cache.records[domain] // Check if domain is in cache
	cache.RUnlock()                     // Release read lock

	if ok {
		return cached, nil // Return cached records
	}

	records, err := resolver.LookupMX(context.Background(), domain) // Perform MX lookup
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err // Return error if lookup fails
	}

	cache.Lock()                    // Acquire write lock
	cache.records[domain] = records // Store records in cache
	cache.Unlock()                  // Release write lock

	return records, nil // Return retrieved records
}
