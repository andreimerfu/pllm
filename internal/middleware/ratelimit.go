package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/cache"
	"github.com/amerfu/pllm/internal/services/ratelimit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type RateLimitMiddleware struct {
	limiter      ratelimit.RateLimiter
	config       *config.RateLimitConfig
	log          *zap.Logger
	keyExtractor func(r *http.Request) string
}

func NewRateLimitMiddleware(cfg *config.Config, log *zap.Logger) *RateLimitMiddleware {
	var limiter ratelimit.RateLimiter

	// Use Redis limiter if available, otherwise in-memory
	if cache.IsHealthy() && cfg.RateLimit.Enabled {
		limiter = ratelimit.NewRedisLimiter(cache.GetClient(), log)
		log.Info("Using Redis-based rate limiter")
	} else if cfg.RateLimit.Enabled {
		limiter = ratelimit.NewInMemoryLimiter(log)
		log.Info("Using in-memory rate limiter")
	}

	return &RateLimitMiddleware{
		limiter: limiter,
		config:  &cfg.RateLimit,
		log:     log,
		keyExtractor: func(r *http.Request) string {
			// Default key extractor uses API key or IP
			apiKey := extractAPIKey(r)
			if apiKey != "" {
				return fmt.Sprintf("ratelimit:key:%s", apiKey)
			}

			// Fall back to IP address
			ip := getClientIP(r)
			return fmt.Sprintf("ratelimit:ip:%s", ip)
		},
	}
}

func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if rate limiting is disabled
		if m.limiter == nil || !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip rate limiting for static content and documentation
		if m.shouldSkipRateLimit(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract rate limit key
		key := m.keyExtractor(r)

		// Determine rate limits based on endpoint
		limit, window := m.getRateLimits(r)

		// Check rate limit
		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()

		allowed, err := m.limiter.Allow(ctx, key, limit, window)
		if err != nil {
			m.log.Error("Rate limit check failed", zap.Error(err))
			// On error, allow the request but log it
			next.ServeHTTP(w, r)
			return
		}

		// Get remaining requests
		remaining, _ := m.limiter.GetRemaining(ctx, key, limit, window)

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(window).Unix(), 10))

		if !allowed {
			// Rate limit hit - record metric
			RecordRateLimitHit(r.URL.Path)

			// Rate limit exceeded
			w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": {"message": "Rate limit exceeded. Please retry later.", "type": "rate_limit_error", "code": "rate_limit_exceeded"}}`))

			m.log.Warn("Rate limit exceeded",
				zap.String("key", key),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method))
			return
		}

		// Rate limit allowed - record metric
		RecordRateLimitAllowed(r.URL.Path)

		next.ServeHTTP(w, r)
	})
}

// shouldSkipRateLimit determines if rate limiting should be skipped for a given path
func (m *RateLimitMiddleware) shouldSkipRateLimit(path string) bool {
	// Skip rate limiting for documentation and UI routes
	if strings.HasPrefix(path, "/docs") ||
		strings.HasPrefix(path, "/ui") ||
		strings.HasPrefix(path, "/swagger") ||
		path == "/health" ||
		path == "/ready" ||
		path == "/metrics" {
		return true
	}
	
	// Skip for static assets that might be served under docs/ui
	if strings.Contains(path, "/assets/") ||
		strings.Contains(path, "/css/") ||
		strings.Contains(path, "/js/") ||
		strings.Contains(path, "/images/") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") {
		return true
	}
	
	return false
}

func (m *RateLimitMiddleware) getRateLimits(r *http.Request) (int, time.Duration) {
	// Get endpoint-specific limits
	routeCtx := chi.RouteContext(r.Context())
	path := ""
	if routeCtx != nil {
		path = routeCtx.RoutePattern()
	}

	// Check for model-specific rate limits
	if strings.HasPrefix(path, "/v1/chat/completions") || strings.Contains(r.URL.Path, "/chat/completions") {
		// Get model from request if available
		model := r.Header.Get("X-Model")
		if model != "" {
			// TODO: Get model-specific rate limits from config
			// For now, use default chat limits
		}

		if m.config.ChatCompletionsRPM > 0 {
			return m.config.ChatCompletionsRPM, time.Minute
		}
	}

	if strings.HasPrefix(path, "/v1/completions") || strings.Contains(r.URL.Path, "/completions") {
		if m.config.CompletionsRPM > 0 {
			return m.config.CompletionsRPM, time.Minute
		}
	}

	if strings.HasPrefix(path, "/v1/embeddings") || strings.Contains(r.URL.Path, "/embeddings") {
		if m.config.EmbeddingsRPM > 0 {
			return m.config.EmbeddingsRPM, time.Minute
		}
	}

	// Default rate limits
	if m.config.GlobalRPM > 0 {
		return m.config.GlobalRPM, time.Minute
	}

	// Fallback to reasonable defaults
	return 60, time.Minute
}

func extractAPIKey(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if auth != "" {
		parts := strings.Split(auth, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Check X-API-Key header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Check query parameter
	return r.URL.Query().Get("api_key")
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}

	return ip
}
