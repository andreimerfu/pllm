package router

import (
	"context"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/api/docs"
	"github.com/amerfu/pllm/internal/api/handlers"
	"github.com/amerfu/pllm/internal/api/handlers/admin"
	deploymenthandlers "github.com/amerfu/pllm/internal/api/handlers/deployment"
	mcphandlers "github.com/amerfu/pllm/internal/api/handlers/mcp"
	registryhandlers "github.com/amerfu/pllm/internal/api/handlers/registry"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/services/data/budget"
	"github.com/amerfu/pllm/internal/services/data/cache"
	"github.com/amerfu/pllm/internal/services/integrations/guardrails"
	"github.com/amerfu/pllm/internal/services/integrations/key"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/realtime"
	deploymentsvc "github.com/amerfu/pllm/internal/services/deployment"
	mcpgateway "github.com/amerfu/pllm/internal/services/mcp/gateway"
	mcpregistryserver "github.com/amerfu/pllm/internal/services/mcp/registryserver"
	"github.com/amerfu/pllm/internal/services/registry/enrichment"
	"github.com/amerfu/pllm/internal/services/registry/importer"
	registryservice "github.com/amerfu/pllm/internal/services/registry/service"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/integrations/team"
	"github.com/amerfu/pllm/internal/api/ui"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewRouter(cfg *config.Config, logger *zap.Logger, modelManager *models.ModelManager, db *gorm.DB, pricingManager *config.ModelPricingManager) http.Handler {
	r := chi.NewRouter()

	// Initialize Redis client
	opt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.Fatal("Failed to parse Redis URL", zap.Error(err))
	}

	// Override with explicit password and DB if provided
	if cfg.Redis.Password != "" {
		opt.Password = cfg.Redis.Password
	}
	if cfg.Redis.DB != 0 {
		opt.DB = cfg.Redis.DB
	}

	redisClient := redis.NewClient(opt)

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
			PublicIssuer: cfg.Auth.Dex.PublicIssuer,
			ClientID:     cfg.Auth.Dex.ClientID,
			ClientSecret: cfg.Auth.Dex.ClientSecret,
			RedirectURL:  cfg.Auth.Dex.RedirectURL,
			Scopes:       cfg.Auth.Dex.Scopes,
		}
	}

	// Initialize team and key services for auth auto-provisioning
	teamService := team.NewTeamService(db)
	keyService := key.NewService(db, logger)

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		DexConfig:        dexConfig,
		JWTSecret:        cfg.JWT.SecretKey,
		JWTIssuer:        "pllm",
		TokenExpiry:      cfg.JWT.AccessTokenDuration,
		MasterKeyService: masterKeyService,
		TeamService:      teamService,
		KeyService:       keyService,
	})
	if err != nil {
		logger.Fatal("Failed to initialize auth service", zap.Error(err))
	}

	// Initialize metrics service if database and Redis are available
	var metricsService *metrics.MetricsService
	var metricsEmitter *metrics.MetricEventEmitter
	if db != nil {
		metricsConfig := &metrics.MetricsServiceConfig{
			DB:                db,
			Redis:             redisClient,
			Logger:            logger,
			WorkerCount:       4,
			BatchSize:         100,
			BatchTimeout:      5 * time.Second,
			AggregateInterval: 1 * time.Minute,
			EnableMonitoring:  false,
			MonitoringPort:    8082,
		}

		metricsService, err = metrics.NewMetricsService(metricsConfig)
		if err != nil {
			logger.Warn("Failed to initialize metrics service", zap.Error(err))
		} else {
			// Start metrics service
			if err := metricsService.Start(context.Background()); err != nil {
				logger.Warn("Failed to start metrics service", zap.Error(err))
				metricsService = nil
			} else {
				metricsEmitter = metricsService.GetEmitter()
				logger.Info("Metrics service started successfully")
			}
		}
	}

	// Initialize historical metrics collector to aggregate existing usage data
	logger.Info("Checking metrics collector prerequisites",
		zap.Bool("db_exists", db != nil),
		zap.Bool("model_manager_exists", modelManager != nil))
	if db != nil && modelManager != nil {
		historicalCollector := metrics.NewMetricsCollector(db, logger, modelManager)
		historicalCollector.Start()
		logger.Info("Historical metrics collector started")
	} else {
		logger.Warn("Cannot start historical metrics collector - missing prerequisites")
	}

	// Initialize Registry services (require DB — skipped in lite mode)
	var registryHandler *registryhandlers.Handler
	var registryImportHandler *registryhandlers.ImportHandler
	var registryMCPHandler *mcphandlers.RegistryMCPHandler
	if db != nil {
		serverSvc := registryservice.NewServerService(db, logger)
		agentSvc := registryservice.NewAgentService(db, logger)
		skillSvc := registryservice.NewSkillService(db, logger)
		promptSvc := registryservice.NewPromptService(db, logger)
		registryHandler = registryhandlers.NewHandler(
			logger, serverSvc, agentSvc, skillSvc, promptSvc,
		)
		// Enrichment runner: OSV scanner only in phase 2. Started in a
		// goroutine; shuts down when context is canceled (process exit).
		runner := enrichment.NewRunner(db, logger)
		registryHandler.Enrichment = runner
		go runner.Start(context.Background())

		// Importer (npm + PyPI). Safe to init without network — sources are
		// only hit when /api/admin/registry/import is called.
		registryImportHandler = registryhandlers.NewImportHandler(
			logger, importer.NewService(serverSvc, logger))

		// Expose the registry over MCP so agents can discover artifacts
		// via tool calls — same surface, different protocol.
		registryMCPHandler = mcphandlers.NewRegistryMCPHandler(
			logger, mcpregistryserver.New(serverSvc, agentSvc, skillSvc, promptSvc))

		logger.Info("Registry services + enrichment + importer + MCP surface initialized")
	}

	// Initialize MCP Gateway manager (safe even without DB — runs with empty backend set)
	var mcpManager *mcpgateway.Manager
	if cfg.MCP.Enabled {
		mcpManager = mcpgateway.NewManager(logger, db)
		if db != nil {
			if err := mcpManager.Load(context.Background()); err != nil {
				logger.Warn("Failed to load MCP servers from DB", zap.Error(err))
			}
		}
		mcpManager.Start(context.Background())
		logger.Info("MCP Gateway initialized", zap.Bool("persistent", db != nil))
	}

	// Deployment service — opt-in via config.deployment.enabled. Falls
	// back gracefully if credentials can't be loaded. Depends on mcpManager
	// so it must be initialized after MCP setup.
	var deployHandler *deploymenthandlers.Handler
	if db != nil && cfg.Deployment.Enabled {
		adapter, err := deploymentsvc.NewK8sAdapter(cfg.Deployment.K8s, logger)
		if err != nil {
			logger.Warn("Deployment disabled: K8s client unavailable", zap.Error(err))
		} else {
			bridge := deploymentsvc.NewMCPBridge(db, mcpManager, logger)
			deploySvc := deploymentsvc.NewService(db, logger, adapter, bridge, cfg.Deployment.K8s.Namespace)
			serverSvc := registryservice.NewServerService(db, logger)
			deployHandler = deploymenthandlers.NewHandler(logger, deploySvc, serverSvc)
			logger.Info("Deployment service initialized",
				zap.String("namespace", cfg.Deployment.K8s.Namespace),
				zap.Bool("in_cluster", cfg.Deployment.K8s.InCluster))
		}
	}

	// Initialize guardrails
	var guardrailsExecutor *guardrails.Executor
	guardrailsFactory := guardrails.NewFactory(cfg, logger)
	
	// Validate guardrails configuration
	if err := guardrailsFactory.ValidateConfiguration(); err != nil {
		logger.Error("Invalid guardrails configuration", zap.Error(err))
	} else {
		// Create guardrails executor
		var err error
		guardrailsExecutor, err = guardrailsFactory.CreateExecutor()
		if err != nil {
			logger.Error("Failed to create guardrails executor", zap.Error(err))
		} else {
			logger.Info("Guardrails executor initialized successfully",
				zap.Bool("enabled", cfg.Guardrails.Enabled),
				zap.Int("guardrails_count", len(cfg.Guardrails.Guardrails)))
		}
	}

	// Legacy synchronous budget/usage systems removed in favor of async Redis-based system

	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Logger(logger))

	// Metrics middleware - use advanced metrics if available, otherwise basic
	if metricsEmitter != nil {
		r.Use(middleware.NewAsyncMetricsMiddleware(metricsEmitter, logger).Middleware)
	} else {
		r.Use(middleware.MetricsMiddleware(logger))
	}

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

	// Initialize specialized handlers
	var (
		chatHandler          *handlers.ChatHandler
		messagesHandler      *handlers.MessagesHandler
		embeddingsHandler    *handlers.EmbeddingsHandler
		modelsHandler        *handlers.ModelsHandler
		filesHandler         *handlers.FilesHandler
		imagesHandler        *handlers.ImagesHandler
		audioHandler         *handlers.AudioHandler
		moderationHandler    *handlers.ModerationHandler
		adminHandler         *handlers.AdminHandler
		modelMgmtHandler     *handlers.ModelManagementHandler
		realtimeHandler      *handlers.RealtimeHandler
	)

	if metricsEmitter != nil {
		chatHandler = handlers.NewChatHandlerWithMetrics(logger, modelManager, metricsEmitter)
		messagesHandler = handlers.NewMessagesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		embeddingsHandler = handlers.NewEmbeddingsHandlerWithMetrics(logger, modelManager, metricsEmitter)
		modelsHandler = handlers.NewModelsHandlerWithMetrics(logger, modelManager, pricingManager, metricsEmitter)
		filesHandler = handlers.NewFilesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		imagesHandler = handlers.NewImagesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		audioHandler = handlers.NewAudioHandlerWithMetrics(logger, modelManager, metricsEmitter)
		moderationHandler = handlers.NewModerationHandlerWithMetrics(logger, modelManager, metricsEmitter)
		adminHandler = handlers.NewAdminHandlerWithMetrics(logger, modelManager, metricsEmitter)
	} else {
		chatHandler = handlers.NewChatHandler(logger, modelManager)
		messagesHandler = handlers.NewMessagesHandler(logger, modelManager)
		embeddingsHandler = handlers.NewEmbeddingsHandler(logger, modelManager)
		modelsHandler = handlers.NewModelsHandler(logger, modelManager, pricingManager)
		filesHandler = handlers.NewFilesHandler(logger, modelManager)
		imagesHandler = handlers.NewImagesHandler(logger, modelManager)
		audioHandler = handlers.NewAudioHandler(logger, modelManager)
		moderationHandler = handlers.NewModerationHandler(logger, modelManager)
		adminHandler = handlers.NewAdminHandler(logger, modelManager)
	}
	modelMgmtHandler = handlers.NewModelManagementHandler(pricingManager)

	// Initialize realtime session manager and handler
	sessionConfig := &realtime.SessionConfig{
		MaxSessions:     100,
		SessionTimeout:  30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
	sessionManager := realtime.NewSessionManager(logger, db, sessionConfig)
	
	handlerConfig := &handlers.RealtimeHandlerConfig{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		HandshakeTimeout:  45 * time.Second,
		CheckOrigin:       false,
		EnableCompression: true,
	}
	realtimeHandler = handlers.NewRealtimeHandler(logger, sessionManager, modelManager, handlerConfig)
	authHandler := handlers.NewAuthHandler(logger, authService, masterKeyService, db)

	// Initialize system handler for auth config
	systemHandler := admin.NewSystemHandler(logger, db)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/v1/register", authHandler.Register)
		r.Post("/v1/login", authHandler.Login)
		r.Post("/v1/refresh", authHandler.RefreshToken)
		r.Get("/api/auth/config", systemHandler.GetAuthConfig) // Public auth config
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

		// Guardrails middleware (after auth, before budget)
		if guardrailsExecutor != nil {
			guardrailsMiddleware := middleware.NewGuardrailsMiddleware(guardrailsExecutor, logger)
			r.Use(guardrailsMiddleware.Middleware)
		}

		// Initialize pricing cache for better performance
		pricingCache := cache.NewPricingCache(redisClient, logger, pricingManager)
		
		// Load all pricing data to Redis cache on startup
		if err := pricingCache.LoadAllPricingToCache(context.Background()); err != nil {
			logger.Warn("Failed to load pricing data to cache", zap.Error(err))
		}

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:         logger,
			AuthService:    authService,
			BudgetCache:    redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:       redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:     redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			PricingManager: pricingManager,
			PricingCache:   pricingCache,
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

		// OpenAI-compatible endpoints
		r.Route("/v1", func(r chi.Router) {
			// Chat completions - use a custom handler that preserves Flusher
			r.HandleFunc("/chat/completions", chatHandler.ChatCompletions)

			// Anthropic Messages API format (LiteLLM compatible)
			r.HandleFunc("/messages", messagesHandler.AnthropicMessages)

			// Completions (legacy)
			r.Post("/completions", chatHandler.Completions)

			// Embeddings
			r.Post("/embeddings", embeddingsHandler.Embeddings)

			// Models
			r.Get("/models", modelsHandler.ListModels)
			r.Get("/models/{model}", modelsHandler.GetModel)
			
			// Model Management (LiteLLM-compatible)
			r.Get("/model/info", modelMgmtHandler.GetModelInfo)
			r.Post("/model/register", modelMgmtHandler.RegisterModel)
			r.Post("/model/calculate-cost", modelMgmtHandler.CalculateCost)
			r.Get("/model/{model_name}/cost", modelMgmtHandler.GetModelCost)
			r.Patch("/model/{model_name}/pricing", modelMgmtHandler.UpdateModelPricing)

			// Files (for fine-tuning, not implemented yet)
			r.Post("/files", filesHandler.UploadFile)
			r.Get("/files", filesHandler.ListFiles)
			r.Get("/files/{file_id}", filesHandler.GetFile)
			r.Delete("/files/{file_id}", filesHandler.DeleteFile)

			// Images
			r.Post("/images/generations", imagesHandler.GenerateImage)
			r.Post("/images/edits", imagesHandler.EditImage)
			r.Post("/images/variations", imagesHandler.CreateImageVariation)

			// Audio
			r.Post("/audio/transcriptions", audioHandler.CreateTranscription)
			r.Post("/audio/translations", audioHandler.CreateTranslation)
			r.Post("/audio/speech", audioHandler.CreateSpeech)

			// Moderations
			r.Post("/moderations", moderationHandler.CreateModeration)

			// Realtime API (WebSocket)
			r.Get("/realtime", realtimeHandler.ConnectRealtime)
			r.Route("/realtime/sessions", func(r chi.Router) {
				r.Post("/", realtimeHandler.CreateSession)
				r.Get("/", realtimeHandler.ListSessions)
				r.Get("/{id}", realtimeHandler.GetSession)
			})

			// MCP Gateway: streamable-HTTP endpoint for Claude Desktop / Cursor / VS Code.
			if mcpManager != nil {
				gatewayHandler := mcphandlers.NewGatewayHandler(logger, mcpManager)
				r.Handle("/mcp", gatewayHandler)
			}

			// Registry exposed as an MCP server — agents can call list_servers,
			// get_agent, etc. as tool calls instead of the REST API.
			if registryMCPHandler != nil {
				r.Handle("/mcp/registry", registryMCPHandler)
			}

			// Registry: read-only catalog endpoints.
			if registryHandler != nil {
				r.Route("/registry", func(rr chi.Router) {
					rr.Get("/servers", registryHandler.ListServers)
					rr.Get("/servers/{name}", registryHandler.GetServer)
					rr.Get("/servers/{name}/versions", registryHandler.ListServerVersions)
					rr.Get("/servers/{name}/versions/{version}", registryHandler.GetServerVersion)
					rr.Get("/agents", registryHandler.ListAgents)
					rr.Get("/agents/{name}", registryHandler.GetAgent)
					rr.Get("/agents/{name}/versions", registryHandler.ListAgentVersions)
					rr.Get("/agents/{name}/versions/{version}", registryHandler.GetAgentVersion)
					rr.Get("/skills", registryHandler.ListSkills)
					rr.Get("/skills/{name}", registryHandler.GetSkill)
					rr.Get("/skills/{name}/versions", registryHandler.ListSkillVersions)
					rr.Get("/skills/{name}/versions/{version}", registryHandler.GetSkillVersion)
					rr.Get("/prompts", registryHandler.ListPrompts)
					rr.Get("/prompts/{name}", registryHandler.GetPrompt)
					rr.Get("/prompts/{name}/versions", registryHandler.ListPromptVersions)
					rr.Get("/prompts/{name}/versions/{version}", registryHandler.GetPromptVersion)
				})
			}
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
			r.Get("/budget", authHandler.GetBudgetStatus)
			r.Get("/teams", authHandler.GetUserTeams)

		})

		// Admin routes for monitoring
		r.Route("/v1/admin", func(r chi.Router) {
			// Model performance statistics
			r.Get("/models/stats", adminHandler.ModelStats)
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

		// Guardrails middleware (after auth, before budget)
		if guardrailsExecutor != nil {
			guardrailsMiddleware := middleware.NewGuardrailsMiddleware(guardrailsExecutor, logger)
			r.Use(guardrailsMiddleware.Middleware)
		}

		// Initialize pricing cache for better performance
		pricingCache := cache.NewPricingCache(redisClient, logger, pricingManager)
		
		// Load all pricing data to Redis cache on startup
		if err := pricingCache.LoadAllPricingToCache(context.Background()); err != nil {
			logger.Warn("Failed to load pricing data to cache", zap.Error(err))
		}

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:         logger,
			AuthService:    authService,
			BudgetCache:    redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:       redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:     redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			PricingManager: pricingManager,
			PricingCache:   pricingCache,
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

		// OpenAI-compatible endpoints with API key
		r.Route("/api/v1", func(r chi.Router) {
			// Chat completions
			r.Post("/chat/completions", chatHandler.ChatCompletions)

			// Anthropic Messages API format (LiteLLM compatible)
			r.Post("/messages", messagesHandler.AnthropicMessages)

			// Completions (legacy)
			r.Post("/completions", chatHandler.Completions)

			// Embeddings
			r.Post("/embeddings", embeddingsHandler.Embeddings)

			// Models
			r.Get("/models", modelsHandler.ListModels)
			r.Get("/models/{model}", modelsHandler.GetModel)

			// Images
			r.Post("/images/generations", imagesHandler.GenerateImage)

			// Audio
			r.Post("/audio/transcriptions", audioHandler.CreateTranscription)
			r.Post("/audio/translations", audioHandler.CreateTranslation)
			r.Post("/audio/speech", audioHandler.CreateSpeech)

			// Moderations
			r.Post("/moderations", moderationHandler.CreateModeration)

			// Realtime API (WebSocket) - authenticated
			r.Get("/realtime", realtimeHandler.ConnectRealtime)
			r.Route("/realtime/sessions", func(r chi.Router) {
				r.Post("/", realtimeHandler.CreateSession)
				r.Get("/", realtimeHandler.ListSessions)
				r.Get("/{id}", realtimeHandler.GetSession)
			})
		})
	})

	// Admin routes - mount if database is available
	if db != nil {
		// Create unified budget service using the existing Redis components
		budgetService := budget.NewUnifiedService(&budget.UnifiedServiceConfig{
			DB:          db,
			Logger:      logger,
			BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			UsageQueue: redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			EventPub: redisService.NewEventPublisher(redisClient, logger),
		})

		// Create admin sub-router configuration
		adminConfig := &AdminRouterConfig{
			Config:              cfg,
			Logger:              logger,
			DB:                  db,
			AuthService:         authService,
			MasterKeyService:    masterKeyService,
			ModelManager:        modelManager,
			BudgetService:       budgetService,
			GuardrailsExecutor:    guardrailsExecutor,
			MCPManager:            mcpManager,
			RegistryHandler:       registryHandler,
			RegistryImportHandler: registryImportHandler,
			DeploymentHandler:     deployHandler,
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

	// Static file serving for uploads
	r.Get("/files/{fileID}", filesHandler.GetFile)

	// Not found handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error": {"message": "Not found", "type": "invalid_request_error", "code": "not_found"}}`)); err != nil {
			logger.Error("Failed to write 404 response", zap.Error(err))
		}
	})

	return r
}
