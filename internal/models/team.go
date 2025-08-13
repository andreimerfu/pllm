package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Team represents an organizational unit with shared resources and permissions
type Team struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	
	// Budget Control
	MaxBudget       float64       `json:"max_budget"`
	BudgetDuration  BudgetPeriod  `json:"budget_duration"`
	CurrentSpend    float64       `json:"current_spend"`
	BudgetResetAt   time.Time     `json:"budget_reset_at"`
	BudgetAlertAt   float64       `gorm:"default:80" json:"budget_alert_at"`
	
	// Rate Limiting
	TPM              int `json:"tpm"`  // Tokens per minute
	RPM              int `json:"rpm"`  // Requests per minute
	MaxParallelCalls int `json:"max_parallel_calls"`
	
	// Model Access Control
	AllowedModels    []string `gorm:"type:text[]" json:"allowed_models"`
	BlockedModels    []string `gorm:"type:text[]" json:"blocked_models"`
	ModelAliases     datatypes.JSON `json:"model_aliases,omitempty"`
	
	// Configuration
	Settings datatypes.JSON `json:"settings,omitempty"`
	Metadata datatypes.JSON `json:"metadata,omitempty"`
	
	// Relationships
	Members []TeamMember `gorm:"foreignKey:TeamID" json:"members,omitempty"`
	Keys    []VirtualKey `gorm:"foreignKey:TeamID" json:"keys,omitempty"`
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID        uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TeamID    uuid.UUID     `gorm:"type:uuid;not null" json:"team_id"`
	Team      *Team         `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	UserID    uuid.UUID     `gorm:"type:uuid;not null" json:"user_id"`
	User      *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role      TeamRole      `gorm:"type:varchar(20);default:'member'" json:"role"`
	
	// Member-specific limits (optional, overrides team defaults)
	MaxBudget        *float64      `json:"max_budget,omitempty"`
	CurrentSpend     float64       `json:"current_spend"`
	CustomTPM        *int          `json:"custom_tpm,omitempty"`
	CustomRPM        *int          `json:"custom_rpm,omitempty"`
	
	JoinedAt  time.Time  `json:"joined_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type TeamRole string

const (
	TeamRoleOwner   TeamRole = "owner"
	TeamRoleAdmin   TeamRole = "admin"
	TeamRoleMember  TeamRole = "member"
	TeamRoleViewer  TeamRole = "viewer"
)

type TeamSettings struct {
	// Notification settings
	WebhookURL         string   `json:"webhook_url"`
	NotificationEmails []string `json:"notification_emails"`
	AlertOnBudget      bool     `json:"alert_on_budget"`
	AlertOnRateLimit   bool     `json:"alert_on_rate_limit"`
	
	// Advanced settings
	EnableCaching      bool     `json:"enable_caching"`
	CacheTTL           int      `json:"cache_ttl"`
	EnableLogging      bool     `json:"enable_logging"`
	LogLevel           string   `json:"log_level"`
	
	// Cost settings
	CostMultiplier     float64  `json:"cost_multiplier"`
	PriorityLevel      int      `json:"priority_level"`
}

func (t *Team) IsModelAllowed(model string) bool {
	// Check if model is blocked
	for _, blocked := range t.BlockedModels {
		if blocked == model || blocked == "*" {
			return false
		}
	}
	
	// If no allowed models specified, allow all (except blocked)
	if len(t.AllowedModels) == 0 {
		return true
	}
	
	// Check if model is in allowed list
	for _, allowed := range t.AllowedModels {
		if allowed == model || allowed == "*" {
			return true
		}
	}
	
	return false
}

func (t *Team) IsBudgetExceeded() bool {
	return t.MaxBudget > 0 && t.CurrentSpend >= t.MaxBudget
}

func (t *Team) ShouldAlertBudget() bool {
	if t.MaxBudget <= 0 {
		return false
	}
	
	percentUsed := (t.CurrentSpend / t.MaxBudget) * 100
	return percentUsed >= t.BudgetAlertAt
}

func (t *Team) ShouldResetBudget() bool {
	return time.Now().After(t.BudgetResetAt)
}

func (t *Team) ResetBudget() {
	t.CurrentSpend = 0
	
	now := time.Now()
	switch t.BudgetDuration {
	case BudgetPeriodDaily:
		t.BudgetResetAt = now.AddDate(0, 0, 1)
	case BudgetPeriodWeekly:
		t.BudgetResetAt = now.AddDate(0, 0, 7)
	case BudgetPeriodMonthly:
		t.BudgetResetAt = now.AddDate(0, 1, 0)
	case BudgetPeriodYearly:
		t.BudgetResetAt = now.AddDate(1, 0, 0)
	default:
		// For custom or unspecified, don't auto-reset
		t.BudgetResetAt = now.AddDate(0, 0, 30)
	}
}

func (tm *TeamMember) GetEffectiveTPM(teamTPM int) int {
	if tm.CustomTPM != nil {
		return *tm.CustomTPM
	}
	return teamTPM
}

func (tm *TeamMember) GetEffectiveRPM(teamRPM int) int {
	if tm.CustomRPM != nil {
		return *tm.CustomRPM
	}
	return teamRPM
}

func (tm *TeamMember) GetEffectiveBudget(teamBudget float64) float64 {
	if tm.MaxBudget != nil {
		return *tm.MaxBudget
	}
	return teamBudget
}

func (tm *TeamMember) IsBudgetExceeded(teamBudget float64) bool {
	effectiveBudget := tm.GetEffectiveBudget(teamBudget)
	return effectiveBudget > 0 && tm.CurrentSpend >= effectiveBudget
}