package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// GroupRole represents the role of a user in a group
type GroupRole string

const (
	GroupRoleOwner  GroupRole = "owner"
	GroupRoleAdmin  GroupRole = "admin"
	GroupRoleMember GroupRole = "member"
	GroupRoleViewer GroupRole = "viewer"
)

type Group struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	
	// Access Control
	AllowedProviders []string `gorm:"type:text[]" json:"allowed_providers"`
	AllowedModels    []string `gorm:"type:text[]" json:"allowed_models"`
	
	// Rate Limiting (per minute)
	RateLimit        int `gorm:"default:60" json:"rate_limit"`
	BurstLimit       int `gorm:"default:10" json:"burst_limit"`
	ConcurrentLimit  int `gorm:"default:5" json:"concurrent_limit"`
	
	// Budget Control
	MonthlyBudget    float64 `gorm:"default:0" json:"monthly_budget"`
	DailyBudget      float64 `gorm:"default:0" json:"daily_budget"`
	CurrentSpend     float64 `gorm:"default:0" json:"current_spend"`
	BudgetAlertAt    float64 `gorm:"default:80" json:"budget_alert_at"`
	
	// Configuration
	Settings datatypes.JSON `json:"settings,omitempty"`
	
	// Relationships
	Users   []User   `gorm:"many2many:user_groups;" json:"users,omitempty"`
	APIKeys []APIKey `gorm:"foreignKey:GroupID" json:"-"`
	Budgets []Budget `gorm:"foreignKey:GroupID" json:"-"`
	
	// Metadata
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
}

type GroupSettings struct {
	MaxTokensPerRequest  int     `json:"max_tokens_per_request"`
	MaxRequestsPerDay    int     `json:"max_requests_per_day"`
	CostMultiplier       float64 `json:"cost_multiplier"`
	EnableCaching        bool    `json:"enable_caching"`
	CacheTTL             int     `json:"cache_ttl"`
	EnableLogging        bool    `json:"enable_logging"`
	LogLevel             string  `json:"log_level"`
	WebhookURL           string  `json:"webhook_url"`
	NotificationEmails   []string `json:"notification_emails"`
}

func (g *Group) IsProviderAllowed(provider string) bool {
	if len(g.AllowedProviders) == 0 {
		return true
	}
	
	for _, p := range g.AllowedProviders {
		if p == provider || p == "*" {
			return true
		}
	}
	
	return false
}

func (g *Group) IsModelAllowed(model string) bool {
	if len(g.AllowedModels) == 0 {
		return true
	}
	
	for _, m := range g.AllowedModels {
		if m == model || m == "*" {
			return true
		}
	}
	
	return false
}

func (g *Group) IsBudgetExceeded() bool {
	if g.MonthlyBudget > 0 && g.CurrentSpend >= g.MonthlyBudget {
		return true
	}
	
	if g.DailyBudget > 0 {
		// TODO: Check daily spend
	}
	
	return false
}

func (g *Group) ShouldAlertBudget() bool {
	if g.MonthlyBudget <= 0 {
		return false
	}
	
	percentUsed := (g.CurrentSpend / g.MonthlyBudget) * 100
	return percentUsed >= g.BudgetAlertAt
}

type GroupMember struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      GroupRole `json:"role"`
	JoinedAt  string    `json:"joined_at"`
}