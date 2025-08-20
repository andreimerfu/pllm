package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/budget"
	redisService "github.com/amerfu/pllm/internal/services/redis"
	"github.com/amerfu/pllm/internal/services/worker"
)

// AsyncBudgetIntegration orchestrates all async budget and usage tracking components
type AsyncBudgetIntegration struct {
	// Core services
	db          *gorm.DB
	redisClient *redis.Client
	logger      *zap.Logger
	authService *auth.AuthService

	// Redis services
	budgetCache *redisService.BudgetCache
	eventPub    *redisService.EventPublisher
	usageQueue  *redisService.UsageQueue
	lockManager *redisService.LockManager

	// Business logic services
	optimizedBudgetService *budget.OptimizedBudgetService
	usageProcessor         *worker.UsageProcessor

	// Middleware
	asyncBudgetMiddleware *middleware.AsyncBudgetMiddleware

	// Configuration
	config *AsyncBudgetConfig
}

type AsyncBudgetConfig struct {
	// Redis configuration
	BudgetCacheTTL     time.Duration
	UsageQueueBatch    int
	UsageMaxRetries    int
	ProcessingInterval time.Duration

	// Performance tuning
	DatabaseBatchSize int

	// Feature flags
	EnableEventPublishing bool
	EnableCacheWarmup     bool
}

func NewAsyncBudgetIntegration(
	db *gorm.DB,
	redisClient *redis.Client,
	logger *zap.Logger,
	authService *auth.AuthService,
	config *AsyncBudgetConfig,
) (*AsyncBudgetIntegration, error) {

	// Set defaults
	if config == nil {
		config = &AsyncBudgetConfig{}
	}
	if config.BudgetCacheTTL == 0 {
		config.BudgetCacheTTL = 5 * time.Minute
	}
	if config.UsageQueueBatch == 0 {
		config.UsageQueueBatch = 100
	}
	if config.UsageMaxRetries == 0 {
		config.UsageMaxRetries = 3
	}
	if config.ProcessingInterval == 0 {
		config.ProcessingInterval = 30 * time.Second
	}
	if config.DatabaseBatchSize == 0 {
		config.DatabaseBatchSize = 100
	}

	integration := &AsyncBudgetIntegration{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
		authService: authService,
		config:      config,
	}

	// Initialize Redis services
	if err := integration.initializeRedisServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize Redis services: %w", err)
	}

	// Initialize business logic services
	if err := integration.initializeBusinessServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize business services: %w", err)
	}

	// Initialize middleware
	if err := integration.initializeMiddleware(); err != nil {
		return nil, fmt.Errorf("failed to initialize middleware: %w", err)
	}

	logger.Info("Async budget integration initialized successfully",
		zap.Duration("cache_ttl", config.BudgetCacheTTL),
		zap.Int("batch_size", config.UsageQueueBatch),
		zap.Duration("processing_interval", config.ProcessingInterval))

	return integration, nil
}

func (abi *AsyncBudgetIntegration) initializeRedisServices() error {
	// Budget cache for fast lookups
	abi.budgetCache = redisService.NewBudgetCache(
		abi.redisClient,
		abi.logger.Named("budget-cache"),
		abi.config.BudgetCacheTTL,
	)

	// Event publisher for real-time monitoring (optional)
	if abi.config.EnableEventPublishing {
		abi.eventPub = redisService.NewEventPublisher(
			abi.redisClient,
			abi.logger.Named("event-publisher"),
		)
	}

	// Usage queue for batch processing
	abi.usageQueue = redisService.NewUsageQueue(&redisService.UsageQueueConfig{
		Client:     abi.redisClient,
		Logger:     abi.logger.Named("usage-queue"),
		BatchSize:  abi.config.UsageQueueBatch,
		MaxRetries: abi.config.UsageMaxRetries,
	})

	// Distributed lock manager
	abi.lockManager = redisService.NewLockManager(
		abi.redisClient,
		abi.logger.Named("lock-manager"),
	)

	return nil
}

func (abi *AsyncBudgetIntegration) initializeBusinessServices() error {
	// Optimized budget service
	abi.optimizedBudgetService = budget.NewOptimizedBudgetService(&budget.OptimizedBudgetConfig{
		DB:          abi.db,
		Logger:      abi.logger.Named("optimized-budget"),
		BudgetCache: abi.budgetCache,
		LockManager: abi.lockManager,
	})

	// Usage processor (background worker functionality)
	abi.usageProcessor = worker.NewUsageProcessor(&worker.UsageProcessorConfig{
		DB:                 abi.db,
		Logger:             abi.logger.Named("usage-processor"),
		UsageQueue:         abi.usageQueue,
		BudgetCache:        abi.budgetCache,
		LockManager:        abi.lockManager,
		BatchSize:          abi.config.DatabaseBatchSize,
		ProcessingInterval: abi.config.ProcessingInterval,
	})

	return nil
}

