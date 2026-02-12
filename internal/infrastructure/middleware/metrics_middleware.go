package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AsyncMetricsMiddleware emits metric events without blocking requests
type AsyncMetricsMiddleware struct {
	emitter *metrics.MetricEventEmitter
	logger  *zap.Logger
}

// NewAsyncMetricsMiddleware creates a new async metrics middleware
func NewAsyncMetricsMiddleware(emitter *metrics.MetricEventEmitter, logger *zap.Logger) *AsyncMetricsMiddleware {
	return &AsyncMetricsMiddleware{
		emitter: emitter,
		logger:  logger,
	}
}

// MetricsContext holds request context for metrics
type MetricsContext struct {
	RequestID     string
	StartTime     time.Time
	ModelName     string
	UserID        string
	TeamID        string
	KeyID         string
	ResolvedModel string // Actual model name after route resolution (e.g., "gpt-4o-41")
	ProviderModel string // Provider's model ID (e.g., "gpt-4o") — for pricing lookups
	ProviderType  string // Provider type (e.g., "openai", "anthropic")
	RouteSlug     string // Route slug if request came through a route; empty otherwise
}

// ContextKey is the type for context keys
type ContextKey string

const MetricsContextKey ContextKey = "metrics_context"

// Middleware wraps HTTP handlers to emit metric events
func (m *AsyncMetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-LLM endpoints
		if !isLLMEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		m.logger.Debug("Metrics middleware processing LLM endpoint", zap.String("path", r.URL.Path))

		// Create metrics context
		metricsCtx := &MetricsContext{
			RequestID: generateRequestID(),
			StartTime: time.Now(),
			// These will be populated by the auth middleware and LLM handler
		}

		// Add to request context
		ctx := context.WithValue(r.Context(), MetricsContextKey, metricsCtx)
		r = r.WithContext(ctx)

		// Create response writer wrapper to capture response data
		responseWriter := &metricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(responseWriter, r)

		// Emit completion event (if metrics context was populated)
		m.logger.Debug("Metrics middleware completion check",
			zap.String("model_name", metricsCtx.ModelName),
			zap.String("path", r.URL.Path))
		if metricsCtx.ModelName != "" {
			m.logger.Debug("Emitting metrics completion event", zap.String("model", metricsCtx.ModelName))
			m.emitCompletionEvent(metricsCtx, responseWriter)
		} else {
			m.logger.Debug("Skipping metrics emission - no model name")
		}
	})
}

// metricsResponseWriter wraps ResponseWriter to capture status codes
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *metricsResponseWriter) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *metricsResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(data)
}

// isLLMEndpoint checks if the request is for an LLM endpoint
func isLLMEndpoint(path string) bool {
	llmPaths := []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings",
		"/chat/completions",
		"/completions",
		"/embeddings",
	}

	for _, llmPath := range llmPaths {
		if path == llmPath {
			return true
		}
	}
	return false
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return uuid.New().String()
}

// emitCompletionEvent emits the request completion event
func (m *AsyncMetricsMiddleware) emitCompletionEvent(ctx *MetricsContext, w *metricsResponseWriter) {
	latency := time.Since(ctx.StartTime).Milliseconds()
	success := w.statusCode >= 200 && w.statusCode < 400

	errorType := ""
	if !success {
		switch {
		case w.statusCode >= 400 && w.statusCode < 500:
			errorType = "client_error"
		case w.statusCode >= 500:
			errorType = "server_error"
		default:
			errorType = "unknown_error"
		}
	}

	// Emit response event - this is fire-and-forget, won't block the request
	m.emitter.EmitResponse(
		ctx.RequestID,
		ctx.ModelName,
		0,   // tokens - will be populated by LLM handler if available
		0,   // prompt tokens
		0,   // output tokens
		0.0, // cost
		latency,
		success,
		false, // cache hit - will be populated if available
		errorType,
	)
}

// Helper functions for other parts of the application to use

// GetMetricsContext extracts metrics context from request context
func GetMetricsContext(ctx context.Context) *MetricsContext {
	if metricsCtx, ok := ctx.Value(MetricsContextKey).(*MetricsContext); ok {
		return metricsCtx
	}
	return nil
}

// SetModelName sets the model name in metrics context
func SetModelName(ctx context.Context, modelName string) {
	if metricsCtx := GetMetricsContext(ctx); metricsCtx != nil {
		metricsCtx.ModelName = modelName
	}
}

// SetUserInfo sets user information in metrics context
func SetUserInfo(ctx context.Context, userID, teamID, keyID string) {
	if metricsCtx := GetMetricsContext(ctx); metricsCtx != nil {
		metricsCtx.UserID = userID
		metricsCtx.TeamID = teamID
		metricsCtx.KeyID = keyID
	}
}

// EmitRequestEvent emits a request start event
func EmitRequestEvent(ctx context.Context, emitter *metrics.MetricEventEmitter) {
	if metricsCtx := GetMetricsContext(ctx); metricsCtx != nil {
		emitter.EmitRequest(
			metricsCtx.ModelName,
			metricsCtx.UserID,
			metricsCtx.TeamID,
			metricsCtx.KeyID,
			metricsCtx.RequestID,
		)
	}
}

// SetResolvedModel sets the resolved model information in metrics context
func SetResolvedModel(ctx context.Context, resolvedModel, providerModel, providerType, routeSlug string) {
	if metricsCtx := GetMetricsContext(ctx); metricsCtx != nil {
		metricsCtx.ResolvedModel = resolvedModel
		metricsCtx.ProviderModel = providerModel
		metricsCtx.ProviderType = providerType
		metricsCtx.RouteSlug = routeSlug
	}
}

// EmitDetailedResponse emits a detailed response event with token/cost information
func EmitDetailedResponse(ctx context.Context, emitter *metrics.MetricEventEmitter,
	tokens, promptTokens, outputTokens int64, cost float64, cacheHit bool) {
	if metricsCtx := GetMetricsContext(ctx); metricsCtx != nil {
		latency := time.Since(metricsCtx.StartTime).Milliseconds()

		emitter.EmitResponse(
			metricsCtx.RequestID,
			metricsCtx.ModelName,
			tokens,
			promptTokens,
			outputTokens,
			cost,
			latency,
			true, // success - we only call this on successful responses
			cacheHit,
			"",
		)
	}
}
