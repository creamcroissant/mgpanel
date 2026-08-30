// Package middleware provides HTTP middleware components.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsConfig holds configuration for the metrics middleware.
type MetricsConfig struct {
	// Namespace is the prefix for all metrics (default: "xboard")
	Namespace string
	// Subsystem is an optional subsystem name
	Subsystem string
	// SkipPaths are paths that should not be tracked
	SkipPaths []string
	// Buckets defines the histogram buckets for request duration
	Buckets []float64
}

// DefaultMetricsConfig returns the default metrics configuration.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Namespace: "xboard",
		Subsystem: "http",
		SkipPaths: []string{"/health", "/healthz", "/_internal/ready", "/metrics"},
		Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}
}

// Metrics holds the Prometheus metrics collectors.
type Metrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
	responseSize     *prometheus.HistogramVec
}

// NewMetrics creates a new Metrics instance with the given configuration.
func NewMetrics(cfg MetricsConfig) *Metrics {
	if cfg.Namespace == "" {
		cfg.Namespace = "xboard"
	}
	if cfg.Subsystem == "" {
		cfg.Subsystem = "http"
	}
	if len(cfg.Buckets) == 0 {
		cfg.Buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}

	return &Metrics{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed.",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "request_duration_seconds",
				Help:      "Request latency in seconds.",
				Buckets:   cfg.Buckets,
			},
			[]string{"method", "path"},
		),
		requestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "requests_in_flight",
				Help:      "Current number of requests being served.",
			},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "response_size_bytes",
				Help:      "Response size in bytes.",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
			},
			[]string{"method", "path"},
		),
	}
}

// metricsResponseWriter wraps http.ResponseWriter to capture status and size.
type metricsResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// Middleware returns an HTTP middleware that collects metrics.
func (m *Metrics) Middleware(cfg MetricsConfig) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool)
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics for certain paths
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Normalize path for metrics to avoid high cardinality
			path := normalizePath(r.URL.Path)

			m.requestsInFlight.Inc()
			defer m.requestsInFlight.Dec()

			start := time.Now()
			wrapped := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()
			statusStr := strconv.Itoa(wrapped.status)

			m.requestsTotal.WithLabelValues(r.Method, path, statusStr).Inc()
			m.requestDuration.WithLabelValues(r.Method, path).Observe(duration)
			m.responseSize.WithLabelValues(r.Method, path).Observe(float64(wrapped.size))
		})
	}
}

// normalizePath replaces dynamic path segments with placeholders to reduce cardinality.
func normalizePath(path string) string {
	// Simple normalization: replace UUIDs and numeric IDs with placeholders
	// This prevents metric explosion from high-cardinality path params
	// For now, just return the path as-is; more sophisticated normalization can be added later
	return path
}

// MetricsGuard validates the metrics token
func MetricsGuard(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
