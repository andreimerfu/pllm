package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	BaseModel
	Email           string     `gorm:"uniqueIndex;not null" json:"email"`
	Username        string     `gorm:"uniqueIndex;not null" json:"username"`
	Password        string     `gorm:"not null" json:"-"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Role            UserRole   `gorm:"type:varchar(20);default:'user'" json:"role"`
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	EmailVerified   bool       `gorm:"default:false" json:"email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	PasswordChangedAt *time.Time `json:"-"`
	
	// Budget Control (user-level)
	MaxBudget       float64       `json:"max_budget"`
	BudgetDuration  BudgetPeriod  `json:"budget_duration"`
	CurrentSpend    float64       `json:"current_spend"`
	BudgetResetAt   time.Time     `json:"budget_reset_at"`
	
	// Rate Limiting (user-level)
	TPM              int `json:"tpm"`
	RPM              int `json:"rpm"`
	MaxParallelCalls int `json:"max_parallel_calls"`
	
	// Model Access Control
	AllowedModels    []string `gorm:"type:text[]" json:"allowed_models,omitempty"`
	BlockedModels    []string `gorm:"type:text[]" json:"blocked_models,omitempty"`
	
	// Relationships
	Teams       []TeamMember `gorm:"foreignKey:UserID" json:"teams,omitempty"`
	VirtualKeys []VirtualKey `gorm:"foreignKey:UserID" json:"virtual_keys,omitempty"`
	Usage       []Usage      `gorm:"foreignKey:UserID" json:"-"`
}

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleManager   UserRole = "manager"
	RoleUser      UserRole = "user"
	RoleViewer    UserRole = "viewer"
)

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if err := u.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}
	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	
	now := time.Now()
	u.PasswordChangedAt = &now
	
	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("Password") {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			return err
		}
		u.Password = string(hashedPassword)
		
		now := time.Now()
		u.PasswordChangedAt = &now
	}
	return nil
}

func (u *User) ComparePassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
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