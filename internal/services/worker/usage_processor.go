package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
	redisService "github.com/amerfu/pllm/internal/services/redis"
)

// UsageProcessor handles batch processing of usage records from Redis queue
type UsageProcessor struct {
	db                 *gorm.DB
	logger             *zap.Logger
	usageQueue         *redisService.UsageQueue
	budgetCache        *redisService.BudgetCache
	lockManager        *redisService.LockManager
	batchSize          int
	processingInterval time.Duration
	stopCh             chan struct{}
}

type UsageProcessorConfig struct {
	DB                 *gorm.DB
	Logger             *zap.Logger
	UsageQueue         *redisService.UsageQueue
	BudgetCache        *redisService.BudgetCache
	LockManager        *redisService.LockManager
	BatchSize          int
	ProcessingInterval time.Duration
}

func NewUsageProcessor(config *UsageProcessorConfig) *UsageProcessor {
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.ProcessingInterval == 0 {
		config.ProcessingInterval = 30 * time.Second
	}

	return &UsageProcessor{
		db:                 config.DB,
		logger:             config.Logger,
		usageQueue:         config.UsageQueue,
		budgetCache:        config.BudgetCache,
		lockManager:        config.LockManager,
		batchSize:          config.BatchSize,
		processingInterval: config.ProcessingInterval,
		stopCh:             make(chan struct{}),
	}
}

// Start begins the background processing of usage records
func (up *UsageProcessor) Start(ctx context.Context) error {
	up.logger.Info("Starting usage processor",
		zap.Int("batch_size", up.batchSize),
		zap.Duration("processing_interval", up.processingInterval))

	// Start the main processing loop
	go up.processLoop(ctx)

	// Start retry queue processor
	go up.retryLoop(ctx)

	return nil
}

// Stop gracefully shuts down the usage processor
func (up *UsageProcessor) Stop() error {
	up.logger.Info("Stopping usage processor")
	close(up.stopCh)
	return nil
}

// processLoop main processing loop for usage records
func (up *UsageProcessor) processLoop(ctx context.Context) {
	ticker := time.NewTicker(up.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			up.logger.Info("Usage processor context cancelled")
			return
		case <-up.stopCh:
			up.logger.Info("Usage processor stopped")
			return
		case <-ticker.C:
			if err := up.processBatch(ctx); err != nil {
				up.logger.Error("Error processing usage batch", zap.Error(err))
			}
		}
	}
}

// retryLoop processes retry queue periodically
func (up *UsageProcessor) retryLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Check retry queue every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-up.stopCh:
			return
		case <-ticker.C:
			if err := up.usageQueue.ProcessRetryQueue(ctx); err != nil {
				up.logger.Error("Error processing retry queue", zap.Error(err))
			}
		}
	}
}

// processBatch processes a batch of usage records
func (up *UsageProcessor) processBatch(ctx context.Context) error {
	// Use distributed lock to ensure only one instance processes at a time
	lockKey := "usage_processor_lock"
	lock, err := up.lockManager.AcquireLock(ctx, lockKey, 2*time.Minute)
	if err != nil {
		// Another instance is processing, skip this round
		up.logger.Debug("Could not acquire processing lock, skipping batch")
		return nil
	}
	defer lock.Release(ctx)

	// Get batch of records from queue
	records, err := up.usageQueue.DequeueUsageBatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to dequeue usage batch: %w", err)
	}

	if len(records) == 0 {
		return nil // No records to process
	}

	up.logger.Info("Processing usage batch", zap.Int("count", len(records)))

	// Process records in batches for database efficiency
	batchesToProcess := up.groupRecordsByBatch(records)

	for _, batch := range batchesToProcess {
		if err := up.processBatchTransactional(ctx, batch); err != nil {
			up.logger.Error("Failed to process batch",
				zap.Error(err),
				zap.Int("batch_size", len(batch)))

			// Re-queue failed records for retry
			for _, record := range batch {
				if retryErr := up.usageQueue.EnqueueUsageFailed(ctx, record, err.Error()); retryErr != nil {
					up.logger.Error("Failed to re-queue failed record",
						zap.String("record_id", record.ID),
						zap.Error(retryErr))
				}
			}
		}
	}

	return nil
}

