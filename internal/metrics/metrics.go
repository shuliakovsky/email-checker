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
)
