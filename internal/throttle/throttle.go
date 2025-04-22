// Package throttle implements domain-based request throttling
package throttle

import (
	"fmt"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/internal/logger"
	"github.com/shuliakovsky/email-checker/internal/metrics"
)

const (
	ThrottleTTL = 60 * time.Second // Default domain block duration
	MaxRetries  = 3                // Max allowed retry attempts per email
)

// Central throttling controller with cache backend
type ThrottleManager struct {
	cache cache.Provider // Storage for throttle states and retry schedules
}

// Creates new manager with specified cache provider
func NewThrottleManager(cache cache.Provider) *ThrottleManager {
	return &ThrottleManager{cache: cache}
}

// Check if domain is currently blocked
func (tm *ThrottleManager) IsThrottled(domain string) bool {
	_, ok := tm.cache.Get("throttle:" + domain) // Cache key format: throttle:<domain>
	return ok
}

// Block domain with default TTL (60s)
func (tm *ThrottleManager) ThrottleDomain(domain string) {
	tm.cache.Set("throttle:"+domain, struct{}{}, ThrottleTTL)
	logger.Log(fmt.Sprintf("[Throttle] Domain %s throttled for %v", domain, ThrottleTTL))
}

// Schedule email retry with attempt-specific delay
func (tm *ThrottleManager) ScheduleRetry(email string, attempt int) {
	metrics.RetryAttempts.WithLabelValues(email, fmt.Sprintf("%d", attempt)).Inc()
	delay := getRetryDelay(attempt) // Get attempt-based delay
	key := fmt.Sprintf("retry:%s:%d", email, attempt)
	tm.cache.Set(key, email, delay) // Store retry schedule
}

// Block domain with custom TTL duration
func (tm *ThrottleManager) ThrottleDomainWithTTL(domain string, ttl time.Duration) {
	tm.cache.Set("throttle:"+domain, struct{}{}, ttl)
	logger.Log(fmt.Sprintf("[Throttle] Domain %s throttled for %v", domain, ttl))
}

// Get delay duration based on attempt number
func getRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1: // First retry after 10s
		return 10 * time.Second
	case 2: // Second retry after 20s
		return 20 * time.Second
	case 3: // Third retry after 30s
		return 30 * time.Second
	default: // All subsequent retries every 30s
		return 30 * time.Second
	}
}
