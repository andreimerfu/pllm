package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Go runtime and process metrics are automatically registered by promhttp.Handler()
// so we don't need to register them explicitly here

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pllm_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pllm_http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "endpoint"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pllm_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "endpoint"},
	)

	// LLM specific metrics
	llmRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_llm_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"model", "provider", "endpoint", "status"},
	)

	llmRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pllm_llm_request_duration_seconds",
			Help:    "LLM request latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"model", "provider", "endpoint"},
	)

	llmTokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_llm_tokens_total",
			Help: "Total number of tokens used",
		},
		[]string{"model", "provider", "type"}, // type: prompt, completion, total
	)

	// Cache metrics
	cacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"endpoint"},
	)

	cacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"endpoint"},
	)

	cacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "pllm_cache_size_bytes",
			Help: "Current cache size in bytes",
		},
	)

	// Rate limit metrics
	rateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"endpoint"},
	)

	rateLimitAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_rate_limit_allowed_total",
			Help: "Total number of requests allowed by rate limiter",
		},
		[]string{"endpoint"},
	)

	// Active connections
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "pllm_active_connections",
			Help: "Number of active connections",
		},
	)

	// Model availability
	modelAvailability = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pllm_model_availability",
			Help: "Model availability status (1 = available, 0 = unavailable)",
		},
		[]string{"model", "instance", "provider"},
	)

	// Error metrics
	errorCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pllm_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "endpoint"},
	)
)

// MetricsMiddleware collects Prometheus metrics
func MetricsMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track active connections
			activeConnections.Inc()
			defer activeConnections.Dec()

			// Get the route pattern
			routePattern := getRoutePattern(r)

			// Track request size
			requestSize := computeRequestSize(r)
			httpRequestSize.WithLabelValues(r.Method, routePattern).Observe(float64(requestSize))

			// Use streaming-aware wrapper that preserves Flusher interface
			wrapped := NewStreamingResponseWriter(w)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Record metrics
			status := strconv.Itoa(wrapped.StatusCode())
			httpRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, routePattern, status).Observe(duration)
			httpResponseSize.WithLabelValues(r.Method, routePattern).Observe(float64(wrapped.BytesWritten()))

			// Log slow requests
			if duration > 10 {
				logger.Warn("Slow request detected",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Float64("duration", duration),
					zap.Int("status", wrapped.StatusCode()),
				)
			}
		})
	}
}

// RecordCacheHit records a cache hit
func RecordCacheHit(endpoint string) {
	cacheHits.WithLabelValues(endpoint).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(endpoint string) {
	cacheMisses.WithLabelValues(endpoint).Inc()
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit(endpoint string) {
	rateLimitHits.WithLabelValues(endpoint).Inc()
}

// RecordRateLimitAllowed records a request allowed by rate limiter
func RecordRateLimitAllowed(endpoint string) {
	rateLimitAllowed.WithLabelValues(endpoint).Inc()
}

// RecordLLMRequest records an LLM request
func RecordLLMRequest(model, provider, endpoint string, duration float64, status string) {
	llmRequestsTotal.WithLabelValues(model, provider, endpoint, status).Inc()
	if status == "success" {
		llmRequestDuration.WithLabelValues(model, provider, endpoint).Observe(duration)
	}
}

// RecordLLMTokens records token usage
func RecordLLMTokens(model, provider string, promptTokens, completionTokens, totalTokens float64) {
	llmTokensUsed.WithLabelValues(model, provider, "prompt").Add(promptTokens)
	llmTokensUsed.WithLabelValues(model, provider, "completion").Add(completionTokens)
	llmTokensUsed.WithLabelValues(model, provider, "total").Add(totalTokens)
}

// RecordError records an error
func RecordError(errorType, endpoint string) {
	errorCount.WithLabelValues(errorType, endpoint).Inc()
}

// UpdateModelAvailability updates model availability status
func UpdateModelAvailability(model, instance, provider string, available bool) {
	value := 0.0
	if available {
		value = 1.0
	}
	modelAvailability.WithLabelValues(model, instance, provider).Set(value)
}

// UpdateCacheSize updates the cache size metric
func UpdateCacheSize(size int64) {
	cacheSize.Set(float64(size))
}

// Helper functions

func getRoutePattern(r *http.Request) string {
	// Try to get the route pattern from chi context
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}

	// Fallback to normalizing the path
	return normalizePath(r.URL.Path)
}

func normalizePath(path string) string {
	// Normalize common patterns
	if strings.HasPrefix(path, "/v1/chat/completions") {
		return "/v1/chat/completions"
	}
	if strings.HasPrefix(path, "/v1/completions") {
		return "/v1/completions"
	}
	if strings.HasPrefix(path, "/v1/embeddings") {
		return "/v1/embeddings"
	}
	if strings.HasPrefix(path, "/v1/models") {
		return "/v1/models"
	}
	if strings.HasPrefix(path, "/health") {
		return "/health"
	}
	if strings.HasPrefix(path, "/ready") {
		return "/ready"
	}
	if strings.HasPrefix(path, "/metrics") {
		return "/metrics"
	}

	// For other paths, remove IDs and parameters
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Replace UUIDs, numbers, and common ID patterns with placeholders
		if len(part) > 0 && (isUUID(part) || isNumeric(part) || isID(part)) {
			parts[i] = "{id}"
		}
	}

	return strings.Join(parts, "/")
}

func computeRequestSize(r *http.Request) int64 {
	size := int64(0)

	// Add method and URL
	size += int64(len(r.Method))
	size += int64(len(r.URL.String()))

	// Add headers
	for name, values := range r.Header {
		size += int64(len(name))
		for _, value := range values {
			size += int64(len(value))
		}
	}

	// Add content length if available
	if r.ContentLength > 0 {
		size += r.ContentLength
	}

	return size
}

func isUUID(s string) bool {
	// Simple UUID check (32 hex chars with optional hyphens)
	if len(s) < 32 || len(s) > 36 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isID(s string) bool {
	// Common ID patterns
	return strings.HasPrefix(s, "sk-") || strings.HasPrefix(s, "pk-") ||
		strings.HasPrefix(s, "key-") || strings.HasPrefix(s, "usr-") ||
		strings.HasPrefix(s, "grp-") || strings.HasPrefix(s, "org-")
}
