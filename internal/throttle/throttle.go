package throttle

import (
	"fmt"
	"github.com/shuliakovsky/email-checker/internal/metrics"
	"time"

	"github.com/shuliakovsky/email-checker/internal/cache"
	"github.com/shuliakovsky/email-checker/internal/logger"
)

const (
	ThrottleTTL = 60 * time.Second // Domain blocking time
	MaxRetries  = 3                // Maximum number of retries
)

type ThrottleManager struct {
	cache cache.Provider
}

func NewThrottleManager(cache cache.Provider) *ThrottleManager {
	return &ThrottleManager{cache: cache}
}

// IsThrottled check if domain is blocked (throttled)
func (tm *ThrottleManager) IsThrottled(domain string) bool {
	_, ok := tm.cache.Get("throttle:" + domain)
	return ok
}

// ThrottleDomain block domain for configured time
func (tm *ThrottleManager) ThrottleDomain(domain string) {
	tm.cache.Set("throttle:"+domain, struct{}{}, ThrottleTTL)
	logger.Log(fmt.Sprintf("[Throttle] Domain %s throttled for %v", domain, ThrottleTTL))
}

// ScheduleRetry Plan next check for email (domain) with a delay
func (tm *ThrottleManager) ScheduleRetry(email string, attempt int) {
	metrics.RetryAttempts.WithLabelValues(email, fmt.Sprintf("%d", attempt)).Inc()
	delay := getRetryDelay(attempt)
	key := fmt.Sprintf("retry:%s:%d", email, attempt)
	tm.cache.Set(key, email, delay)
}

// GetRetryDelay return retry with delay
func getRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 10 * time.Second
	case 2:
		return 20 * time.Second
	case 3:
		return 30 * time.Second
	default:
		return 30 * time.Second
	}
}
