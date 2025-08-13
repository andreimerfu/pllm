package virtualkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	
	"github.com/amerfu/pllm/internal/models"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrKeyExpired       = errors.New("key expired")
	ErrKeyRevoked       = errors.New("key revoked")
	ErrKeyInactive      = errors.New("key inactive")
	ErrBudgetExceeded   = errors.New("budget exceeded")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrModelNotAllowed  = errors.New("model not allowed")
)

type VirtualKeyService struct {
	db *gorm.DB
}

func NewVirtualKeyService(db *gorm.DB) *VirtualKeyService {
	return &VirtualKeyService{db: db}
}

// CreateKey creates a new virtual key
func (s *VirtualKeyService) CreateKey(ctx context.Context, req *models.VirtualKeyRequest, createdBy uuid.UUID) (*models.VirtualKey, error) {
	key := &models.VirtualKey{
		Key:              models.GenerateVirtualKey(),
		Name:             req.Name,
		UserID:           req.UserID,
		TeamID:           req.TeamID,
		MaxBudget:        req.MaxBudget,
		BudgetDuration:   req.BudgetDuration,
		TPM:              req.TPM,
		RPM:              req.RPM,
		MaxParallelCalls: req.MaxParallelCalls,
		AllowedModels:    req.AllowedModels,
		BlockedModels:    req.BlockedModels,
		Tags:             req.Tags,
		IsActive:         true,
		CreatedBy:        &createdBy,
	}

	// Set expiration if duration is provided
	if req.Duration != nil && *req.Duration > 0 {
		expiresAt := time.Now().Add(time.Duration(*req.Duration) * time.Second)
		key.ExpiresAt = &expiresAt
	}

	// Set budget reset time if budget duration is provided
	if req.BudgetDuration != nil {
		now := time.Now()
		var resetAt time.Time
		switch *req.BudgetDuration {
		case models.BudgetPeriodDaily:
			resetAt = now.AddDate(0, 0, 1)
		case models.BudgetPeriodWeekly:
			resetAt = now.AddDate(0, 0, 7)
		case models.BudgetPeriodMonthly:
			resetAt = now.AddDate(0, 1, 0)
		case models.BudgetPeriodYearly:
			resetAt = now.AddDate(1, 0, 0)
		default:
			resetAt = now.AddDate(0, 0, 30)
		}
		key.BudgetResetAt = &resetAt
	}

	// Set metadata if provided
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		key.Metadata = metadataBytes
	}

	if err := s.db.Create(key).Error; err != nil {
		return nil, err
	}

	return key, nil
}

// ValidateKey validates a key and returns its details
func (s *VirtualKeyService) ValidateKey(ctx context.Context, keyString string) (*models.VirtualKey, error) {
	if !models.ValidateVirtualKey(keyString) {
		return nil, ErrKeyNotFound
	}

	var key models.VirtualKey
	err := s.db.Preload("User").Preload("Team").
		Where("key = ?", keyString).First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	// Check if key can be used
	if !key.CanUse() {
		if key.IsExpired() {
			return nil, ErrKeyExpired
		}
		if key.IsRevoked() {
			return nil, ErrKeyRevoked
		}
		return nil, ErrKeyInactive
	}

	// Check and reset budget if needed
	if key.ShouldResetBudget() {
		key.ResetBudget()
		s.db.Save(&key)
	}

	// Check budget
	if key.IsBudgetExceeded() {
		return nil, ErrBudgetExceeded
	}

	return &key, nil
}

// CheckModelAccess checks if a key has access to a specific model
func (s *VirtualKeyService) CheckModelAccess(ctx context.Context, key *models.VirtualKey, model string) error {
	// Check key-level access
	if !key.IsModelAllowed(model) {
		return ErrModelNotAllowed
	}

	// Check team-level access if key belongs to a team
	if key.TeamID != nil && key.Team != nil {
		if !key.Team.IsModelAllowed(model) {
			return ErrModelNotAllowed
		}
	}

	// Check user-level access if key belongs to a user
	if key.UserID != nil && key.User != nil {
		if !key.User.IsModelAllowed(model) {
			return ErrModelNotAllowed
		}
	}

	return nil
}

