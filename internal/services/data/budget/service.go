package budget

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service provides a unified interface for budget operations
// This consolidates multiple budget implementations into a single interface
type Service interface {
	// CheckBudget validates if a request can be made within budget limits
	CheckBudget(ctx context.Context, keyID uuid.UUID, estimatedCost float64) (*BudgetCheck, error)

	// CheckBudgetCached performs fast budget check using Redis cache
	CheckBudgetCached(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error)

	// RecordUsage records actual usage after a request completes
	RecordUsage(ctx context.Context, keyID uuid.UUID, cost float64, model string,
		inputTokens, outputTokens int) error

	// UpdateSpending updates budget spending for an entity
	UpdateSpending(ctx context.Context, entityType, entityID string, cost float64) error
}

// BudgetCheck represents the result of a budget validation
type BudgetCheck struct {
	Allowed         bool       `json:"allowed"`
	RemainingBudget float64    `json:"remaining_budget"`
	UsedBudget      float64    `json:"used_budget"`
	TotalBudget     float64    `json:"total_budget"`
	Period          string     `json:"period"`
	ResetDate       *time.Time `json:"reset_date,omitempty"`
	Message         string     `json:"message,omitempty"`
}
