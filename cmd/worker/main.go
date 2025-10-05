package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/config"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/worker"
)

func main() {
	var (
		configPath         = flag.String("config", "", "Path to config file")
		batchSize          = flag.Int("batch-size", 100, "Batch size for processing")
		processingInterval = flag.Duration("interval", 30*time.Second, "Processing interval")
		logLevel           = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Initialize logger
	logger, err := initLogger(*logLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Starting PLLM Usage Worker",
		zap.Int("batch_size", *batchSize),
		zap.Duration("processing_interval", *processingInterval))

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Initialize database
	db, err := initDatabase(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize Redis client
	redisClient, err := initRedis(cfg.Redis, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}

	// Initialize Redis services
	budgetCache := redisService.NewBudgetCache(redisClient, logger, 5*time.Minute)
	usageQueue := redisService.NewUsageQueue(&redisService.UsageQueueConfig{
		Client:     redisClient,
		Logger:     logger,
		BatchSize:  *batchSize,
		MaxRetries: 3,
	})
	lockManager := redisService.NewLockManager(redisClient, logger)

	// Initialize usage processor
	processor := worker.NewUsageProcessor(&worker.UsageProcessorConfig{
		DB:                 db,
		Logger:             logger,
		UsageQueue:         usageQueue,
		BudgetCache:        budgetCache,
		LockManager:        lockManager,
		BatchSize:          *batchSize,
		ProcessingInterval: *processingInterval,
	})

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the processor
	if err := processor.Start(ctx); err != nil {
		logger.Fatal("Failed to start usage processor", zap.Error(err))
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Usage worker started successfully")

	// Start health check server (optional)
	go startHealthCheckServer(":8082", processor, logger)

	// Wait for shutdown signal
	<-sigCh
	logger.Info("Shutdown signal received, stopping worker...")

	// Cancel context to stop processor
	cancel()

	// Give processor time to finish current batch
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop processor
	if err := processor.Stop(); err != nil {
		logger.Error("Error stopping processor", zap.Error(err))
	}

	// Wait for graceful shutdown or timeout
	done := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Second) // Allow current operations to complete
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Worker stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("Worker shutdown timeout reached")
	}

	// Close connections
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	_ = redisClient.Close()

	logger.Info("Usage worker shutdown complete")
}

func initLogger(level string) (*zap.Logger, error) {
	var zapLevel zap.AtomicLevel
	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config := zap.NewProductionConfig()
	config.Level = zapLevel
	return config.Build()
}

func initDatabase(cfg config.DatabaseConfig, logger *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: nil, // Use custom logger if needed
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	logger.Info("Database connection established",
		zap.String("url", cfg.URL),
		zap.Int("max_connections", cfg.MaxConnections))

	return db, nil
}

func initRedis(cfg config.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.Password != "" {
		opt.Password = cfg.Password
	}
	if cfg.DB != 0 {
		opt.DB = cfg.DB
	}
	if cfg.PoolSize != 0 {
		opt.PoolSize = cfg.PoolSize
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	logger.Info("Redis connection established",
		zap.String("url", cfg.URL),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize))

	return client, nil
}

func startHealthCheckServer(addr string, processor *worker.UsageProcessor, logger *zap.Logger) {
	// Simple HTTP health check server
	// This is a minimal implementation - you could expand this with proper HTTP handlers
	logger.Info("Health check server would start here", zap.String("addr", addr))

	// TODO: Implement HTTP server with health endpoints:
	// GET /health - basic health check
	// GET /health/detailed - processor statistics
	// GET /metrics - Prometheus metrics (optional)
}
