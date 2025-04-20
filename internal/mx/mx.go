package mx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	// Cache structure for storing MX records with read-write mutex for thread-safe operations
	cache struct {
		sync.RWMutex                      // Mutex for ensuring thread safety during cache access
		records      map[string][]*net.MX // Map to store MX records by domain
	}
	resolver *net.Resolver // Custom DNS resolver for handling MX record lookups
)

func init() {
	// Initialize the cache for storing MX records
	cache.records = make(map[string][]*net.MX)
}

// InitResolver configures and initializes a custom DNS resolver
func InitResolver(dnsServer string) {
	// Create a resolver instance with custom dialing functionality
	resolver = &net.Resolver{
		PreferGo: true, // Prefer using Go's DNS resolution over the system's implementation
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Configure a dialer with a timeout for establishing connections
			d := net.Dialer{
				Timeout: 2 * time.Second,
			}

			// Attempt to connect using both UDP and TCP protocols
			protocols := []string{"udp", "tcp"}
			for _, proto := range protocols {
				conn, err := d.DialContext(ctx, proto, net.JoinHostPort(dnsServer, "53")) // Connect to port 53 (DNS)
				if err == nil {
					return conn, nil // Return the connection if successful
				}
			}

			// Return an error if neither UDP nor TCP connections succeed
			return nil, fmt.Errorf("failed to connect via UDP and TCP")
		},
	}
}

// GetMXRecords retrieves MX records for the given domain
func GetMXRecords(domain string) ([]*net.MX, error) {
	// Check the cache for existing MX records
	cache.RLock()                       // Acquire read lock for safe cache access
	cached, ok := cache.records[domain] // Look up the domain in the cache
	cache.RUnlock()                     // Release the read lock

	if ok {
		return cached, nil // Return cached MX records if available
	}

	// Perform MX record lookup using the custom DNS resolver
	records, err := resolver.LookupMX(context.Background(), domain)
	if err != nil {
		return nil, err // Return an error if the lookup fails
	}

	// Store the retrieved MX records in the cache
	cache.Lock()                    // Acquire write lock for updating the cache
	cache.records[domain] = records // Save the MX records under the domain key
	cache.Unlock()                  // Release the write lock

	return records, nil // Return the retrieved MX records
}
