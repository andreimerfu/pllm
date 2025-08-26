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

// UnifiedService consolidates all budget operations into a single service
// It uses the async Redis system for high performance
type UnifiedService struct {
	db          *gorm.DB
	logger      *zap.Logger
	budgetCache *redisService.BudgetCache
	usageQueue  *redisService.UsageQueue
	eventPub    *redisService.EventPublisher
}

type UnifiedServiceConfig struct {
	DB          *gorm.DB
	Logger      *zap.Logger
	BudgetCache *redisService.BudgetCache
	UsageQueue  *redisService.UsageQueue
	EventPub    *redisService.EventPublisher
}

// NewUnifiedService creates a new consolidated budget service
func NewUnifiedService(config *UnifiedServiceConfig) Service {
	return &UnifiedService{
		db:          config.DB,
		logger:      config.Logger,
		budgetCache: config.BudgetCache,
		usageQueue:  config.UsageQueue,
		eventPub:    config.EventPub,
	}
}

// CheckBudget validates if a request can be made within budget limits
func (s *UnifiedService) CheckBudget(ctx context.Context, keyID uuid.UUID, estimatedCost float64) (*BudgetCheck, error) {
	// Get the key and its associated user/team
	var key models.Key
	if err := s.db.WithContext(ctx).Preload("User").Preload("Team").First(&key, keyID).Error; err != nil {
		return nil, fmt.Errorf("failed to find key: %w", err)
	}

	// Determine budget to check (team takes precedence)
	var maxBudget float64
	var currentSpend float64
	var budgetPeriod models.BudgetPeriod
	var entityType string

	if key.TeamID != nil && key.Team != nil {
		maxBudget = key.Team.MaxBudget
		currentSpend = key.Team.CurrentSpend
		budgetPeriod = key.Team.BudgetDuration
		entityType = "team"
	} else if key.MaxBudget != nil && *key.MaxBudget > 0 {
		maxBudget = *key.MaxBudget
		currentSpend = key.CurrentSpend
		if key.BudgetDuration != nil {
			budgetPeriod = *key.BudgetDuration
		}
		entityType = "key"
	} else {
		// No budget limits - allow request
		return &BudgetCheck{
			Allowed:         true,
			RemainingBudget: 0,
			UsedBudget:      currentSpend,
			TotalBudget:     0,
			Period:          "none",
			Message:         "No budget limits configured",
		}, nil
	}

	// Check if request would exceed budget
	remaining := maxBudget - currentSpend
	allowed := remaining >= estimatedCost

	result := &BudgetCheck{
		Allowed:         allowed,
		RemainingBudget: remaining,
		UsedBudget:      currentSpend,
		TotalBudget:     maxBudget,
		Period:          string(budgetPeriod),
	}

	if !allowed {
		result.Message = fmt.Sprintf("Request would exceed %s budget limit. Remaining: $%.4f, Required: $%.4f",
			entityType, remaining, estimatedCost)
	}

	// Add reset date for period-based budgets
	resetDate := s.getNextResetDate(budgetPeriod)
	result.ResetDate = &resetDate

	return result, nil
}

// CheckBudgetCached performs fast budget check using Redis cache
func (s *UnifiedService) CheckBudgetCached(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	if s.budgetCache == nil {
		// Fallback to database check if cache not available
		return s.checkBudgetFromDB(ctx, entityType, entityID, requestCost)
	}

	// First check Redis cache for immediate response
	available, err := s.budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, requestCost)
	if err == nil {
		return available, nil
	}

	// Cache miss - check database and populate cache
	return s.checkBudgetFromDB(ctx, entityType, entityID, requestCost)
}

