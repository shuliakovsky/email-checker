package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	EmailsChecked = promauto.NewCounter(prometheus.CounterOpts{
		Name: "emails_checked_total",
		Help: "Total emails processed",
	})

	CacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total cache hits",
	})

	CacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total cache misses",
	})

	MXCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mx_cache_hits_total",
		Help: "MX records cache hits",
	})

	MXCacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mx_cache_misses_total",
		Help: "MX records cache misses",
	})
	WebhookAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_attempts_total",
		Help: "Total webhook delivery attempts",
	}, []string{"task_id", "status"})

	WebhookRetries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "webhook_retries_total",
		Help: "Total webhook retry attempts",
	})

	WebhookLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "webhook_latency_seconds",
		Help:    "Webhook delivery latency distribution",
		Buckets: prometheus.DefBuckets,
	})
	ThrottledDomains = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smtp_throttled_domains_total",
		Help: "Total number of throttled domains",
	})

	RetryAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smtp_retry_attempts_total",
		Help: "Total email retry attempts",
	}, []string{"email", "attempt"})

	TemporaryErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smtp_temporary_errors_total",
		Help: "Total temporary SMTP errors",
	}, []string{"domain"})

	RBLRestrictions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smtp_rbl_restrictions_total",
		Help: "Total RBL restriction errors",
	})
	APIKeyChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "apikey_checks_total",
		Help: "Total checks per API key",
	}, []string{"key", "type"})

	APIKeyQuota = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "apikey_remaining_quota",
		Help: "Remaining checks per API key",
	}, []string{"key"})
)
