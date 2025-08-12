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
			RedisURL:  cfg.Redis.URL,
			Password:  cfg.Redis.Password,
			DB:        cfg.Redis.DB,
			TTL:       cfg.Cache.TTL,
			MaxSize:   cfg.Cache.MaxSize,
		}
		
		if err := cache.Initialize(cacheConfig); err != nil {
			log.Warn("Failed to initialize cache, continuing without caching", zap.Error(err))
			appMode.RedisAvailable = false
		} else {
			defer cache.Close()
		}
	}
	
	// Initialize model manager (always needed)
	modelManager := models.NewModelManager(log, cfg.Router)
	if err := modelManager.LoadModelInstances(cfg.ModelList); err != nil {
		log.Fatal("Failed to load model instances", zap.Error(err))
	}
	
	// Create routers based on mode
	var servers []*http.Server
	
	// Main API router is always created
	mainRouter := router.NewRouter(cfg, log, modelManager)
	servers = append(servers, &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mainRouter,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	})
	
	// Only create admin and metrics routers in full mode
	if !appMode.IsLiteMode {
		// Create admin router
		adminRouter := router.NewAdminRouter(cfg, log)
		servers = append(servers, &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.AdminPort),
			Handler:      adminRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		})
		
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
					serverType = "Main API"
				case 1:
					serverType = "Admin API"
				case 2:
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