// RecordUsage records actual usage after a request completes
func (s *UnifiedService) RecordUsage(ctx context.Context, keyID uuid.UUID, cost float64, model string, inputTokens, outputTokens int) error {
	if s.usageQueue != nil {
		// Use async queue for high performance
		usageRecord := &redisService.UsageRecord{
			KeyID:        keyID.String(),
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalCost:    cost,
			Timestamp:    time.Now(),
		}
		return s.usageQueue.EnqueueUsage(ctx, usageRecord)
	}

	// Fallback to synchronous database update
	return s.recordUsageSync(ctx, keyID, cost, model, inputTokens, outputTokens)
}

// UpdateSpending updates budget spending for an entity
func (s *UnifiedService) UpdateSpending(ctx context.Context, entityType, entityID string, cost float64) error {
	// Parse entity ID
	id, err := uuid.Parse(entityID)
	if err != nil {
		return fmt.Errorf("invalid entity ID: %w", err)
	}

	// Update spending based on entity type
	switch entityType {
	case "team":
		return s.db.WithContext(ctx).Model(&models.Team{}).
			Where("id = ?", id).
			Update("current_spend", gorm.Expr("current_spend + ?", cost)).Error
	case "key":
		return s.db.WithContext(ctx).Model(&models.Key{}).
			Where("id = ?", id).
			Update("current_spend", gorm.Expr("current_spend + ?", cost)).Error
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// checkBudgetFromDB checks budget from database and updates cache
func (s *UnifiedService) checkBudgetFromDB(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	id, err := uuid.Parse(entityID)
	if err != nil {
		return false, fmt.Errorf("invalid entity ID: %w", err)
	}

	var maxBudget, currentSpend float64

	switch entityType {
	case "team":
		var team models.Team
		if err := s.db.WithContext(ctx).First(&team, id).Error; err != nil {
			return false, fmt.Errorf("team not found: %w", err)
		}
		maxBudget = team.MaxBudget
		currentSpend = team.CurrentSpend
	case "key":
		var key models.Key
		if err := s.db.WithContext(ctx).First(&key, id).Error; err != nil {
			return false, fmt.Errorf("key not found: %w", err)
		}
		if key.MaxBudget == nil {
			return true, nil // No budget limit
		}
		maxBudget = *key.MaxBudget
		currentSpend = key.CurrentSpend
	default:
		return false, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	available := (maxBudget - currentSpend) >= requestCost

	// Cache is automatically updated by CheckBudgetAvailable
	// when cache miss occurs

	return available, nil
}

// recordUsageSync records usage synchronously to database
func (s *UnifiedService) recordUsageSync(ctx context.Context, keyID uuid.UUID, cost float64, model string, inputTokens, outputTokens int) error {
	// Record usage in the usage table
	usage := &models.Usage{
		KeyID:        keyID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalCost:    cost,
		Timestamp:    time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(usage).Error; err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Update key spending
	if err := s.db.WithContext(ctx).Model(&models.Key{}).
		Where("id = ?", keyID).
		Update("current_spend", gorm.Expr("current_spend + ?", cost)).Error; err != nil {
		return fmt.Errorf("failed to update key spending: %w", err)
	}

	// Update team spending if key belongs to a team
	if err := s.db.WithContext(ctx).Model(&models.Team{}).
		Where("id = (SELECT team_id FROM keys WHERE id = ?)", keyID).
		Update("current_spend", gorm.Expr("current_spend + ?", cost)).Error; err != nil {
		s.logger.Warn("Failed to update team spending", zap.Error(err))
	}

	return nil
}

// getNextResetDate calculates the next budget reset date based on period
func (s *UnifiedService) getNextResetDate(period models.BudgetPeriod) time.Time {
	now := time.Now()
	
	switch period {
	case models.BudgetPeriodDaily:
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case models.BudgetPeriodWeekly:
		// Next Monday
		daysUntilMonday := (7 - int(now.Weekday()) + 1) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		return now.AddDate(0, 0, daysUntilMonday).Truncate(24 * time.Hour)
	case models.BudgetPeriodMonthly:
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	case models.BudgetPeriodYearly:
		return time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, now.Location())
	default:
		return now.AddDate(0, 1, 0) // Default to monthly
	}
}