// RecordUsage records usage for a key and updates related budgets
func (s *VirtualKeyService) RecordUsage(ctx context.Context, keyID uuid.UUID, tokens int, cost float64) error {
	var key models.VirtualKey
	if err := s.db.First(&key, "id = ?", keyID).Error; err != nil {
		return err
	}

	// Record usage on the key
	key.RecordUsage(tokens, cost)

	// Use transaction to ensure consistency
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Update the key
		if err := tx.Save(&key).Error; err != nil {
			return err
		}

		// Update team spend if key belongs to a team
		if key.TeamID != nil {
			if err := tx.Model(&models.Team{}).Where("id = ?", key.TeamID).
				UpdateColumn("current_spend", gorm.Expr("current_spend + ?", cost)).Error; err != nil {
				return err
			}
		}

		// Update user spend if key belongs to a user
		if key.UserID != nil {
			if err := tx.Model(&models.User{}).Where("id = ?", key.UserID).
				UpdateColumn("current_spend", gorm.Expr("current_spend + ?", cost)).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// CheckBudgetLimits checks if the key can be used within budget constraints
func (s *VirtualKeyService) CheckBudgetLimits(ctx context.Context, key *models.VirtualKey, estimatedCost float64) error {
	// Check key-level budget
	if key.MaxBudget != nil && *key.MaxBudget > 0 {
		if key.CurrentSpend+estimatedCost > *key.MaxBudget {
			return ErrBudgetExceeded
		}
	}

	// Check team budget if key belongs to a team
	if key.TeamID != nil {
		var team models.Team
		if err := s.db.First(&team, "id = ?", key.TeamID).Error; err != nil {
			return err
		}
		
		if team.MaxBudget > 0 && team.CurrentSpend+estimatedCost > team.MaxBudget {
			return ErrBudgetExceeded
		}
	}

	// Check user budget if key belongs to a user
	if key.UserID != nil {
		var user models.User
		if err := s.db.First(&user, "id = ?", key.UserID).Error; err != nil {
			return err
		}
		
		if user.MaxBudget > 0 && user.CurrentSpend+estimatedCost > user.MaxBudget {
			return ErrBudgetExceeded
		}
	}

	return nil
}

// GetKey gets a key by ID
func (s *VirtualKeyService) GetKey(ctx context.Context, keyID uuid.UUID) (*models.VirtualKey, error) {
	var key models.VirtualKey
	err := s.db.Preload("User").Preload("Team").
		First(&key, "id = ?", keyID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// ListKeys lists keys with filters
func (s *VirtualKeyService) ListKeys(ctx context.Context, userID, teamID *uuid.UUID, limit, offset int) ([]*models.VirtualKey, int64, error) {
	query := s.db.Model(&models.VirtualKey{})

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if teamID != nil {
		query = query.Where("team_id = ?", *teamID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var keys []*models.VirtualKey
	err := query.Preload("User").Preload("Team").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&keys).Error
	if err != nil {
		return nil, 0, err
	}

	return keys, total, nil
}

// UpdateKey updates a key
func (s *VirtualKeyService) UpdateKey(ctx context.Context, keyID uuid.UUID, updates map[string]interface{}) (*models.VirtualKey, error) {
	var key models.VirtualKey
	if err := s.db.First(&key, "id = ?", keyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	if err := s.db.Model(&key).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &key, nil
}

// RevokeKey revokes a key
func (s *VirtualKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, revokedBy uuid.UUID, reason string) error {
	var key models.VirtualKey
	if err := s.db.First(&key, "id = ?", keyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKeyNotFound
		}
		return err
	}

	key.Revoke(revokedBy, reason)
	return s.db.Save(&key).Error
}

// DeleteKey deletes a key
func (s *VirtualKeyService) DeleteKey(ctx context.Context, keyID uuid.UUID) error {
	result := s.db.Delete(&models.VirtualKey{}, "id = ?", keyID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// GetKeyStats gets usage statistics for a key
func (s *VirtualKeyService) GetKeyStats(ctx context.Context, keyID uuid.UUID) (map[string]interface{}, error) {
	var key models.VirtualKey
	if err := s.db.First(&key, "id = ?", keyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	stats := map[string]interface{}{
		"key_id":          key.ID,
		"name":            key.Name,
		"usage_count":     key.UsageCount,
		"total_tokens":    key.TotalTokens,
		"current_spend":   key.CurrentSpend,
		"max_budget":      key.MaxBudget,
		"budget_remaining": 0.0,
		"is_active":       key.IsActive,
		"created_at":      key.CreatedAt,
		"last_used_at":    key.LastUsedAt,
	}

	if key.MaxBudget != nil && *key.MaxBudget > 0 {
		stats["budget_remaining"] = *key.MaxBudget - key.CurrentSpend
		stats["budget_percentage"] = (key.CurrentSpend / *key.MaxBudget) * 100
	}

	return stats, nil
}

// TemporaryBudgetIncrease temporarily increases the budget for a key
func (s *VirtualKeyService) TemporaryBudgetIncrease(ctx context.Context, keyID uuid.UUID, amount float64, duration time.Duration) error {
	var key models.VirtualKey
	if err := s.db.First(&key, "id = ?", keyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKeyNotFound
		}
		return err
	}

	// Store the original budget
	originalBudget := key.MaxBudget

	// Increase the budget
	if key.MaxBudget == nil {
		key.MaxBudget = &amount
	} else {
		newBudget := *key.MaxBudget + amount
		key.MaxBudget = &newBudget
	}

	if err := s.db.Save(&key).Error; err != nil {
		return err
	}

	// Schedule budget reset
	go func() {
		time.Sleep(duration)
		
		var k models.VirtualKey
		if err := s.db.First(&k, "id = ?", keyID).Error; err == nil {
			k.MaxBudget = originalBudget
			s.db.Save(&k)
		}
	}()

	return nil
}