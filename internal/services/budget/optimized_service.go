package budget

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

// OptimizedBudgetService provides high-performance budget operations
type OptimizedBudgetService struct {
	db          *gorm.DB
	logger      *zap.Logger
	budgetCache *redisService.BudgetCache
	lockManager *redisService.LockManager
}

type OptimizedBudgetConfig struct {
	DB          *gorm.DB
	Logger      *zap.Logger
	BudgetCache *redisService.BudgetCache
	LockManager *redisService.LockManager
}

func NewOptimizedBudgetService(config *OptimizedBudgetConfig) *OptimizedBudgetService {
	return &OptimizedBudgetService{
		db:          config.DB,
		logger:      config.Logger,
		budgetCache: config.BudgetCache,
		lockManager: config.LockManager,
	}
}

// CheckBudgetCached performs fast budget check using Redis cache
func (s *OptimizedBudgetService) CheckBudgetCached(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	// First check Redis cache for immediate response
	available, err := s.budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, requestCost)
	if err == nil {
		return available, nil
	}

	// Cache miss - check database and populate cache
	return s.checkBudgetFromDB(ctx, entityType, entityID, requestCost)
}

// checkBudgetFromDB checks budget from database and updates cache
func (s *OptimizedBudgetService) checkBudgetFromDB(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	// Use distributed lock to prevent race conditions during cache population
	lockKey := fmt.Sprintf("budget_check_%s_%s", entityType, entityID)
	err := s.lockManager.WithLock(ctx, lockKey, 10*time.Second, func() error {
		// Double-check cache after acquiring lock
		if _, err := s.budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, requestCost); err == nil {
			return nil // Cache was populated by another instance
		}

		// Load active budgets for entity
		budgets, err := s.loadActiveBudgetsForEntity(ctx, entityType, entityID)
		if err != nil {
			return err
		}

		if len(budgets) == 0 {
			// No budgets configured - allow request and cache result
			s.budgetCache.UpdateBudgetCache(ctx, entityType, entityID,
				float64(999999), 0, float64(999999), false)
			return nil
		}

		// Calculate total available budget and spent
		var totalLimit, totalSpent float64
		var isAnyExceeded bool

		for _, budget := range budgets {
			totalLimit += budget.Amount
			totalSpent += budget.Spent
			if budget.IsExceeded() {
				isAnyExceeded = true
			}
		}

		available := totalLimit - totalSpent

		// Update cache with latest values
		return s.budgetCache.UpdateBudgetCache(ctx, entityType, entityID,
			available, totalSpent, totalLimit, isAnyExceeded)
	})

	if err != nil {
		s.logger.Error("Error checking budget from database",
			zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
			zap.Error(err))
		// On error, optimistically allow request
		return true, nil
	}

	// Final check after cache update
	return s.budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, requestCost)
}

