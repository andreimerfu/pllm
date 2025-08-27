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
		userBudgetUpdates := make(map[uuid.UUID]float64) // user_id -> amount to add
		teamBudgetUpdates := make(map[uuid.UUID]float64) // team_id -> amount to add

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

			// Collect budget updates from the Budget table
			budgets, err := up.findActivebudgets(tx, usage)
			if err != nil {
				up.logger.Warn("Failed to find budgets for usage record",
					zap.String("record_id", record.ID),
					zap.Error(err))
			} else {
				for _, budget := range budgets {
					budgetUpdates[budget.ID] += record.TotalCost
				}
			}

			// Also update user-level budgets (stored directly in users table)
			if usage.UserID != uuid.Nil {
				userBudgetUpdates[usage.UserID] += record.TotalCost
			}

			// Update team-level budgets (stored directly in teams table)
			if usage.TeamID != nil {
				teamBudgetUpdates[*usage.TeamID] += record.TotalCost
			}
		}

		// Batch insert usage records
		if len(usageModels) > 0 {
			if err := tx.CreateInBatches(usageModels, up.batchSize).Error; err != nil {
				return fmt.Errorf("failed to batch insert usage records: %w", err)
			}
		}

		// Batch update budgets (Budget table)
		if len(budgetUpdates) > 0 {
			if err := up.updateBudgetsBatch(tx, budgetUpdates); err != nil {
				return fmt.Errorf("failed to batch update budgets: %w", err)
			}
		}

		// Batch update user-level budgets (users.current_spend)
		if len(userBudgetUpdates) > 0 {
			if err := up.updateUserBudgetsBatch(tx, userBudgetUpdates); err != nil {
				return fmt.Errorf("failed to batch update user budgets: %w", err)
			}
		}

		// Batch update team-level budgets (teams.current_spend)
		if len(teamBudgetUpdates) > 0 {
			if err := up.updateTeamBudgetsBatch(tx, teamBudgetUpdates); err != nil {
				return fmt.Errorf("failed to batch update team budgets: %w", err)
			}
		}

		// Update cache with latest budget information
		go up.refreshBudgetCaches(context.Background(), budgetUpdates)
		go up.refreshUserBudgetCaches(context.Background(), userBudgetUpdates)
		go up.refreshTeamBudgetCaches(context.Background(), teamBudgetUpdates)

		up.logger.Info("Successfully processed usage batch",
			zap.Int("usage_records", len(usageModels)),
			zap.Int("budget_updates", len(budgetUpdates)),
			zap.Int("user_budget_updates", len(userBudgetUpdates)),
			zap.Int("team_budget_updates", len(teamBudgetUpdates)))

		return nil
	})
}

