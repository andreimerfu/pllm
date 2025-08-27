package router

import (
	"log"
	"net/http"

	"github.com/amerfu/pllm/internal/config"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func NewMetricsRouter(cfg *config.Config, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	// Health check for metrics service
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status": "ok", "service": "metrics"}`)); err != nil {
			log.Printf("Failed to write metrics health response: %v", err)
		}
	})

	// Prometheus metrics endpoint
	if cfg.Monitoring.EnableMetrics {
		r.Handle("/metrics", promhttp.Handler())
	}

	return r
}
