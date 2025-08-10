package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/database"
	"github.com/amerfu/pllm/internal/logger"
	"github.com/amerfu/pllm/internal/router"
	"github.com/amerfu/pllm/internal/services/cache"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	
	_ "github.com/amerfu/pllm/docs"
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
	
	// Initialize database
	dbConfig := &database.Config{
		DSN:             cfg.Database.URL,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}
	
	if err := database.Initialize(dbConfig); err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer database.Close()
	
	// Initialize cache
	if cfg.Cache.Enabled {
		cacheConfig := &cache.Config{
			RedisURL:  cfg.Redis.URL,
			Password:  cfg.Redis.Password,
			DB:        cfg.Redis.DB,
			TTL:       cfg.Cache.TTL,
			MaxSize:   cfg.Cache.MaxSize,
		}
		
		if err := cache.Initialize(cacheConfig); err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}
		defer cache.Close()
	}
	
	// Initialize model manager
	modelManager := models.NewModelManager(log, cfg.Router)
	if err := modelManager.LoadModelInstances(cfg.ModelList); err != nil {
		log.Fatal("Failed to load model instances", zap.Error(err))
	}
	
	// Create main router
	mainRouter := router.NewRouter(cfg, log, modelManager)
	
	// Create admin router
	adminRouter := router.NewAdminRouter(cfg, log)
	
	// Create metrics router
	metricsRouter := router.NewMetricsRouter(cfg, log)
	
	// Start servers
	servers := []*http.Server{
		{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      mainRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		{
			Addr:         fmt.Sprintf(":%d", cfg.Server.AdminPort),
			Handler:      adminRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		{
			Addr:         fmt.Sprintf(":%d", cfg.Server.MetricsPort),
			Handler:      metricsRouter,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
	}
	
	// Start servers in goroutines
	for i, srv := range servers {
		go func(s *http.Server, idx int) {
			var serverType string
			switch idx {
			case 0:
				serverType = "Main API"
			case 1:
				serverType = "Admin API"
			case 2:
				serverType = "Metrics"
			}
			
			log.Info(fmt.Sprintf("%s server starting", serverType), 
				zap.String("address", s.Addr))
			
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal(fmt.Sprintf("%s server failed to start", serverType), 
					zap.Error(err))
			}
		}(srv, i)
	}
	
	log.Info("pllm Gateway started successfully",
		zap.Int("api_port", cfg.Server.Port),
		zap.Int("admin_port", cfg.Server.AdminPort),
		zap.Int("metrics_port", cfg.Server.MetricsPort))
	
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