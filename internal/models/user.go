package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	BaseModel
	Email         string     `gorm:"uniqueIndex;not null" json:"email"`
	Username      string     `gorm:"uniqueIndex;not null" json:"username"`
	DexID         string     `gorm:"uniqueIndex" json:"-"` // Dex subject ID - nullable for existing users
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	Role          UserRole   `gorm:"type:varchar(20);default:'user'" json:"role"`
	IsActive      bool       `gorm:"default:true" json:"is_active"`
	EmailVerified bool       `gorm:"default:false" json:"email_verified"`
	LastLoginAt   *time.Time `json:"last_login_at"`

	// Budget Control (user-level)
	MaxBudget      float64      `json:"max_budget"`
	BudgetDuration BudgetPeriod `json:"budget_duration"`
	CurrentSpend   float64      `json:"current_spend"`
	BudgetResetAt  time.Time    `json:"budget_reset_at"`

	// Rate Limiting (user-level)
	TPM              int `json:"tpm"`
	RPM              int `json:"rpm"`
	MaxParallelCalls int `json:"max_parallel_calls"`

	// Model Access Control
	AllowedModels []string `gorm:"type:text[]" json:"allowed_models,omitempty"`
	BlockedModels []string `gorm:"type:text[]" json:"blocked_models,omitempty"`

	// OAuth/External identity
	ExternalID       string     `gorm:"index" json:"external_id,omitempty"`
	ExternalProvider string     `json:"external_provider,omitempty"` // github, google, microsoft, local
	ExternalGroups   []string   `gorm:"type:text[]" json:"external_groups,omitempty"`
	ProvisionedAt    *time.Time `json:"provisioned_at,omitempty"`
	AvatarURL        string     `json:"avatar_url,omitempty"`

	// Relationships
	Teams  []TeamMember `gorm:"foreignKey:UserID" json:"teams,omitempty"`
	Keys   []Key        `gorm:"foreignKey:UserID" json:"keys,omitempty"`
	Usage  []Usage      `gorm:"foreignKey:UserID" json:"-"`
	Audits []Audit      `gorm:"foreignKey:UserID" json:"-"`
}

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleUser    UserRole = "user"
	RoleViewer  UserRole = "viewer"
)

// UserProvisionRequest represents a request to auto-provision a user from Dex
type UserProvisionRequest struct {
	Email            string   `json:"email"`
	Username         string   `json:"username"`
	FirstName        string   `json:"first_name"`
	LastName         string   `json:"last_name"`
	DexID            string   `json:"dex_id"` // Dex subject ID
	ExternalProvider string   `json:"external_provider"`
	ExternalGroups   []string `json:"external_groups"`
	Role             UserRole `json:"role"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if err := u.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	// Set default budget reset time if budget is configured
	if u.MaxBudget > 0 && u.BudgetResetAt.IsZero() {
		u.ResetBudget()
	}

	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	return nil
}

func (u *User) IsBudgetExceeded() bool {
	return u.MaxBudget > 0 && u.CurrentSpend >= u.MaxBudget
}

func (u *User) ShouldResetBudget() bool {
	return time.Now().After(u.BudgetResetAt)
}

func (u *User) ResetBudget() {
	u.CurrentSpend = 0

	now := time.Now()
	switch u.BudgetDuration {
	case BudgetPeriodDaily:
		u.BudgetResetAt = now.AddDate(0, 0, 1)
	case BudgetPeriodWeekly:
		u.BudgetResetAt = now.AddDate(0, 0, 7)
	case BudgetPeriodMonthly:
		u.BudgetResetAt = now.AddDate(0, 1, 0)
	case BudgetPeriodYearly:
		u.BudgetResetAt = now.AddDate(1, 0, 0)
	default:
		u.BudgetResetAt = now.AddDate(0, 0, 30)
	}
}

func (u *User) IsModelAllowed(model string) bool {
	// Check if model is blocked
	for _, blocked := range u.BlockedModels {
		if blocked == model || blocked == "*" {
			return false
		}
	}

	// If no allowed models specified, allow all (except blocked)
	if len(u.AllowedModels) == 0 {
		return true
	}

	// Check if model is in allowed list
	for _, allowed := range u.AllowedModels {
		if allowed == model || allowed == "*" {
			return true
		}
	}

	return false
}

// IsProvisioned checks if the user was auto-provisioned from external OAuth
func (u *User) IsProvisioned() bool {
	return u.ProvisionedAt != nil && u.ExternalProvider != ""
}

// MarkAsProvisioned marks the user as auto-provisioned from Dex
func (u *User) MarkAsProvisioned(provider string, dexID string, groups []string) {
	now := time.Now()
	u.ProvisionedAt = &now
	u.ExternalProvider = provider
	u.ExternalID = dexID // For backward compatibility, store Dex ID here too
	u.DexID = dexID
	u.ExternalGroups = groups
	u.EmailVerified = true // Auto-verify for Dex users
}

// UpdateExternalGroups updates the user's external groups from OAuth
func (u *User) UpdateExternalGroups(groups []string) {
	u.ExternalGroups = groups
	now := time.Now()
	u.LastLoginAt = &now
}
