package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrInvalidKeyType = errors.New("invalid key type")
	ErrKeyNotFound    = errors.New("key not found")
)

// Key represents a unified API key model
type Key struct {
	BaseModel

	// Key identification
	Key       string  `gorm:"uniqueIndex;not null" json:"-"`
	KeyHash   string  `gorm:"uniqueIndex;not null" json:"-"`
	KeyPrefix string  `gorm:"index;not null" json:"key_prefix"`
	Name      string  `json:"name"`
	Type      KeyType `gorm:"type:varchar(20);default:'api'" json:"type"`

	// Ownership (can belong to user OR team)
	UserID *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	User   *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	TeamID *uuid.UUID `gorm:"type:uuid" json:"team_id,omitempty"`
	Team   *Team      `gorm:"foreignKey:TeamID" json:"team,omitempty"`

	// Status and lifecycle
	IsActive   bool       `gorm:"default:true" json:"is_active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// Budget Control
	MaxBudget      *float64      `json:"max_budget,omitempty"`
	BudgetDuration *BudgetPeriod `json:"budget_duration,omitempty"`
	CurrentSpend   float64       `json:"current_spend"`
	BudgetResetAt  *time.Time    `json:"budget_reset_at,omitempty"`

	// Rate Limiting (overrides team/user defaults)
	TPM              *int `json:"tpm,omitempty"`
	RPM              *int `json:"rpm,omitempty"`
	MaxParallelCalls *int `json:"max_parallel_calls,omitempty"`

	// Model Access Control
	AllowedModels pq.StringArray `gorm:"type:text[]" json:"allowed_models,omitempty"`
	BlockedModels pq.StringArray `gorm:"type:text[]" json:"blocked_models,omitempty"`

	// Usage Tracking
	UsageCount  int64   `json:"usage_count"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`

	// Permissions and scopes
	Scopes pq.StringArray `gorm:"type:text[]" json:"scopes,omitempty"`

	// Metadata
	Metadata datatypes.JSON `json:"metadata,omitempty"`
	Tags     pq.StringArray `gorm:"type:text[]" json:"tags,omitempty"`

	// Audit
	CreatedBy        *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevokedBy        *uuid.UUID `gorm:"type:uuid" json:"revoked_by,omitempty"`
	RevocationReason string     `json:"revocation_reason,omitempty"`
}

type KeyType string

const (
	KeyTypeAPI     KeyType = "api"     // API key
	KeyTypeVirtual KeyType = "virtual" // Virtual key (OpenAI compatible)
	KeyTypeMaster  KeyType = "master"  // Master key for admin access
	KeyTypeSystem  KeyType = "system"  // System key for backend services
)

// KeyRequest represents a request to create a new key
type KeyRequest struct {
	Name             string        `json:"name"`
	Type             KeyType       `json:"type"`
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
	Scopes           []string      `json:"scopes,omitempty"`
	Metadata         interface{}   `json:"metadata,omitempty"`
	Tags             []string      `json:"tags,omitempty"`
}

// KeyResponse represents the response when creating a key
type KeyResponse struct {
	Key
	KeyValue string `json:"key,omitempty"` // Only returned on creation
}

// GenerateKey creates a new key with appropriate prefix based on type
func GenerateKey(keyType KeyType) (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	var key string
	switch keyType {
	case KeyTypeAPI:
		key = fmt.Sprintf("pllm_ak_%s", hex.EncodeToString(b))
	case KeyTypeVirtual:
		key = fmt.Sprintf("sk-%s", hex.EncodeToString(b[:24]))
	case KeyTypeMaster:
		key = fmt.Sprintf("pllm_mk_%s", hex.EncodeToString(b))
	case KeyTypeSystem:
		key = fmt.Sprintf("pllm_sk_%s", hex.EncodeToString(b))
	default:
		key = fmt.Sprintf("pllm_uk_%s", hex.EncodeToString(b))
	}

	// Generate hash for storage
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	return key, keyHash, nil
}

// ValidateKeyFormat checks if a key has the correct format
func ValidateKeyFormat(key string) bool {
	switch {
	case strings.HasPrefix(key, "pllm_ak_"):
		return len(key) == 72 // pllm_ak_ (8) + 64 hex chars
	case strings.HasPrefix(key, "sk-"):
		return len(key) == 51 // sk- (3) + 48 hex chars
	case strings.HasPrefix(key, "pllm_mk_"):
		return len(key) == 72 // pllm_mk_ (8) + 64 hex chars
	case strings.HasPrefix(key, "pllm_sk_"):
		return len(key) == 72 // pllm_sk_ (8) + 64 hex chars
	default:
		return false
	}
}

// HashKey generates a SHA256 hash of the key for storage
func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// BeforeCreate hook to set defaults and generate prefix
func (k *Key) BeforeCreate(tx *gorm.DB) error {
	if err := k.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	// Set key prefix from hash if not set
	if k.KeyPrefix == "" && k.KeyHash != "" {
		k.KeyPrefix = k.KeyHash[:8]
	}

	// Set default type if not specified
	if k.Type == "" {
		k.Type = KeyTypeAPI
	}

	return nil
}

// IsExpired checks if the key has expired
func (k *Key) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsRevoked checks if the key has been revoked
func (k *Key) IsRevoked() bool {
	return k.RevokedAt != nil
}

// CanUse checks if the key can be used (active, not expired, not revoked)
func (k *Key) CanUse() bool {
	return k.IsActive && !k.IsExpired() && !k.IsRevoked()
}

// IsBudgetExceeded checks if the key has exceeded its budget
func (k *Key) IsBudgetExceeded() bool {
	if k.MaxBudget == nil || *k.MaxBudget <= 0 {
		return false
	}
	return k.CurrentSpend >= *k.MaxBudget
}

// ShouldResetBudget checks if the budget should be reset
func (k *Key) ShouldResetBudget() bool {
	if k.BudgetResetAt == nil {
		return false
	}
	return time.Now().After(*k.BudgetResetAt)
}

// ResetBudget resets the current spend and budget reset time
func (k *Key) ResetBudget() {
	k.CurrentSpend = 0

	if k.BudgetDuration == nil {
		return
	}

	now := time.Now()
	switch *k.BudgetDuration {
	case BudgetPeriodDaily:
		resetAt := now.AddDate(0, 0, 1)
		k.BudgetResetAt = &resetAt
	case BudgetPeriodWeekly:
		resetAt := now.AddDate(0, 0, 7)
		k.BudgetResetAt = &resetAt
	case BudgetPeriodMonthly:
		resetAt := now.AddDate(0, 1, 0)
		k.BudgetResetAt = &resetAt
	case BudgetPeriodYearly:
		resetAt := now.AddDate(1, 0, 0)
		k.BudgetResetAt = &resetAt
	}
}

// IsModelAllowed checks if the key has access to a specific model
func (k *Key) IsModelAllowed(model string) bool {
	// Check if model is blocked
	for _, blocked := range k.BlockedModels {
		if blocked == model || blocked == "*" {
			return false
		}
	}

	// If no allowed models specified, allow all (except blocked)
	if len(k.AllowedModels) == 0 {
		return true
	}

	// Check if model is in allowed list
	for _, allowed := range k.AllowedModels {
		if allowed == model || allowed == "*" {
			return true
		}
	}

	return false
}

// HasScope checks if the key has a specific scope
func (k *Key) HasScope(scope string) bool {
	if len(k.Scopes) == 0 {
		return true // No scopes means all access
	}

	for _, s := range k.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}

	return false
}

// RecordUsage updates usage statistics for the key
func (k *Key) RecordUsage(tokens int, cost float64) {
	now := time.Now()
	k.LastUsedAt = &now
	k.UsageCount++
	k.TotalTokens += int64(tokens)
	k.TotalCost += cost
	k.CurrentSpend += cost
}

// Revoke revokes the key with audit information
func (k *Key) Revoke(userID uuid.UUID, reason string) {
	now := time.Now()
	k.RevokedAt = &now
	k.RevokedBy = &userID
	k.RevocationReason = reason
	k.IsActive = false
}

// GetEffectiveRateLimits returns the effective rate limits for this key
func (k *Key) GetEffectiveRateLimits(defaultTPM, defaultRPM, defaultParallel int) (tpm, rpm, parallel int) {
	tpm = defaultTPM
	rpm = defaultRPM
	parallel = defaultParallel

	if k.TPM != nil {
		tpm = *k.TPM
	}
	if k.RPM != nil {
		rpm = *k.RPM
	}
	if k.MaxParallelCalls != nil {
		parallel = *k.MaxParallelCalls
	}

	return
}

// GetType returns the key type, inferring from prefix if not set
func (k *Key) GetType() KeyType {
	if k.Type != "" {
		return k.Type
	}

	// Infer from key prefix (using new service generator format)
	switch {
	case strings.HasPrefix(k.Key, "sk-api"):
		return KeyTypeAPI
	case strings.HasPrefix(k.Key, "sk-vrt"):
		return KeyTypeVirtual
	case strings.HasPrefix(k.Key, "sk-mst"):
		return KeyTypeMaster
	case strings.HasPrefix(k.Key, "sk-sys"):
		return KeyTypeSystem
	// Legacy format support
	case strings.HasPrefix(k.Key, "sk-"):
		return KeyTypeVirtual
	case strings.HasPrefix(k.Key, "pllm_ak_"):
		return KeyTypeAPI
	case strings.HasPrefix(k.Key, "pllm_mk_"):
		return KeyTypeMaster
	case strings.HasPrefix(k.Key, "pllm_sk_"):
		return KeyTypeSystem
	default:
		return KeyTypeAPI
	}
}

// IsValid performs comprehensive validation of the key
func (k *Key) IsValid() bool {
	if !k.CanUse() {
		return false
	}

	if k.IsBudgetExceeded() {
		return false
	}

	return true
}