// convertToUsageModel converts Redis usage record to database model
func (up *UsageProcessor) convertToUsageModel(record *redisService.UsageRecord) (*models.Usage, error) {
	usage := &models.Usage{
		RequestID:    record.RequestID,
		Timestamp:    record.Timestamp,
		Model:        record.Model,
		Provider:     record.Provider,
		Method:       record.Method,
		Path:         record.Path,
		StatusCode:   record.StatusCode,
		InputTokens:  record.InputTokens,
		OutputTokens: record.OutputTokens,
		TotalTokens:  record.TotalTokens,
		TotalCost:    record.TotalCost,
		Latency:      record.Latency,
	}

	// Parse UUIDs for key entities
	if record.UserID != "" {
		if userUUID, err := uuid.Parse(record.UserID); err == nil {
			usage.UserID = userUUID
		}
	}

	// Parse ActualUserID (who made the request)
	if record.ActualUserID != "" {
		if actualUserUUID, err := uuid.Parse(record.ActualUserID); err == nil {
			usage.ActualUserID = actualUserUUID
		}
	} else if record.UserID != "" {
		// Fallback: if ActualUserID not set, use UserID
		if userUUID, err := uuid.Parse(record.UserID); err == nil {
			usage.ActualUserID = userUUID
		}
	}

	if record.KeyID != "" {
		if keyUUID, err := uuid.Parse(record.KeyID); err == nil {
			usage.KeyID = &keyUUID
		}
	}

	// Parse KeyOwnerID
	if record.KeyOwnerID != "" {
		if keyOwnerUUID, err := uuid.Parse(record.KeyOwnerID); err == nil {
			usage.KeyOwnerID = &keyOwnerUUID
		}
	}

	// Parse TeamID
	if record.TeamID != "" {
		if teamUUID, err := uuid.Parse(record.TeamID); err == nil {
			usage.TeamID = &teamUUID
		}
	}

	// Required fields validation
	// We need either:
	// 1. Both actual_user_id and key_id (for API key authentication)
	// 2. Just actual_user_id with no key_id (for JWT authentication)
	if usage.ActualUserID == uuid.Nil {
		return nil, fmt.Errorf("actual_user_id is required for usage record")
	}
	
	// If we have a key_id, it should be valid (not nil), but it's optional for JWT auth
	// No additional validation needed for KeyID - it can be nil for JWT authentication

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

	// Add team budget support when TeamID is available in usage record
	if usage.TeamID != nil {
		if len(conditions) > 0 {
			conditions[0] += " OR team_id = ?"
			args = append(args, *usage.TeamID)
		} else {
			conditions = append(conditions, "team_id = ? OR type = ?")
			args = append(args, *usage.TeamID, models.BudgetTypeGlobal)
		}
	}

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

// updateUserBudgetsBatch performs batch updates to user current_spend amounts
func (up *UsageProcessor) updateUserBudgetsBatch(tx *gorm.DB, updates map[uuid.UUID]float64) error {
	if len(updates) == 0 {
		return nil
	}

	// Use a single UPDATE query with CASE statement for efficiency
	userIDs := make([]uuid.UUID, 0, len(updates))
	for userID := range updates {
		userIDs = append(userIDs, userID)
	}

	// Build CASE statement for atomic update
	caseStmt := "CASE id "
	args := []interface{}{}

	for userID, amount := range updates {
		caseStmt += "WHEN ? THEN current_spend + ? "
		args = append(args, userID, amount)
	}
	caseStmt += "ELSE current_spend END"

	// Execute batch update
	result := tx.Model(&models.User{}).
		Where("id IN ?", userIDs).
		Update("current_spend", gorm.Expr(caseStmt, args...))

	if result.Error != nil {
		return result.Error
	}

	up.logger.Debug("Batch updated user budgets",
		zap.Int("count", len(updates)),
		zap.Int64("affected_rows", result.RowsAffected))

	return nil
}

// updateTeamBudgetsBatch performs batch updates to team current_spend amounts
func (up *UsageProcessor) updateTeamBudgetsBatch(tx *gorm.DB, updates map[uuid.UUID]float64) error {
	if len(updates) == 0 {
		return nil
	}

	// Use a single UPDATE query with CASE statement for efficiency
	teamIDs := make([]uuid.UUID, 0, len(updates))
	for teamID := range updates {
		teamIDs = append(teamIDs, teamID)
	}

	// Build CASE statement for atomic update
	caseStmt := "CASE id "
	args := []interface{}{}

	for teamID, amount := range updates {
		caseStmt += "WHEN ? THEN current_spend + ? "
		args = append(args, teamID, amount)
	}
	caseStmt += "ELSE current_spend END"

	// Execute batch update
	result := tx.Model(&models.Team{}).
		Where("id IN ?", teamIDs).
		Update("current_spend", gorm.Expr(caseStmt, args...))

	if result.Error != nil {
		return result.Error
	}

	up.logger.Debug("Batch updated team budgets",
		zap.Int("count", len(updates)),
		zap.Int64("affected_rows", result.RowsAffected))

	return nil
}

// refreshUserBudgetCaches updates Redis cache with latest user budget information
func (up *UsageProcessor) refreshUserBudgetCaches(ctx context.Context, userUpdates map[uuid.UUID]float64) {
	for userID := range userUpdates {
		// Fetch latest user from database
		var user models.User
		if err := up.db.First(&user, "id = ?", userID).Error; err != nil {
			up.logger.Error("Failed to fetch user for cache refresh",
				zap.String("user_id", userID.String()),
				zap.Error(err))
			continue
		}

		// Update cache
		available := user.MaxBudget - user.CurrentSpend
		isExceeded := user.CurrentSpend >= user.MaxBudget

		if err := up.budgetCache.UpdateBudgetCache(ctx, "user", userID.String(),
			available, user.CurrentSpend, user.MaxBudget, isExceeded); err != nil {
			up.logger.Error("Failed to update user budget cache",
				zap.String("user_id", userID.String()),
				zap.Error(err))
		}
	}
}

// refreshTeamBudgetCaches updates Redis cache with latest team budget information
func (up *UsageProcessor) refreshTeamBudgetCaches(ctx context.Context, teamUpdates map[uuid.UUID]float64) {
	for teamID := range teamUpdates {
		// Fetch latest team from database
		var team models.Team
		if err := up.db.First(&team, "id = ?", teamID).Error; err != nil {
			up.logger.Error("Failed to fetch team for cache refresh",
				zap.String("team_id", teamID.String()),
				zap.Error(err))
			continue
		}

		// Update cache
		available := team.MaxBudget - team.CurrentSpend
		isExceeded := team.CurrentSpend >= team.MaxBudget

		if err := up.budgetCache.UpdateBudgetCache(ctx, "team", teamID.String(),
			available, team.CurrentSpend, team.MaxBudget, isExceeded); err != nil {
			up.logger.Error("Failed to update team budget cache",
				zap.String("team_id", teamID.String()),
				zap.Error(err))
		}
	}
}
