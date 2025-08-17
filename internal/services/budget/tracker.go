package budget

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

type Tracker struct {
	db *gorm.DB
}

func NewTracker(db *gorm.DB) *Tracker {
	return &Tracker{db: db}
}

// BudgetCheck represents the result of a budget validation
type BudgetCheck struct {
	Allowed        bool    `json:"allowed"`
	RemainingBudget float64 `json:"remaining_budget"`
	UsedBudget     float64 `json:"used_budget"`
	TotalBudget    float64 `json:"total_budget"`
	Period         string  `json:"period"`
	ResetDate      *time.Time `json:"reset_date,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// CheckBudget validates if a request can be made within budget limits
func (t *Tracker) CheckBudget(ctx context.Context, keyID uuid.UUID, estimatedCost float64) (*BudgetCheck, error) {
	// Get the key and its associated user/team
	var key models.Key
	if err := t.db.WithContext(ctx).Preload("User").Preload("Team").First(&key, keyID).Error; err != nil {
		return nil, fmt.Errorf("failed to find key: %w", err)
	}

	// Determine budget to check (team takes precedence)
	var maxBudget float64
	var currentSpend float64
	var budgetPeriod models.BudgetPeriod
	var entityType string

	if key.TeamID != nil {
		maxBudget = key.Team.MaxBudget
		currentSpend = key.Team.CurrentSpend
		budgetPeriod = key.Team.BudgetDuration
		entityType = "team"
	} else if key.UserID != nil {
		// For users, we need to look up their budget or use defaults
		maxBudget = 100.0 // Default user budget
		currentSpend = 0.0 // TODO: Track user spend
		budgetPeriod = models.BudgetPeriodMonthly
		entityType = "user"
	} else {
		return &BudgetCheck{
			Allowed: true,
			Message: "No budget limits for master keys",
		}, nil
	}

	// If no budget limit set, allow the request
	if maxBudget <= 0 {
		return &BudgetCheck{
			Allowed: true,
			Message: "No budget limit configured",
		}, nil
	}

	// Calculate remaining budget
	remaining := maxBudget - currentSpend
	
	// Check if the estimated cost would exceed the budget
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
	resetDate := t.getNextResetDate(budgetPeriod)
	result.ResetDate = &resetDate

	return result, nil
}

// RecordUsage records actual usage after a request completes
func (t *Tracker) RecordUsage(ctx context.Context, keyID uuid.UUID, cost float64, model string, 
	inputTokens, outputTokens int) error {
	
	var key models.Key
	if err := t.db.WithContext(ctx).First(&key, keyID).Error; err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	// Create usage record using existing Usage model
	usage := models.Usage{
		BaseModel:    models.BaseModel{ID: uuid.New(), CreatedAt: time.Now()},
		RequestID:    uuid.New().String(), // Generate unique request ID
		Timestamp:    time.Now(),
		KeyID:        keyID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		TotalCost:    cost,
		InputCost:    cost * 0.7,  // Approximate split
		OutputCost:   cost * 0.3,  // Approximate split
	}

	if key.UserID != nil {
		usage.UserID = *key.UserID
	}
	if key.TeamID != nil {
		usage.GroupID = key.TeamID // Usage model uses GroupID for teams
	}

	if err := t.db.WithContext(ctx).Create(&usage).Error; err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Update team/user current spend
	if key.TeamID != nil {
		t.db.Model(&models.Team{}).Where("id = ?", *key.TeamID).
			UpdateColumn("current_spend", gorm.Expr("current_spend + ?", cost))
	}

	// Check if we need to send alerts
	go t.checkBudgetAlerts(keyID, cost)

	return nil
}

// getCurrentUsage calculates usage for the current budget period
func (t *Tracker) getCurrentUsage(ctx context.Context, entityID uuid.UUID, entityType string, period models.BudgetPeriod) (float64, error) {
	var totalCost float64
	
	query := t.db.WithContext(ctx).Model(&models.Usage{})
	
	// Filter by entity type
	switch entityType {
	case "user":
		query = query.Where("user_id = ?", entityID)
	case "team":
		query = query.Where("group_id = ?", entityID) // Usage model uses group_id for teams
	default:
		return 0, fmt.Errorf("invalid entity type: %s", entityType)
	}

	// Filter by time period
	now := time.Now()
	switch period {
	case models.BudgetPeriodDaily:
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		query = query.Where("timestamp >= ?", startOfDay)
	case models.BudgetPeriodWeekly:
		startOfWeek := now.AddDate(0, 0, -int(now.Weekday()))
		startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())
		query = query.Where("timestamp >= ?", startOfWeek)
	case models.BudgetPeriodMonthly:
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		query = query.Where("timestamp >= ?", startOfMonth)
	}

	err := query.Select("COALESCE(SUM(total_cost), 0)").Scan(&totalCost).Error
	return totalCost, err
}

// getNextResetDate calculates when the budget period resets
func (t *Tracker) getNextResetDate(period models.BudgetPeriod) time.Time {
	now := time.Now()
	
	switch period {
	case models.BudgetPeriodDaily:
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case models.BudgetPeriodWeekly:
		daysUntilSunday := 7 - int(now.Weekday())
		nextSunday := now.AddDate(0, 0, daysUntilSunday)
		return time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 0, 0, 0, 0, nextSunday.Location())
	case models.BudgetPeriodMonthly:
		nextMonth := now.AddDate(0, 1, 0)
		return time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, nextMonth.Location())
	default:
		return time.Time{}
	}
}

// checkBudgetAlerts sends alerts when budget thresholds are reached
func (t *Tracker) checkBudgetAlerts(keyID uuid.UUID, newCost float64) {
	// This would integrate with a notification system
	// For now, we'll just log the alert conditions
	
	ctx := context.Background()
	
	var key models.Key
	if err := t.db.WithContext(ctx).Preload("User").Preload("Team").First(&key, keyID).Error; err != nil {
		return
	}

	// Simple budget checking using existing team model
	if key.TeamID != nil && key.Team != nil {
		if key.Team.ShouldAlertBudget() {
			// Could create a BudgetAlert record here
			// For now, just log that an alert should be sent
		}
	}
}

// ResetAlertFlags resets alert flags at the start of a new budget period
func (t *Tracker) ResetAlertFlags(ctx context.Context) error {
	// This should be called by a periodic job to reset team budgets that are due
	now := time.Now()
	
	// Find teams that need budget reset
	var teams []models.Team
	if err := t.db.WithContext(ctx).Where("budget_reset_at <= ?", now).Find(&teams).Error; err != nil {
		return err
	}
	
	for _, team := range teams {
		team.ResetBudget()
		if err := t.db.WithContext(ctx).Save(&team).Error; err != nil {
			return err
		}
	}
	
	return nil
}

// GetUsageStats returns detailed usage statistics
func (t *Tracker) GetUsageStats(ctx context.Context, entityID uuid.UUID, entityType string, period models.BudgetPeriod) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Get total usage for period
	totalUsage, err := t.getCurrentUsage(ctx, entityID, entityType, period)
	if err != nil {
		return nil, err
	}
	stats["total_usage"] = totalUsage
	
	// Get usage breakdown by model
	var modelUsage []struct {
		Model string  `json:"model"`
		Cost  float64 `json:"cost"`
		Count int64   `json:"count"`
	}
	
	query := t.db.WithContext(ctx).Model(&models.Usage{})
	
	switch entityType {
	case "user":
		query = query.Where("user_id = ?", entityID)
	case "team":
		query = query.Where("group_id = ?", entityID) // Usage model uses group_id for teams
	}
	
	// Apply time filtering
	now := time.Now()
	switch period {
	case models.BudgetPeriodDaily:
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		query = query.Where("timestamp >= ?", startOfDay)
	case models.BudgetPeriodWeekly:
		startOfWeek := now.AddDate(0, 0, -int(now.Weekday()))
		startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())
		query = query.Where("timestamp >= ?", startOfWeek)
	case models.BudgetPeriodMonthly:
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		query = query.Where("timestamp >= ?", startOfMonth)
	}
	
	err = query.Select("model, SUM(total_cost) as cost, COUNT(*) as count").
		Group("model").
		Scan(&modelUsage).Error
	if err != nil {
		return nil, err
	}
	
	stats["model_breakdown"] = modelUsage
	
	return stats, nil
}