// loadActiveBudgetsForEntity loads all active budgets for an entity with optimized query
func (s *OptimizedBudgetService) loadActiveBudgetsForEntity(ctx context.Context, entityType, entityID string) ([]*models.Budget, error) {
	var budgets []*models.Budget

	query := s.db.WithContext(ctx).Model(&models.Budget{}).
		Where("is_active = ? AND ends_at > ?", true, time.Now())

	switch entityType {
	case "user":
		userUUID, err := uuid.Parse(entityID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		query = query.Where("user_id = ? OR type = ?", userUUID, models.BudgetTypeGlobal)

	case "key":
		// For API keys, we need to find the associated user/team
		// This requires a join with the keys table
		keyUUID, err := uuid.Parse(entityID)
		if err != nil {
			return nil, fmt.Errorf("invalid key ID: %w", err)
		}

		// Use subquery to find budgets for the key's user/team
		subQuery := s.db.Model(&models.Key{}).Select("user_id, team_id").Where("id = ?", keyUUID)
		query = query.Where(`
			user_id IN (SELECT user_id FROM (?) AS key_info WHERE user_id IS NOT NULL)
			OR team_id IN (SELECT team_id FROM (?) AS key_info WHERE team_id IS NOT NULL)
			OR type = ?
		`, subQuery, subQuery, models.BudgetTypeGlobal)

	case "team":
		teamUUID, err := uuid.Parse(entityID)
		if err != nil {
			return nil, fmt.Errorf("invalid team ID: %w", err)
		}
		query = query.Where("team_id = ? OR type = ?", teamUUID, models.BudgetTypeGlobal)

	default:
		// Global or unknown type
		query = query.Where("type = ?", models.BudgetTypeGlobal)
	}

	if err := query.Find(&budgets).Error; err != nil {
		return nil, fmt.Errorf("failed to load budgets for %s:%s: %w", entityType, entityID, err)
	}

	s.logger.Debug("Loaded active budgets for entity",
		zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
		zap.Int("count", len(budgets)))

	return budgets, nil
}

// BatchUpdateBudgetSpending performs efficient batch updates of budget spending
func (s *OptimizedBudgetService) BatchUpdateBudgetSpending(ctx context.Context, updates map[uuid.UUID]float64) error {
	if len(updates) == 0 {
		return nil
	}

	// Group updates into smaller batches for database efficiency
	const maxBatchSize = 50
	batches := s.groupUpdatesIntoBatches(updates, maxBatchSize)

	for _, batch := range batches {
		if err := s.executeBatchUpdate(ctx, batch); err != nil {
			return fmt.Errorf("failed to execute batch update: %w", err)
		}
	}

	// Update cache for affected budgets asynchronously
	go s.refreshBudgetCachesAfterUpdate(context.Background(), updates)

	return nil
}

// executeBatchUpdate executes a single batch of budget updates
func (s *OptimizedBudgetService) executeBatchUpdate(ctx context.Context, updates map[uuid.UUID]float64) error {
	if len(updates) == 0 {
		return nil
	}

	budgetIDs := make([]uuid.UUID, 0, len(updates))
	for budgetID := range updates {
		budgetIDs = append(budgetIDs, budgetID)
	}

	// Use a CASE statement for atomic updates
	caseStmt := "CASE id "
	args := []interface{}{}

	for budgetID, amount := range updates {
		caseStmt += "WHEN ? THEN spent + ? "
		args = append(args, budgetID, amount)
	}
	caseStmt += "ELSE spent END"

	// Execute batch update with optimistic locking
	result := s.db.WithContext(ctx).Model(&models.Budget{}).
		Where("id IN ?", budgetIDs).
		Where("is_active = ?", true). // Additional safety check
		Update("spent", gorm.Expr(caseStmt, args...))

	if result.Error != nil {
		return result.Error
	}

	s.logger.Debug("Batch updated budget spending",
		zap.Int("count", len(updates)),
		zap.Int64("affected_rows", result.RowsAffected))

	return nil
}

// groupUpdatesIntoBatches splits updates into smaller batches
func (s *OptimizedBudgetService) groupUpdatesIntoBatches(updates map[uuid.UUID]float64, batchSize int) []map[uuid.UUID]float64 {
	var batches []map[uuid.UUID]float64
	currentBatch := make(map[uuid.UUID]float64)

	for budgetID, amount := range updates {
		currentBatch[budgetID] = amount

		if len(currentBatch) >= batchSize {
			batches = append(batches, currentBatch)
			currentBatch = make(map[uuid.UUID]float64)
		}
	}

	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// refreshBudgetCachesAfterUpdate refreshes Redis cache after budget updates
func (s *OptimizedBudgetService) refreshBudgetCachesAfterUpdate(ctx context.Context, updates map[uuid.UUID]float64) {
	budgetIDs := make([]uuid.UUID, 0, len(updates))
	for budgetID := range updates {
		budgetIDs = append(budgetIDs, budgetID)
	}

	// Load updated budgets from database
	var budgets []*models.Budget
	if err := s.db.WithContext(ctx).Where("id IN ?", budgetIDs).Find(&budgets).Error; err != nil {
		s.logger.Error("Failed to load budgets for cache refresh", zap.Error(err))
		return
	}

	// Update cache for each budget
	for _, budget := range budgets {
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

		available := budget.GetRemainingBudget()
		isExceeded := budget.IsExceeded()

		if err := s.budgetCache.UpdateBudgetCache(ctx, entityType, entityID,
			available, budget.Spent, budget.Amount, isExceeded); err != nil {
			s.logger.Error("Failed to update budget cache",
				zap.String("budget_id", budget.ID.String()),
				zap.Error(err))
		}
	}

	s.logger.Debug("Refreshed budget caches after update",
		zap.Int("count", len(budgets)))
}

// RefreshAllBudgetCaches refreshes all budget caches (useful for initial startup)
func (s *OptimizedBudgetService) RefreshAllBudgetCaches(ctx context.Context) error {
	// Load all active budgets
	var budgets []*models.Budget
	err := s.db.WithContext(ctx).Where("is_active = ? AND ends_at > ?", true, time.Now()).Find(&budgets).Error
	if err != nil {
		return fmt.Errorf("failed to load active budgets: %w", err)
	}

	// Group budgets by entity for cache updates
	entityBudgets := make(map[string]map[string][]*models.Budget)

	for _, budget := range budgets {
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

		if entityBudgets[entityType] == nil {
			entityBudgets[entityType] = make(map[string][]*models.Budget)
		}
		entityBudgets[entityType][entityID] = append(entityBudgets[entityType][entityID], budget)
	}

	// Update cache for each entity
	for entityType, entities := range entityBudgets {
		for entityID, budgets := range entities {
			var totalLimit, totalSpent float64
			var isAnyExceeded bool

			for _, budget := range budgets {
				totalLimit += budget.Amount
				totalSpent += budget.Spent
				if budget.IsExceeded() {
					isAnyExceeded = true
				}
			}

			available := totalLimit - totalSpent

			if err := s.budgetCache.UpdateBudgetCache(ctx, entityType, entityID,
				available, totalSpent, totalLimit, isAnyExceeded); err != nil {
				s.logger.Error("Failed to refresh budget cache",
					zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
					zap.Error(err))
			}
		}
	}

	s.logger.Info("Refreshed all budget caches",
		zap.Int("total_budgets", len(budgets)),
		zap.Int("entity_types", len(entityBudgets)))

	return nil
}

// GetBudgetStatsBatch gets budget statistics for multiple entities efficiently
func (s *OptimizedBudgetService) GetBudgetStatsBatch(ctx context.Context, entities []struct {
	Type string
	ID   string
}) (map[string]*redisService.BudgetStatus, error) {
	results := make(map[string]*redisService.BudgetStatus)

	// Try to get all from cache first
	cacheHits := 0
	cacheMisses := []struct {
		Type string
		ID   string
	}{}

	for _, entity := range entities {
		key := fmt.Sprintf("%s:%s", entity.Type, entity.ID)
		status, err := s.budgetCache.GetBudgetStats(ctx, entity.Type, entity.ID)
		if err == nil && status != nil {
			results[key] = status
			cacheHits++
		} else {
			cacheMisses = append(cacheMisses, entity)
		}
	}

	// Load cache misses from database
	if len(cacheMisses) > 0 {
		for _, entity := range cacheMisses {
			key := fmt.Sprintf("%s:%s", entity.Type, entity.ID)

			// Check database and update cache
			budgets, err := s.loadActiveBudgetsForEntity(ctx, entity.Type, entity.ID)
			if err != nil {
				s.logger.Error("Failed to load budgets for batch stats",
					zap.String("entity", key),
					zap.Error(err))
				continue
			}

			var totalLimit, totalSpent float64
			var isAnyExceeded bool

			for _, budget := range budgets {
				totalLimit += budget.Amount
				totalSpent += budget.Spent
				if budget.IsExceeded() {
					isAnyExceeded = true
				}
			}

			available := totalLimit - totalSpent
			status := &redisService.BudgetStatus{
				EntityID:    entity.ID,
				EntityType:  entity.Type,
				Available:   available,
				Spent:       totalSpent,
				Limit:       totalLimit,
				Percentage:  (totalSpent / totalLimit) * 100,
				IsExceeded:  isAnyExceeded,
				LastUpdated: time.Now(),
			}

			results[key] = status

			// Update cache asynchronously
			go s.budgetCache.UpdateBudgetCache(context.Background(), entity.Type, entity.ID,
				available, totalSpent, totalLimit, isAnyExceeded)
		}
	}

	s.logger.Debug("Retrieved budget stats batch",
		zap.Int("total_requested", len(entities)),
		zap.Int("cache_hits", cacheHits),
		zap.Int("cache_misses", len(cacheMisses)))

	return results, nil
}