func (abi *AsyncBudgetIntegration) initializeMiddleware() error {
	// Async budget middleware
	abi.asyncBudgetMiddleware = middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
		Logger:      abi.logger.Named("async-budget-middleware"),
		AuthService: abi.authService,
		BudgetCache: abi.budgetCache,
		EventPub:    abi.eventPub, // Can be nil if disabled
		UsageQueue:  abi.usageQueue,
	})

	return nil
}

// Start starts all background services
func (abi *AsyncBudgetIntegration) Start(ctx context.Context) error {
	abi.logger.Info("Starting async budget integration services")

	// Warm up budget cache if enabled
	if abi.config.EnableCacheWarmup {
		if err := abi.WarmupBudgetCache(ctx); err != nil {
			abi.logger.Warn("Failed to warm up budget cache", zap.Error(err))
		}
	}

	// Start usage processor (background worker)
	if err := abi.usageProcessor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start usage processor: %w", err)
	}

	abi.logger.Info("Async budget integration started successfully")
	return nil
}

// Stop gracefully stops all services
func (abi *AsyncBudgetIntegration) Stop() error {
	abi.logger.Info("Stopping async budget integration services")

	// Stop usage processor
	if err := abi.usageProcessor.Stop(); err != nil {
		abi.logger.Error("Error stopping usage processor", zap.Error(err))
	}

	abi.logger.Info("Async budget integration stopped")
	return nil
}

// GetAsyncBudgetMiddleware returns the configured async budget middleware
func (abi *AsyncBudgetIntegration) GetAsyncBudgetMiddleware() *middleware.AsyncBudgetMiddleware {
	return abi.asyncBudgetMiddleware
}

// GetOptimizedBudgetService returns the optimized budget service
func (abi *AsyncBudgetIntegration) GetOptimizedBudgetService() *budget.OptimizedBudgetService {
	return abi.optimizedBudgetService
}

// GetUsageProcessor returns the usage processor for external management
func (abi *AsyncBudgetIntegration) GetUsageProcessor() *worker.UsageProcessor {
	return abi.usageProcessor
}

// WarmupBudgetCache pre-loads budget cache with active budgets
func (abi *AsyncBudgetIntegration) WarmupBudgetCache(ctx context.Context) error {
	abi.logger.Info("Warming up budget cache")

	start := time.Now()
	err := abi.optimizedBudgetService.RefreshAllBudgetCaches(ctx)
	duration := time.Since(start)

	if err != nil {
		abi.logger.Error("Budget cache warmup failed",
			zap.Duration("duration", duration),
			zap.Error(err))
		return err
	}

	abi.logger.Info("Budget cache warmup completed successfully",
		zap.Duration("duration", duration))

	return nil
}

// HealthCheck performs health check on all integrated services
func (abi *AsyncBudgetIntegration) HealthCheck(ctx context.Context) error {
	// Check Redis connectivity
	if err := abi.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connectivity check failed: %w", err)
	}

	// Check database connectivity
	if sqlDB, err := abi.db.DB(); err == nil {
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("database connectivity check failed: %w", err)
		}
	}

	// Check usage queue health
	if err := abi.usageQueue.HealthCheck(ctx); err != nil {
		return fmt.Errorf("usage queue health check failed: %w", err)
	}

	return nil
}

// GetStats returns comprehensive statistics about the integration
func (abi *AsyncBudgetIntegration) GetStats(ctx context.Context) (*IntegrationStats, error) {
	// Get queue stats
	queueStats, err := abi.usageQueue.GetQueueStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	// Get processor stats
	processorStats, err := abi.usageProcessor.GetProcessorStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get processor stats: %w", err)
	}

	return &IntegrationStats{
		QueueStats:     *queueStats,
		ProcessorStats: *processorStats,
		CacheTTL:       abi.config.BudgetCacheTTL,
		BatchSize:      abi.config.UsageQueueBatch,
		IsHealthy:      abi.isHealthy(ctx),
	}, nil
}

// IntegrationStats represents comprehensive integration statistics
type IntegrationStats struct {
	QueueStats     redisService.QueueStats `json:"queue_stats"`
	ProcessorStats worker.ProcessorStats   `json:"processor_stats"`
	CacheTTL       time.Duration           `json:"cache_ttl"`
	BatchSize      int                     `json:"batch_size"`
	IsHealthy      bool                    `json:"is_healthy"`
}

// isHealthy performs a quick health check
func (abi *AsyncBudgetIntegration) isHealthy(ctx context.Context) bool {
	return abi.HealthCheck(ctx) == nil
}

// ForceBudgetCacheRefresh forces a refresh of budget caches (admin operation)
func (abi *AsyncBudgetIntegration) ForceBudgetCacheRefresh(ctx context.Context) error {
	lockKey := "force_cache_refresh"
	return abi.lockManager.WithLock(ctx, lockKey, 2*time.Minute, func() error {
		return abi.optimizedBudgetService.RefreshAllBudgetCaches(ctx)
	})
}

// ClearUsageQueue clears all pending usage records (emergency operation)
func (abi *AsyncBudgetIntegration) ClearUsageQueue(ctx context.Context) error {
	abi.logger.Warn("Clearing usage queue - this will result in data loss")
	return abi.usageQueue.ClearQueue(ctx)
}
