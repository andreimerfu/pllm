package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for health and metrics endpoints to reduce noise
			if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			
			start := time.Now()
			
			// Use streaming-aware wrapper that preserves Flusher interface
			ww := NewStreamingResponseWriter(w)
			
			defer func() {
				logger.Info("request",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.StatusCode()),
					zap.Duration("duration", time.Since(start)),
					zap.String("remote", r.RemoteAddr),
					zap.String("request_id", middleware.GetReqID(r.Context())),
				)
			}()
			
			next.ServeHTTP(ww, r)
		})
	}
}