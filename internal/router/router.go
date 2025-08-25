package router

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/docs"
	"github.com/amerfu/pllm/internal/handlers"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/models"
	redisService "github.com/amerfu/pllm/internal/services/redis"
	"github.com/amerfu/pllm/internal/ui"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewRouter(cfg *config.Config, logger *zap.Logger, modelManager *models.ModelManager, db *gorm.DB) http.Handler {
	r := chi.NewRouter()

	// Initialize Redis client
	redisAddr := cfg.Redis.URL
	// Handle redis:// protocol prefix
	if strings.HasPrefix(redisAddr, "redis://") {
		redisAddr = strings.TrimPrefix(redisAddr, "redis://")
	}
	
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize auth services
	masterKeyService := auth.NewMasterKeyService(&auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   cfg.Auth.MasterKey,
		JWTSecret:   []byte(cfg.JWT.SecretKey),
		JWTIssuer:   "pllm",
		TokenExpiry: 24 * time.Hour,
	})

	// Prepare Dex config if enabled
	var dexConfig *auth.DexConfig
	if cfg.Auth.Dex.Enabled {
		dexConfig = &auth.DexConfig{
			Issuer:       cfg.Auth.Dex.Issuer,
			ClientID:     cfg.Auth.Dex.ClientID,
			ClientSecret: cfg.Auth.Dex.ClientSecret,
			RedirectURL:  cfg.Auth.Dex.RedirectURL,
			Scopes:       cfg.Auth.Dex.Scopes,
		}
	}

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		DexConfig:        dexConfig,
		JWTSecret:        cfg.JWT.SecretKey,
		JWTIssuer:        "pllm",
		TokenExpiry:      cfg.JWT.AccessTokenDuration,
		MasterKeyService: masterKeyService,
	})
	if err != nil {
		logger.Fatal("Failed to initialize auth service", zap.Error(err))
	}

	// Legacy synchronous budget/usage systems removed in favor of async Redis-based system

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
	authHandler := handlers.NewAuthHandler(logger, authService, masterKeyService, db)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/v1/register", authHandler.Register)
		r.Post("/v1/login", authHandler.Login)
		r.Post("/v1/refresh", authHandler.RefreshToken)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Authentication middleware
		authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
			Logger:           logger,
			AuthService:      authService,
			MasterKeyService: masterKeyService,
			RequireAuth:      true,
		})
		r.Use(authMiddleware.Authenticate)

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:      logger,
			AuthService: authService,
			BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:    redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:  redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

		// OpenAI-compatible endpoints
		r.Route("/v1", func(r chi.Router) {
			// Chat completions - use a custom handler that preserves Flusher
			r.HandleFunc("/chat/completions", llmHandler.ChatCompletions)

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

		})

		// Admin routes for monitoring
		r.Route("/v1/admin", func(r chi.Router) {
			// Model performance statistics
			r.Get("/models/stats", llmHandler.ModelStats)
		})
	})

	// API Key authentication routes
	r.Group(func(r chi.Router) {
		// Authentication middleware
		authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
			Logger:           logger,
			AuthService:      authService,
			MasterKeyService: masterKeyService,
			RequireAuth:      true,
		})
		r.Use(authMiddleware.Authenticate)

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:      logger,
			AuthService: authService,
			BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:    redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:  redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

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

	// Admin routes - mount if database is available
	if db != nil {
		// Create admin sub-router configuration
		adminConfig := &AdminRouterConfig{
			Config:           cfg,
			Logger:           logger,
			DB:               db,
			AuthService:      authService,
			MasterKeyService: masterKeyService,
			ModelManager:     modelManager,
		}

		// Mount admin routes at /api/admin
		adminSubRouter := NewAdminSubRouter(adminConfig)
		r.Mount("/api/admin", adminSubRouter)

		logger.Info("Admin routes mounted at /api/admin")
	}

	// Documentation routes
	docsHandler, err := docs.NewHandler(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize docs handler", zap.Error(err))
	} else if docsHandler.IsEnabled() {
		// Mount docs at /docs with a wildcard to catch all subpaths
		r.Mount("/docs", http.StripPrefix("/docs", docsHandler))

		// Also handle /docs without trailing slash
		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
		})

		logger.Info("Documentation routes enabled")
	}

	// UI routes (if enabled)
	uiHandler, err := ui.NewHandler(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize UI handler", zap.Error(err))
	} else if uiHandler.IsEnabled() {
		// Mount UI at /ui with a wildcard to catch all subpaths
		r.Mount("/ui", http.StripPrefix("/ui", uiHandler))

		// Also handle /ui without trailing slash
		r.Get("/ui", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
		})

		// Redirect root to UI
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})

		logger.Info("UI routes enabled")
	}

	// Not found handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"message": "Not found", "type": "invalid_request_error", "code": "not_found"}}`))
	})

	return r
}