// processBatchTransactional processes a batch of records in a database transaction
func (up *UsageProcessor) processBatchTransactional(ctx context.Context, records []*redisService.UsageRecord) error {
	return up.db.Transaction(func(tx *gorm.DB) error {
		// Convert Redis records to database models
		usageModels := make([]*models.Usage, 0, len(records))
		budgetUpdates := make(map[uuid.UUID]float64) // budget_id -> amount to add

		for _, record := range records {
			// Convert to database model
			usage, err := up.convertToUsageModel(record)
			if err != nil {
				up.logger.Error("Failed to convert usage record",
					zap.String("record_id", record.ID),
					zap.Error(err))
				continue
			}

			usageModels = append(usageModels, usage)

			// Collect budget updates
			budgets, err := up.findActivebudgets(tx, usage)
			if err != nil {
				up.logger.Warn("Failed to find budgets for usage record",
					zap.String("record_id", record.ID),
					zap.Error(err))
				continue
			}

			for _, budget := range budgets {
				budgetUpdates[budget.ID] += record.TotalCost
			}
		}

		// Batch insert usage records
		if len(usageModels) > 0 {
			if err := tx.CreateInBatches(usageModels, up.batchSize).Error; err != nil {
				return fmt.Errorf("failed to batch insert usage records: %w", err)
			}
		}

		// Batch update budgets
		if len(budgetUpdates) > 0 {
			if err := up.updateBudgetsBatch(tx, budgetUpdates); err != nil {
				return fmt.Errorf("failed to batch update budgets: %w", err)
			}
		}

		// Update cache with latest budget information
		go up.refreshBudgetCaches(context.Background(), budgetUpdates)

		up.logger.Info("Successfully processed usage batch",
			zap.Int("usage_records", len(usageModels)),
			zap.Int("budget_updates", len(budgetUpdates)))

		return nil
	})
}

// convertToUsageModel converts Redis usage record to database model
func (up *UsageProcessor) convertToUsageModel(record *redisService.UsageRecord) (*models.Usage, error) {
	usage := &models.Usage{
		RequestID:   record.RequestID,
		Timestamp:   record.Timestamp,
		Model:       record.Model,
		Provider:    record.Provider,
		Method:      record.Method,
		Path:        record.Path,
		StatusCode:  record.StatusCode,
		InputTokens: record.InputTokens,
		TotalCost:   record.TotalCost,
		Latency:     record.Latency,
	}

	// Parse UUIDs
	if record.UserID != "" {
		if userUUID, err := uuid.Parse(record.UserID); err == nil {
			usage.UserID = userUUID
		}
	}

	if record.KeyID != "" {
		if keyUUID, err := uuid.Parse(record.KeyID); err == nil {
			usage.KeyID = keyUUID
		}
	}

	// Both UserID and KeyID are required by the model
	if usage.UserID == uuid.Nil || usage.KeyID == uuid.Nil {
		return nil, fmt.Errorf("both user_id and key_id are required for usage record")
	}

	return usage, nil
}

// findActivebudgets finds all active budgets that apply to a usage record
func (up *UsageProcessor) findActivebudgets(tx *gorm.DB, usage *models.Usage) ([]*models.Budget, error) {
	var budgets []*models.Budget

	query := tx.Model(&models.Budget{}).Where("is_active = ? AND ends_at > ?", true, time.Now())

	// Find budgets that apply to this user or team
	conditions := []string{}
	args := []interface{}{}

	if usage.UserID != uuid.Nil {
		conditions = append(conditions, "user_id = ? OR type = ?")
		args = append(args, usage.UserID, models.BudgetTypeGlobal)
	}

	// TODO: Add team budget support when TeamID is available in usage record

	if len(conditions) > 0 {
		query = query.Where(fmt.Sprintf("(%s)", conditions[0]), args...)
	}

	if err := query.Find(&budgets).Error; err != nil {
		return nil, err
	}

	return budgets, nil
}

