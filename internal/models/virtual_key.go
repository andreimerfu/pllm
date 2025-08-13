package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// VirtualKey represents a database-backed API key with granular controls
type VirtualKey struct {
	BaseModel
	Key         string     `gorm:"uniqueIndex;not null" json:"key"`
	Name        string     `json:"name"`
	
	// Ownership (can belong to user OR team)
	UserID      *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	User        *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	TeamID      *uuid.UUID `gorm:"type:uuid" json:"team_id,omitempty"`
	Team        *Team      `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	
	// Status
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	
	// Budget Control
	MaxBudget       *float64      `json:"max_budget,omitempty"`
	BudgetDuration  *BudgetPeriod `json:"budget_duration,omitempty"`
	CurrentSpend    float64       `json:"current_spend"`
	BudgetResetAt   *time.Time    `json:"budget_reset_at,omitempty"`
	
	// Rate Limiting (overrides team/user defaults)
	TPM              *int `json:"tpm,omitempty"`
	RPM              *int `json:"rpm,omitempty"`
	MaxParallelCalls *int `json:"max_parallel_calls,omitempty"`
	
	// Model Access Control
	AllowedModels    []string `gorm:"type:text[]" json:"allowed_models,omitempty"`
	BlockedModels    []string `gorm:"type:text[]" json:"blocked_models,omitempty"`
	
	// Usage Tracking
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	UsageCount   int64      `json:"usage_count"`
	TotalTokens  int64      `json:"total_tokens"`
	
	// Metadata
	Metadata     datatypes.JSON `json:"metadata,omitempty"`
	Tags         []string       `gorm:"type:text[]" json:"tags,omitempty"`
	
	// Audit
	CreatedBy    *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    *uuid.UUID `gorm:"type:uuid" json:"revoked_by,omitempty"`
	RevocationReason string     `json:"revocation_reason,omitempty"`
}

// KeyMetadata stores additional information about the key
type KeyMetadata struct {
	Environment  string                 `json:"environment,omitempty"`
	Application  string                 `json:"application,omitempty"`
	Version      string                 `json:"version,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}

// GenerateVirtualKey creates a new virtual key with "sk-" prefix
func GenerateVirtualKey() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("sk-%s", hex.EncodeToString(b))
}

// GenerateCustomVirtualKey creates a key with custom prefix
func GenerateCustomVirtualKey(prefix string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

// ValidateVirtualKey checks if a key has the correct format
func ValidateVirtualKey(key string) bool {
	return strings.HasPrefix(key, "sk-") && len(key) == 51
}

func (vk *VirtualKey) IsExpired() bool {
	if vk.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*vk.ExpiresAt)
}

func (vk *VirtualKey) IsRevoked() bool {
	return vk.RevokedAt != nil
}

func (vk *VirtualKey) CanUse() bool {
	return vk.IsActive && !vk.IsExpired() && !vk.IsRevoked()
}

func (vk *VirtualKey) IsBudgetExceeded() bool {
	if vk.MaxBudget == nil || *vk.MaxBudget <= 0 {
		return false
	}
	return vk.CurrentSpend >= *vk.MaxBudget
}

func (vk *VirtualKey) ShouldResetBudget() bool {
	if vk.BudgetResetAt == nil {
		return false
	}
	return time.Now().After(*vk.BudgetResetAt)
}

func (vk *VirtualKey) ResetBudget() {
	vk.CurrentSpend = 0
	
	if vk.BudgetDuration == nil {
		return
	}
	
	now := time.Now()
	switch *vk.BudgetDuration {
	case BudgetPeriodDaily:
		resetAt := now.AddDate(0, 0, 1)
		vk.BudgetResetAt = &resetAt
	case BudgetPeriodWeekly:
		resetAt := now.AddDate(0, 0, 7)
		vk.BudgetResetAt = &resetAt
	case BudgetPeriodMonthly:
		resetAt := now.AddDate(0, 1, 0)
		vk.BudgetResetAt = &resetAt
	case BudgetPeriodYearly:
		resetAt := now.AddDate(1, 0, 0)
		vk.BudgetResetAt = &resetAt
	}
}

func (vk *VirtualKey) IsModelAllowed(model string) bool {
	// Check if model is blocked
	for _, blocked := range vk.BlockedModels {
		if blocked == model || blocked == "*" {
			return false
		}
	}
	
	// If no allowed models specified, allow all (except blocked)
	if len(vk.AllowedModels) == 0 {
		return true
	}
	
	// Check if model is in allowed list
	for _, allowed := range vk.AllowedModels {
		if allowed == model || allowed == "*" {
			return true
		}
	}
	
	return false
}

func (vk *VirtualKey) RecordUsage(tokens int, cost float64) {
	now := time.Now()
	vk.LastUsedAt = &now
	vk.UsageCount++
	vk.TotalTokens += int64(tokens)
	vk.CurrentSpend += cost
}

func (vk *VirtualKey) Revoke(userID uuid.UUID, reason string) {
	now := time.Now()
	vk.RevokedAt = &now
	vk.RevokedBy = &userID
	vk.RevocationReason = reason
	vk.IsActive = false
}

// GetEffectiveRateLimits returns the rate limits for this key
func (vk *VirtualKey) GetEffectiveRateLimits(defaultTPM, defaultRPM, defaultParallel int) (tpm, rpm, parallel int) {
	tpm = defaultTPM
	rpm = defaultRPM
	parallel = defaultParallel
	
	if vk.TPM != nil {
		tpm = *vk.TPM
	}
	if vk.RPM != nil {
		rpm = *vk.RPM
	}
	if vk.MaxParallelCalls != nil {
		parallel = *vk.MaxParallelCalls
	}
	
	return
}

// VirtualKeyRequest represents a request to create a new virtual key
type VirtualKeyRequest struct {
	Name             string        `json:"name"`
	UserID           *uuid.UUID    `json:"user_id,omitempty"`
	TeamID           *uuid.UUID    `json:"team_id,omitempty"`
	Duration         *int          `json:"duration,omitempty"` // in seconds
	MaxBudget        *float64      `json:"max_budget,omitempty"`
	BudgetDuration   *BudgetPeriod `json:"budget_duration,omitempty"`
	TPM              *int          `json:"tpm,omitempty"`
	RPM              *int          `json:"rpm,omitempty"`
	MaxParallelCalls *int          `json:"max_parallel_calls,omitempty"`
	AllowedModels    []string      `json:"allowed_models,omitempty"`
	BlockedModels    []string      `json:"blocked_models,omitempty"`
	Metadata         interface{}   `json:"metadata,omitempty"`
	Tags             []string      `json:"tags,omitempty"`
}