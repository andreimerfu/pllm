package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Usage struct {
	BaseModel
	RequestID string    `gorm:"uniqueIndex;not null" json:"request_id"`
	Timestamp time.Time `gorm:"index" json:"timestamp"`

	// User/Team/API Key - Enhanced for better tracking
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"` // Who made the request
	User         User       `gorm:"foreignKey:UserID" json:"-"`
	ActualUserID uuid.UUID  `gorm:"type:uuid;not null;index" json:"actual_user_id"` // Who actually used the key (same as UserID for personal keys)
	ActualUser   User       `gorm:"foreignKey:ActualUserID" json:"-"`
	TeamID       *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`
	Team         *Team      `gorm:"foreignKey:TeamID" json:"-"`
	KeyID        *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`
	Key          *Key       `gorm:"foreignKey:KeyID" json:"-"`
	KeyOwnerID   *uuid.UUID `gorm:"type:uuid;index" json:"key_owner_id,omitempty"` // Who owns the key (for user keys)

	// Provider/Model
	Provider string `gorm:"index" json:"provider"`
	Model    string `gorm:"index" json:"model"`

	// Request/Response
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	Latency    int64  `json:"latency"`

	// Tokens
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	// Cost
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`

	// Cache
	CacheHit bool   `json:"cache_hit"`
	CacheKey string `json:"cache_key,omitempty"`

	// Error
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`

	// Request/Response Data
	RequestBody  datatypes.JSON `json:"request_body,omitempty"`
	ResponseBody datatypes.JSON `json:"response_body,omitempty"`

	// Metadata
	Metadata datatypes.JSON `json:"metadata,omitempty"`
}

type UsageStats struct {
	Period        string  `json:"period"`
	TotalRequests int64   `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
	ErrorRate     float64 `json:"error_rate"`
	AvgLatency    float64 `json:"avg_latency"`

	// Breakdown
	ByProvider map[string]*ProviderStats `json:"by_provider,omitempty"`
	ByModel    map[string]*ModelStats    `json:"by_model,omitempty"`
	ByUser     map[string]*UserStats     `json:"by_user,omitempty"`
	ByTeam     map[string]*TeamStats     `json:"by_team,omitempty"`
}

type ProviderStats struct {
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	Cost       float64 `json:"cost"`
	ErrorRate  float64 `json:"error_rate"`
	AvgLatency float64 `json:"avg_latency"`
}

type ModelStats struct {
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	AvgLatency   float64 `json:"avg_latency"`
}

type UserStats struct {
	UserID       string  `json:"user_id"`
	UserEmail    string  `json:"user_email"`
	UserName     string  `json:"user_name"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	CacheHits    int64   `json:"cache_hits"`
	KeysUsed     int     `json:"keys_used"`     // Number of different keys used
	TeamRequests int64   `json:"team_requests"` // Requests made using team keys
	UserRequests int64   `json:"user_requests"` // Requests made using personal keys
}

type TeamStats struct {
	TeamID        string                `json:"team_id"`
	TeamName      string                `json:"team_name"`
	Requests      int64                 `json:"requests"`
	Tokens        int64                 `json:"tokens"`
	Cost          float64               `json:"cost"`
	BudgetUsed    float64               `json:"budget_used"`
	UserBreakdown map[string]*UserStats `json:"user_breakdown,omitempty"` // Per-user stats within team
	MemberCount   int                   `json:"member_count"`
	ActiveMembers int                   `json:"active_members"` // Members who made requests this period
}

// Indexes for performance
func (Usage) TableName() string {
	return "usage_logs"
}

type UsageAggregation struct {
	Date         time.Time  `json:"date"`
	UserID       uuid.UUID  `json:"user_id"`
	GroupID      *uuid.UUID `json:"group_id"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
	Requests     int64      `json:"requests"`
	InputTokens  int64      `json:"input_tokens"`
	OutputTokens int64      `json:"output_tokens"`
	TotalCost    float64    `json:"total_cost"`
	CacheHits    int64      `json:"cache_hits"`
	Errors       int64      `json:"errors"`
	AvgLatency   float64    `json:"avg_latency"`
}
