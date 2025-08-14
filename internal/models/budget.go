package models

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	BaseModel
	Name        string     `gorm:"not null" json:"name"`
	Type        BudgetType `gorm:"not null" json:"type"`
	Amount      float64    `gorm:"not null" json:"amount"`
	Spent       float64    `gorm:"default:0" json:"spent"`
	Period      BudgetPeriod `gorm:"not null" json:"period"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	AlertAt     float64    `gorm:"default:80" json:"alert_at"`
	AlertSent   bool       `gorm:"default:false" json:"alert_sent"`
	
	// Relationships
	UserID  *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	User    *User      `gorm:"foreignKey:UserID" json:"-"`
	GroupID *uuid.UUID `gorm:"type:uuid" json:"group_id,omitempty"`
	Group   *Group     `gorm:"foreignKey:GroupID" json:"-"`
	
	// Actions
	Actions []BudgetAction `gorm:"type:jsonb" json:"actions,omitempty"`
	
	// Metadata
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
}

type BudgetType string

const (
	BudgetTypeUser  BudgetType = "user"
	BudgetTypeGroup BudgetType = "group"
	BudgetTypeGlobal BudgetType = "global"
)

type BudgetPeriod string

const (
	BudgetPeriodDaily   BudgetPeriod = "daily"
	BudgetPeriodWeekly  BudgetPeriod = "weekly"
	BudgetPeriodMonthly BudgetPeriod = "monthly"
	BudgetPeriodYearly  BudgetPeriod = "yearly"
	BudgetPeriodCustom  BudgetPeriod = "custom"
)

type BudgetAction struct {
	Threshold   float64 `json:"threshold"`
	Action      string  `json:"action"`
	Executed    bool    `json:"executed"`
	ExecutedAt  *time.Time `json:"executed_at,omitempty"`
}

func (b *Budget) GetRemainingBudget() float64 {
	return b.Amount - b.Spent
}

func (b *Budget) GetUsagePercentage() float64 {
	if b.Amount == 0 {
		return 0
	}
	return (b.Spent / b.Amount) * 100
}

func (b *Budget) IsExceeded() bool {
	return b.Spent >= b.Amount
}

func (b *Budget) ShouldAlert() bool {
	if b.AlertSent {
		return false
	}
	return b.GetUsagePercentage() >= b.AlertAt
}

func (b *Budget) IsExpired() bool {
	return time.Now().After(b.EndsAt)
}

func (b *Budget) Reset() {
	b.Spent = 0
	b.AlertSent = false
	
	// Reset action executions
	for i := range b.Actions {
		b.Actions[i].Executed = false
		b.Actions[i].ExecutedAt = nil
	}
	
	// Update period dates
	now := time.Now()
	switch b.Period {
	case BudgetPeriodDaily:
		b.StartsAt = now
		b.EndsAt = now.AddDate(0, 0, 1)
	case BudgetPeriodWeekly:
		b.StartsAt = now
		b.EndsAt = now.AddDate(0, 0, 7)
	case BudgetPeriodMonthly:
		b.StartsAt = now
		b.EndsAt = now.AddDate(0, 1, 0)
	case BudgetPeriodYearly:
		b.StartsAt = now
		b.EndsAt = now.AddDate(1, 0, 0)
	}
}

// BudgetTracking represents detailed tracking of budget spending
type BudgetTracking struct {
	BaseModel
	UserID   *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	TeamID   *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`
	KeyID    *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`
	Model    string     `gorm:"index" json:"model"`
	Provider string     `gorm:"index" json:"provider"`
	Tokens   int        `json:"tokens"`
	Cost     float64    `gorm:"index" json:"cost"`
	
	// Request metadata
	RequestID string                   `gorm:"index" json:"request_id,omitempty"`
	Metadata  map[string]interface{}   `gorm:"type:jsonb" json:"metadata,omitempty"`
	
	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"-"`
	Team *Team `gorm:"foreignKey:TeamID" json:"-"`
	Key  *Key  `gorm:"foreignKey:KeyID" json:"-"`
}

// BudgetAlert represents budget alert notifications
type BudgetAlert struct {
	BaseModel
	Type       string     `gorm:"not null;index" json:"type"`
	UserID     *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	TeamID     *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`
	KeyID      *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`
	Threshold  float64    `json:"threshold"`
	CurrentPct float64    `json:"current_pct"`
	Message    string     `json:"message"`
	SentAt     time.Time  `json:"sent_at"`
	
	// Alert delivery status
	WebhookSent bool `gorm:"default:false" json:"webhook_sent"`
	EmailSent   bool `gorm:"default:false" json:"email_sent"`
	
	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"-"`
	Team *Team `gorm:"foreignKey:TeamID" json:"-"`
	Key  *Key  `gorm:"foreignKey:KeyID" json:"-"`
}