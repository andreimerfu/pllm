package router

import (
	"net/http"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/handlers"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, logger *zap.Logger, modelManager *models.ModelManager) http.Handler {
	r := chi.NewRouter()
	
	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Logger(logger))
	
	// Metrics middleware
	r.Use(middleware.MetricsMiddleware(logger))
	
	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}))
	
	// Global rate limiting
	if cfg.RateLimit.Enabled {
		rateLimitMiddleware := middleware.NewRateLimitMiddleware(cfg, logger)
		r.Use(rateLimitMiddleware.Handler)
	}
	
	// Caching middleware
	if cfg.Cache.Enabled {
		cacheMiddleware := middleware.NewCacheMiddleware(cfg, logger)
		r.Use(cacheMiddleware.Handler)
	}
	
	// Health check
	r.Get("/health", handlers.Health)
	r.Get("/ready", handlers.Ready)
	
	// Prometheus metrics endpoint
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	
	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	
	// Initialize handlers
	llmHandler := handlers.NewLLMHandler(logger, modelManager)
	authHandler := handlers.NewAuthHandler(logger)
	
	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/v1/register", authHandler.Register)
		r.Post("/v1/login", authHandler.Login)
		r.Post("/v1/refresh", authHandler.RefreshToken)
	})
	
	// Protected routes
	r.Group(func(r chi.Router) {
		// Authentication middleware
		// r.Use(middleware.Authenticate(cfg.JWT.SecretKey))
		// r.Use(middleware.UsageTracking())
		
		// OpenAI-compatible endpoints
		r.Route("/v1", func(r chi.Router) {
			// Chat completions
			r.Post("/chat/completions", llmHandler.ChatCompletions)
			
			// Completions (legacy)
			r.Post("/completions", llmHandler.Completions)
			
			// Embeddings
			r.Post("/embeddings", llmHandler.Embeddings)
			
			// Models
			r.Get("/models", llmHandler.ListModels)
			r.Get("/models/{model}", llmHandler.GetModel)
			
			// Files (for fine-tuning, not implemented yet)
			r.Post("/files", llmHandler.UploadFile)
			r.Get("/files", llmHandler.ListFiles)
			r.Get("/files/{file_id}", llmHandler.GetFile)
			r.Delete("/files/{file_id}", llmHandler.DeleteFile)
			
			// Images
			r.Post("/images/generations", llmHandler.GenerateImage)
			r.Post("/images/edits", llmHandler.EditImage)
			r.Post("/images/variations", llmHandler.CreateImageVariation)
			
			// Audio
			r.Post("/audio/transcriptions", llmHandler.CreateTranscription)
			r.Post("/audio/translations", llmHandler.CreateTranslation)
			r.Post("/audio/speech", llmHandler.CreateSpeech)
			
			// Moderations
			r.Post("/moderations", llmHandler.CreateModeration)
		})
		
		// User management
		r.Route("/v1/user", func(r chi.Router) {
			r.Get("/profile", authHandler.GetProfile)
			r.Put("/profile", authHandler.UpdateProfile)
			r.Put("/password", authHandler.ChangePassword)
			
			// API Keys
			r.Get("/keys", authHandler.ListAPIKeys)
			r.Post("/keys", authHandler.CreateAPIKey)
			r.Delete("/keys/{key_id}", authHandler.DeleteAPIKey)
			
			// Usage
			r.Get("/usage", authHandler.GetUsage)
			r.Get("/usage/daily", authHandler.GetDailyUsage)
			r.Get("/usage/monthly", authHandler.GetMonthlyUsage)
			
			// Groups
			r.Get("/groups", authHandler.ListGroups)
			r.Post("/groups/join", authHandler.JoinGroup)
			r.Post("/groups/leave", authHandler.LeaveGroup)
		})
	})
	
	// API Key authentication routes
	r.Group(func(r chi.Router) {
		// r.Use(middleware.APIKeyAuth())
		// r.Use(middleware.UsageTracking())
		
		// OpenAI-compatible endpoints with API key
		r.Route("/api/v1", func(r chi.Router) {
			// Chat completions
			r.Post("/chat/completions", llmHandler.ChatCompletions)
			
			// Completions (legacy)
			r.Post("/completions", llmHandler.Completions)
			
			// Embeddings
			r.Post("/embeddings", llmHandler.Embeddings)
			
			// Models
			r.Get("/models", llmHandler.ListModels)
			r.Get("/models/{model}", llmHandler.GetModel)
			
			// Images
			r.Post("/images/generations", llmHandler.GenerateImage)
			
			// Audio
			r.Post("/audio/transcriptions", llmHandler.CreateTranscription)
			r.Post("/audio/translations", llmHandler.CreateTranslation)
			r.Post("/audio/speech", llmHandler.CreateSpeech)
			
			// Moderations
			r.Post("/moderations", llmHandler.CreateModeration)
		})
	})
	
	// Not found handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"message": "Not found", "type": "invalid_request_error", "code": "not_found"}}`))
	})
	
	return r
}