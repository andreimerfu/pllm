package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// BudgetStatus represents cached budget status
type BudgetStatus struct {
	EntityID    string    `json:"entity_id"`
	EntityType  string    `json:"entity_type"`
	Available   float64   `json:"available"`
	Spent       float64   `json:"spent"`
	Limit       float64   `json:"limit"`
	Percentage  float64   `json:"percentage"`
	IsExceeded  bool      `json:"is_exceeded"`
	LastUpdated time.Time `json:"last_updated"`
	TTL         int64     `json:"ttl"`
}

// BudgetCache manages distributed budget caching with Redis
type BudgetCache struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// NewBudgetCache creates a new budget cache
func NewBudgetCache(client *redis.Client, logger *zap.Logger, ttl time.Duration) *BudgetCache {
	if ttl == 0 {
		ttl = 5 * time.Minute // Default TTL
	}

	return &BudgetCache{
		client: client,
		logger: logger,
		ttl:    ttl,
	}
}

// CheckBudgetAvailable performs a fast budget check using Redis cache
func (bc *BudgetCache) CheckBudgetAvailable(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	cacheKey := bc.budgetKey(entityType, entityID)

	// Try to get from cache first
	status, err := bc.getBudgetStatus(ctx, cacheKey)
	if err == nil && status != nil {
		// Cache hit - return quick result
		available := status.Available - requestCost
		return available >= 0 && !status.IsExceeded, nil
	}

	// Cache miss - return optimistic result and trigger background refresh
	bc.logger.Debug("Budget cache miss, allowing request optimistically",
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID))

	// Trigger async budget refresh
	go bc.refreshBudgetStatus(context.Background(), entityType, entityID)

	// Return optimistic result (allow request to proceed)
	return true, nil
}

// UpdateBudgetCache updates cached budget status
func (bc *BudgetCache) UpdateBudgetCache(ctx context.Context, entityType, entityID string, available, spent, limit float64, isExceeded bool) error {
	cacheKey := bc.budgetKey(entityType, entityID)

	status := &BudgetStatus{
		EntityID:    entityID,
		EntityType:  entityType,
		Available:   available,
		Spent:       spent,
		Limit:       limit,
		Percentage:  (spent / limit) * 100,
		IsExceeded:  isExceeded,
		LastUpdated: time.Now(),
		TTL:         int64(bc.ttl.Seconds()),
	}

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal budget status: %w", err)
	}

	err = bc.client.SetEx(ctx, cacheKey, data, bc.ttl).Err()
	if err != nil {
		bc.logger.Error("Failed to update budget cache",
			zap.Error(err),
			zap.String("cache_key", cacheKey))
		return err
	}

	bc.logger.Debug("Budget cache updated",
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID),
		zap.Float64("available", available),
		zap.Float64("spent", spent))

	return nil
}

// IncrementSpent atomically increments the spent amount
func (bc *BudgetCache) IncrementSpent(ctx context.Context, entityType, entityID string, amount float64) error {
	cacheKey := bc.budgetKey(entityType, entityID)
	spentKey := fmt.Sprintf("%s:spent", cacheKey)

	// Use Redis INCRBYFLOAT for atomic increment
	newSpent, err := bc.client.IncrByFloat(ctx, spentKey, amount).Result()
	if err != nil {
		return fmt.Errorf("failed to increment spent amount: %w", err)
	}

	// Set TTL if it's a new key
	bc.client.Expire(ctx, spentKey, bc.ttl)

	bc.logger.Debug("Budget spent incremented",
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID),
		zap.Float64("amount", amount),
		zap.Float64("new_spent", newSpent))

	return nil
}

// InvalidateBudgetCache removes cached budget status
func (bc *BudgetCache) InvalidateBudgetCache(ctx context.Context, entityType, entityID string) error {
	cacheKey := bc.budgetKey(entityType, entityID)
	spentKey := fmt.Sprintf("%s:spent", cacheKey)

	// Delete both the status and spent counter
	pipe := bc.client.Pipeline()
	pipe.Del(ctx, cacheKey)
	pipe.Del(ctx, spentKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		bc.logger.Error("Failed to invalidate budget cache",
			zap.Error(err),
			zap.String("cache_key", cacheKey))
		return err
	}

	bc.logger.Debug("Budget cache invalidated",
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID))

	return nil
}

// GetBudgetStats returns current budget statistics from cache
func (bc *BudgetCache) GetBudgetStats(ctx context.Context, entityType, entityID string) (*BudgetStatus, error) {
	cacheKey := bc.budgetKey(entityType, entityID)
	return bc.getBudgetStatus(ctx, cacheKey)
}

// refreshBudgetStatus triggers background refresh of budget status
func (bc *BudgetCache) refreshBudgetStatus(ctx context.Context, entityType, entityID string) {
	// This would be called from the main budget service
	// For now, just log the refresh request
	bc.logger.Debug("Budget refresh requested",
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID))

	// TODO: Call budget service to refresh actual data
	// budgetService.RefreshBudgetStatus(ctx, entityType, entityID)
}

// getBudgetStatus retrieves budget status from cache
func (bc *BudgetCache) getBudgetStatus(ctx context.Context, cacheKey string) (*BudgetStatus, error) {
	data, err := bc.client.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get budget status from cache: %w", err)
	}

	var status BudgetStatus
	err = json.Unmarshal([]byte(data), &status)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal budget status: %w", err)
	}

	return &status, nil
}

// budgetKey generates the Redis key for budget status
func (bc *BudgetCache) budgetKey(entityType, entityID string) string {
	return fmt.Sprintf("budget:%s:%s", entityType, entityID)
}

// SetupBudgetLimits initializes budget limits in Redis for fast access
func (bc *BudgetCache) SetupBudgetLimits(ctx context.Context, limits map[string]map[string]float64) error {
	pipe := bc.client.Pipeline()

	for entityType, entities := range limits {
		for entityID, limit := range entities {
			status := &BudgetStatus{
				EntityID:    entityID,
				EntityType:  entityType,
				Available:   limit,
				Spent:       0,
				Limit:       limit,
				Percentage:  0,
				IsExceeded:  false,
				LastUpdated: time.Now(),
				TTL:         int64(bc.ttl.Seconds()),
			}

			data, _ := json.Marshal(status)
			cacheKey := bc.budgetKey(entityType, entityID)
			pipe.SetEx(ctx, cacheKey, data, bc.ttl)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		bc.logger.Error("Failed to setup budget limits", zap.Error(err))
		return err
	}

	bc.logger.Info("Budget limits initialized in Redis",
		zap.Int("entity_types", len(limits)))

	return nil
}
