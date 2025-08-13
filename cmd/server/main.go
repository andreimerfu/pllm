package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/database"
	"github.com/amerfu/pllm/internal/logger"
	"github.com/amerfu/pllm/internal/router"
	"github.com/amerfu/pllm/internal/services/cache"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gorm.io/gorm"

	_ "github.com/amerfu/pllm/internal/handlers/swagger"
)

// @title pllm - Blazing Fast LLM Gateway
// @version 1.0
// @description A high-performance LLM Gateway with OpenAI-compatible API, supporting multiple providers with load balancing, rate limiting, caching, and comprehensive monitoring.

// @contact.name API Support
// @contact.email support@pllm.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

type AppMode struct {
	DatabaseAvailable bool
	RedisAvailable    bool
	IsLiteMode        bool
}

func main() {
	// Load .env file if exists
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.Initialize(cfg.Logging)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Detect available dependencies
	appMode := detectDependencies(cfg, log)

	if appMode.IsLiteMode {
		log.Warn("Running in LITE MODE - Limited functionality",
			zap.Bool("database", appMode.DatabaseAvailable),
			zap.Bool("redis", appMode.RedisAvailable))
		log.Info("Available features in LITE MODE:",
			zap.Strings("features", []string{
				"LLM Proxy",
				"Load Balancing",
				"Multiple Model Instances",
				"Routing Strategies",
				"Basic Health Checks",
			}))
		log.Warn("Disabled features in LITE MODE:",
			zap.Strings("disabled", []string{
				"User Management",
				"Authentication",
				"Usage Tracking",
				"Budgeting",
				"Persistent Cache",
				"Admin API",
			}))
	} else {
		log.Info("Running in FULL MODE - All features enabled")
	}

	// Initialize database if available
	if appMode.DatabaseAvailable {
		dbConfig := &database.Config{
			DSN:             cfg.Database.URL,
			MaxConnections:  cfg.Database.MaxConnections,
			MaxIdleConns:    cfg.Database.MaxIdleConns,
			ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		}

		if err := database.Initialize(dbConfig); err != nil {
			log.Warn("Failed to initialize database, switching to LITE MODE", zap.Error(err))
			appMode.DatabaseAvailable = false
			appMode.IsLiteMode = true
		} else {
			defer database.Close()
		}
	}

	// Initialize cache if Redis is available
	if appMode.RedisAvailable && cfg.Cache.Enabled {
		cacheConfig := &cache.Config{
			RedisURL: cfg.Redis.URL,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			TTL:      cfg.Cache.TTL,
			MaxSize:  cfg.Cache.MaxSize,
		}

		if err := cache.Initialize(cacheConfig); err != nil {
			log.Warn("Failed to initialize cache, continuing without caching", zap.Error(err))
			appMode.RedisAvailable = false
		} else {
			defer cache.Close()
		}
	}

	// Initialize services (only in full mode)
	// TODO: Implement authentication and services when config is updated
	/*
		var authService *auth.Service
		var budgetService *budget.Service
		var teamService *team.Service
		var virtualKeyService *virtualkey.Service

		if !appMode.IsLiteMode {
			// Initialize auth service with Dex if configured
			authConfig := &auth.Config{
				JWTSecret:     cfg.Auth.JWTSecret,
				JWTIssuer:     cfg.Auth.JWTIssuer,
				JWTExpiration: time.Duration(cfg.Auth.JWTExpirationHours) * time.Hour,
			}

			if cfg.Auth.Dex.Enabled {
				authConfig.DexConfig = &auth.DexConfig{
					Issuer:       cfg.Auth.Dex.Issuer,
					ClientID:     cfg.Auth.Dex.ClientID,
					ClientSecret: cfg.Auth.Dex.ClientSecret,
					RedirectURL:  cfg.Auth.Dex.RedirectURL,
				}
			}

			authService, err = auth.NewService(authConfig, database.GetDB())
			if err != nil {
				log.Fatal("Failed to initialize auth service", zap.Error(err))
			}

			// Initialize other services
			budgetService = budget.NewService(database.GetDB())
			teamService = team.NewService(database.GetDB())
			virtualKeyService = virtualkey.NewService(database.GetDB())
		}
	*/

	// Initialize model manager (always needed)
	modelManager := models.NewModelManager(log, cfg.Router)
	if err := modelManager.LoadModelInstances(cfg.ModelList); err != nil {
		log.Fatal("Failed to load model instances", zap.Error(err))
	}

	// Create routers based on mode
	var servers []*http.Server

	// Main API router is always created
	// Pass database if available (nil in lite mode)
	var db *gorm.DB
	if !appMode.IsLiteMode && appMode.DatabaseAvailable {
		db = database.GetDB()
	}
	mainRouter := router.NewRouter(cfg, log, modelManager, db)

	// Add authentication middleware in full mode
	// TODO: Enable when auth services are implemented
	/*
		if !appMode.IsLiteMode && authService != nil {
			authMiddleware := middleware.NewAuthMiddleware(
				authService,
				virtualKeyService,
				cfg.Auth.MasterKey,
			)
			mainRouter.Use(authMiddleware.Authenticate)
		}
	*/

	servers = append(servers, &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mainRouter,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	})

	// Only create metrics router in full mode (admin is now part of main router)
	if !appMode.IsLiteMode {
		// Admin routes are now mounted on the main router at /api/admin
		// This simplifies deployment and frontend access
		
		// Create metrics router
		metricsRouter := router.NewMetricsRouter(cfg, log)
		servers = append(servers, &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.MetricsPort),
			Handler:      metricsRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		})
	}

	// Start servers in goroutines
	for i, srv := range servers {
		go func(s *http.Server, idx int) {
			var serverType string
			if appMode.IsLiteMode {
				serverType = "Main API (LITE MODE)"
			} else {
				switch idx {
				case 0:
					serverType = "Main API (with Admin)"
				case 1:
					serverType = "Metrics"
				}
			}

			log.Info(fmt.Sprintf("%s server starting", serverType),
				zap.String("address", s.Addr))

			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal(fmt.Sprintf("%s server failed to start", serverType),
					zap.Error(err))
			}
		}(srv, i)
	}

	if appMode.IsLiteMode {
		log.Info("pllm Gateway started successfully in LITE MODE",
			zap.Int("api_port", cfg.Server.Port),
			zap.String("mode", "LITE"),
			zap.Bool("database", appMode.DatabaseAvailable),
			zap.Bool("redis", appMode.RedisAvailable))
	} else {
		log.Info("pllm Gateway started successfully in FULL MODE",
			zap.Int("api_port", cfg.Server.Port),
			zap.Int("admin_port", cfg.Server.AdminPort),
			zap.Int("metrics_port", cfg.Server.MetricsPort))
	}

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down servers...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulShutdown)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server forced to shutdown", zap.Error(err))
		}
	}

	log.Info("Servers shutdown complete")
}

