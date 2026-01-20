package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 6),
		},
		[]string{"method", "path"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 6),
		},
		[]string{"method", "path"},
	)

	// Registry-specific metrics
	RegistrySyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "registry_sync_duration_seconds",
			Help:    "Duration of registry sync operations",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
	)

	RegistrySyncErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "registry_sync_errors_total",
			Help: "Total number of registry sync errors",
		},
	)

	RegistryCacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "registry_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	RegistryCacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "registry_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	RegistryServersTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_servers_total",
			Help: "Total number of servers in the registry",
		},
	)

	RegistryCacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_cache_size",
			Help: "Current size of the cache",
		},
	)

	RegistryIndexValid = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "registry_index_valid",
			Help: "Whether the index is valid (1) or not (0)",
		},
	)
)

// Metrics returns a middleware that records Prometheus metrics
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Record request size
		if r.ContentLength > 0 {
			httpRequestSize.WithLabelValues(r.Method, normalizePath(r.URL.Path)).Observe(float64(r.ContentLength))
		}

		next.ServeHTTP(ww, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())
		path := normalizePath(r.URL.Path)

		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		httpResponseSize.WithLabelValues(r.Method, path).Observe(float64(ww.BytesWritten()))
	})
}

// normalizePath normalizes URL paths for metrics labels
// This prevents cardinality explosion from dynamic path segments
func normalizePath(path string) string {
	// Map common dynamic paths to static labels
	switch {
	case len(path) > 20 && path[:12] == "/v0.1/servers" && path != "/v0.1/servers":
		return "/v0.1/servers/{serverName}"
	case len(path) > 15 && path[:10] == "/v0/servers" && path != "/v0/servers":
		return "/v0/servers/{serverName}"
	default:
		return path
	}
}