// updateBudgetsBatch performs batch updates to budget spent amounts
func (up *UsageProcessor) updateBudgetsBatch(tx *gorm.DB, updates map[uuid.UUID]float64) error {
	if len(updates) == 0 {
		return nil
	}

	// Use a single UPDATE query with CASE statement for efficiency
	budgetIDs := make([]uuid.UUID, 0, len(updates))
	for budgetID := range updates {
		budgetIDs = append(budgetIDs, budgetID)
	}

	// Build CASE statement for atomic update
	caseStmt := "CASE id "
	args := []interface{}{}

	for budgetID, amount := range updates {
		caseStmt += "WHEN ? THEN spent + ? "
		args = append(args, budgetID, amount)
	}
	caseStmt += "ELSE spent END"

	// Execute batch update
	result := tx.Model(&models.Budget{}).
		Where("id IN ?", budgetIDs).
		Update("spent", gorm.Expr(caseStmt, args...))

	if result.Error != nil {
		return result.Error
	}

	up.logger.Debug("Batch updated budgets",
		zap.Int("count", len(updates)),
		zap.Int64("affected_rows", result.RowsAffected))

	return nil
}

// refreshBudgetCaches updates Redis cache with latest budget information
func (up *UsageProcessor) refreshBudgetCaches(ctx context.Context, budgetUpdates map[uuid.UUID]float64) {
	for budgetID := range budgetUpdates {
		// Fetch latest budget from database
		var budget models.Budget
		if err := up.db.First(&budget, "id = ?", budgetID).Error; err != nil {
			up.logger.Error("Failed to fetch budget for cache refresh",
				zap.String("budget_id", budgetID.String()),
				zap.Error(err))
			continue
		}

		// Determine entity type and ID for cache key
		var entityType, entityID string
		if budget.UserID != nil {
			entityType = "user"
			entityID = budget.UserID.String()
		} else if budget.TeamID != nil {
			entityType = "team"
			entityID = budget.TeamID.String()
		} else {
			entityType = "global"
			entityID = "global"
		}

		// Update cache
		available := budget.GetRemainingBudget()
		isExceeded := budget.IsExceeded()

		if err := up.budgetCache.UpdateBudgetCache(ctx, entityType, entityID,
			available, budget.Spent, budget.Amount, isExceeded); err != nil {
			up.logger.Error("Failed to update budget cache",
				zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
				zap.Error(err))
		}
	}
}

// groupRecordsByBatch groups records into smaller batches for processing
func (up *UsageProcessor) groupRecordsByBatch(records []*redisService.UsageRecord) [][]*redisService.UsageRecord {
	if len(records) <= up.batchSize {
		return [][]*redisService.UsageRecord{records}
	}

	var batches [][]*redisService.UsageRecord
	for i := 0; i < len(records); i += up.batchSize {
		end := i + up.batchSize
		if end > len(records) {
			end = len(records)
		}
		batches = append(batches, records[i:end])
	}

	return batches
}

// GetProcessorStats returns statistics about the processor
func (up *UsageProcessor) GetProcessorStats(ctx context.Context) (*ProcessorStats, error) {
	queueStats, err := up.usageQueue.GetQueueStats(ctx)
	if err != nil {
		return nil, err
	}

	return &ProcessorStats{
		QueueStats:         *queueStats,
		ProcessingInterval: up.processingInterval,
		BatchSize:          up.batchSize,
		IsRunning:          up.isRunning(),
	}, nil
}

// ProcessorStats represents processor statistics
type ProcessorStats struct {
	QueueStats         redisService.QueueStats `json:"queue_stats"`
	ProcessingInterval time.Duration           `json:"processing_interval"`
	BatchSize          int                     `json:"batch_size"`
	IsRunning          bool                    `json:"is_running"`
}

// isRunning checks if the processor is currently running
func (up *UsageProcessor) isRunning() bool {
	select {
	case <-up.stopCh:
		return false
	default:
		return true
	}
}
