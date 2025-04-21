package mx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/internal/metrics"
)

// Package mx provides DNS MX record lookup with caching capabilities
var (
	// Local in-memory cache for MX records with thread-safe access
	localCache struct {
		sync.RWMutex
		records map[string][]*net.MX
	}

	// Distributed cache provider for cross-instance caching (e.g., Redis)
	cacheProvider cache.Provider

	// Custom DNS resolver instance
	resolver *net.Resolver
)

// Initialize local cache storage
func init() {
	localCache.records = make(map[string][]*net.MX)
}

// Configures DNS resolver with custom DNS server address
func InitResolver(dnsServer string) {
	resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 2 * time.Second}
			// Attempt connection using both UDP and TCP protocols
			for _, proto := range []string{"udp", "tcp"} {
				conn, err := dialer.DialContext(ctx, proto, net.JoinHostPort(dnsServer, "53"))
				if err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("failed to connect to DNS server")
		},
	}
}

// Sets the distributed cache provider for MX records
func SetCacheProvider(provider cache.Provider) {
	cacheProvider = provider
}

// Retrieves MX records for domain with caching strategy:
// 1. Check distributed cache
// 2. Check local in-memory cache
// 3. Perform DNS lookup
// 4. Cache results in both layers
func GetMXRecords(domain string) ([]*net.MX, error) {
	// First check distributed cache if available
	if cacheProvider != nil {
		if cached, ok := cacheProvider.Get("mx:" + domain); ok {
			metrics.MXCacheHits.Inc()
			return cached.([]*net.MX), nil
		}
		metrics.MXCacheMisses.Inc()
	}

	// Then check local in-memory cache
	localCache.RLock()
	cached, ok := localCache.records[domain]
	localCache.RUnlock()
	if ok {
		return cached, nil
	}

	// Perform actual DNS MX lookup
	records, err := resolver.LookupMX(context.Background(), domain)
	if err != nil {
		return nil, fmt.Errorf("MX lookup failed: %w", err)
	}

	// Update local cache with write lock
	localCache.Lock()
	localCache.records[domain] = records
	localCache.Unlock()

	// Update distributed cache if available
	if cacheProvider != nil {
		cacheProvider.Set("mx:"+domain, records, time.Hour)
	}

	return records, nil
}