// detectDependencies checks if PostgreSQL and Redis are available
func detectDependencies(cfg *config.Config, log *zap.Logger) AppMode {
	mode := AppMode{
		DatabaseAvailable: false,
		RedisAvailable:    false,
		IsLiteMode:        false,
	}

	// Check if database is configured and reachable
	if cfg.Database.URL != "" {
		log.Debug("Checking database connectivity...", zap.String("url", maskConnectionString(cfg.Database.URL)))
		dbConfig := &database.Config{
			DSN:             cfg.Database.URL,
			MaxConnections:  1,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Second * 5,
		}

		// Try to connect with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := database.TestConnection(ctx, dbConfig); err == nil {
			mode.DatabaseAvailable = true
			log.Debug("Database is available")
		} else {
			log.Debug("Database is not available", zap.Error(err))
		}
	}

	// Check if Redis is configured and reachable
	if cfg.Redis.URL != "" && cfg.Cache.Enabled {
		log.Debug("Checking Redis connectivity...", zap.String("url", maskConnectionString(cfg.Redis.URL)))
		cacheConfig := &cache.Config{
			RedisURL: cfg.Redis.URL,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		}

		// Try to connect with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := cache.TestConnection(ctx, cacheConfig); err == nil {
			mode.RedisAvailable = true
			log.Debug("Redis is available")
		} else {
			log.Debug("Redis is not available", zap.Error(err))
		}
	}

	// Determine if we're in lite mode
	mode.IsLiteMode = !mode.DatabaseAvailable || !mode.RedisAvailable

	// Allow override via environment variable
	if os.Getenv("PLLM_LITE_MODE") == "true" {
		mode.IsLiteMode = true
		log.Info("LITE MODE forced via environment variable")
	}

	return mode
}

// maskConnectionString masks sensitive parts of connection strings
func maskConnectionString(conn string) string {
	// Simple masking - in production, use a proper parser
	if len(conn) > 20 {
		return conn[:10] + "****" + conn[len(conn)-10:]
	}
	return "****"
}
