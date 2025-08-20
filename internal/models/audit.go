package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Audit represents an audit log entry for security and compliance
type Audit struct {
	BaseModel

	// Event identification
	EventType   AuditEventType `gorm:"type:varchar(50);not null" json:"event_type"`
	EventAction string         `gorm:"not null" json:"event_action"`
	EventResult AuditResult    `gorm:"type:varchar(20);not null" json:"event_result"`

	// Context
	UserID *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	User   *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	TeamID *uuid.UUID `gorm:"type:uuid" json:"team_id,omitempty"`
	Team   *Team      `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	KeyID  *uuid.UUID `gorm:"type:uuid" json:"key_id,omitempty"`
	Key    *Key       `gorm:"foreignKey:KeyID" json:"key,omitempty"`

	// Request information
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	RequestID string `gorm:"index" json:"request_id"`
	Method    string `json:"method"`
	Path      string `json:"path"`

	// Authentication details
	AuthMethod   string `json:"auth_method,omitempty"`   // jwt, api_key, master_key, oauth
	AuthProvider string `json:"auth_provider,omitempty"` // dex, local, etc.

	// Event details
	ResourceType string         `json:"resource_type,omitempty"` // user, team, key, model, etc.
	ResourceID   *uuid.UUID     `gorm:"type:uuid" json:"resource_id,omitempty"`
	OldValues    datatypes.JSON `json:"old_values,omitempty"`
	NewValues    datatypes.JSON `json:"new_values,omitempty"`

	// Additional context
	Message   string         `json:"message,omitempty"`
	ErrorCode string         `json:"error_code,omitempty"`
	Metadata  datatypes.JSON `json:"metadata,omitempty"`

	// Timing
	Duration  *time.Duration `json:"duration,omitempty"`
	Timestamp time.Time      `gorm:"index;not null" json:"timestamp"`
}

type AuditEventType string

const (
	// Authentication events
	AuditEventAuth           AuditEventType = "auth"
	AuditEventLogin          AuditEventType = "login"
	AuditEventLogout         AuditEventType = "logout"
	AuditEventTokenRefresh   AuditEventType = "token_refresh"
	AuditEventPasswordChange AuditEventType = "password_change"

	// User management
	AuditEventUserCreate    AuditEventType = "user_create"
	AuditEventUserUpdate    AuditEventType = "user_update"
	AuditEventUserDelete    AuditEventType = "user_delete"
	AuditEventUserProvision AuditEventType = "user_provision"

	// Team management
	AuditEventTeamCreate AuditEventType = "team_create"
	AuditEventTeamUpdate AuditEventType = "team_update"
	AuditEventTeamDelete AuditEventType = "team_delete"
	AuditEventTeamJoin   AuditEventType = "team_join"
	AuditEventTeamLeave  AuditEventType = "team_leave"

	// Key management
	AuditEventKeyCreate AuditEventType = "key_create"
	AuditEventKeyUpdate AuditEventType = "key_update"
	AuditEventKeyRevoke AuditEventType = "key_revoke"
	AuditEventKeyUsage  AuditEventType = "key_usage"

	// Budget and billing
	AuditEventBudgetExceeded AuditEventType = "budget_exceeded"
	AuditEventBudgetAlert    AuditEventType = "budget_alert"
	AuditEventBudgetReset    AuditEventType = "budget_reset"

	// System events
	AuditEventSystemAccess  AuditEventType = "system_access"
	AuditEventConfigChange  AuditEventType = "config_change"
	AuditEventSecurityAlert AuditEventType = "security_alert"

	// API requests
	AuditEventAPIRequest   AuditEventType = "api_request"
	AuditEventRateLimit    AuditEventType = "rate_limit"
	AuditEventAccessDenied AuditEventType = "access_denied"
)

type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailure AuditResult = "failure"
	AuditResultError   AuditResult = "error"
	AuditResultWarning AuditResult = "warning"
)

// BeforeCreate sets the timestamp if not already set
func (a *Audit) BeforeCreate(tx *gorm.DB) error {
	if err := a.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}

	return nil
}

// AuditRequest represents a request to create an audit log entry
type AuditRequest struct {
	EventType    AuditEventType `json:"event_type"`
	EventAction  string         `json:"event_action"`
	EventResult  AuditResult    `json:"event_result"`
	UserID       *uuid.UUID     `json:"user_id,omitempty"`
	TeamID       *uuid.UUID     `json:"team_id,omitempty"`
	KeyID        *uuid.UUID     `json:"key_id,omitempty"`
	IPAddress    string         `json:"ip_address"`
	UserAgent    string         `json:"user_agent"`
	RequestID    string         `json:"request_id"`
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	AuthMethod   string         `json:"auth_method,omitempty"`
	AuthProvider string         `json:"auth_provider,omitempty"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID     `json:"resource_id,omitempty"`
	OldValues    interface{}    `json:"old_values,omitempty"`
	NewValues    interface{}    `json:"new_values,omitempty"`
	Message      string         `json:"message,omitempty"`
	ErrorCode    string         `json:"error_code,omitempty"`
	Metadata     interface{}    `json:"metadata,omitempty"`
	Duration     *time.Duration `json:"duration,omitempty"`
}

// AuditFilter represents filters for querying audit logs
type AuditFilter struct {
	EventTypes   []AuditEventType `json:"event_types,omitempty"`
	EventResults []AuditResult    `json:"event_results,omitempty"`
	UserID       *uuid.UUID       `json:"user_id,omitempty"`
	TeamID       *uuid.UUID       `json:"team_id,omitempty"`
	KeyID        *uuid.UUID       `json:"key_id,omitempty"`
	IPAddress    string           `json:"ip_address,omitempty"`
	ResourceType string           `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID       `json:"resource_id,omitempty"`
	StartTime    *time.Time       `json:"start_time,omitempty"`
	EndTime      *time.Time       `json:"end_time,omitempty"`
	Limit        int              `json:"limit,omitempty"`
	Offset       int              `json:"offset,omitempty"`
}

// GetSeverity returns the severity level of the audit event
func (a *Audit) GetSeverity() string {
	switch a.EventType {
	case AuditEventSecurityAlert, AuditEventAccessDenied:
		return "critical"
	case AuditEventBudgetExceeded, AuditEventKeyRevoke, AuditEventUserDelete:
		return "high"
	case AuditEventBudgetAlert, AuditEventPasswordChange, AuditEventConfigChange:
		return "medium"
	case AuditEventLogin, AuditEventKeyUsage, AuditEventAPIRequest:
		return "low"
	default:
		return "info"
	}
}

// IsSecurityEvent checks if this is a security-related audit event
func (a *Audit) IsSecurityEvent() bool {
	securityEvents := []AuditEventType{
		AuditEventAuth,
		AuditEventLogin,
		AuditEventLogout,
		AuditEventPasswordChange,
		AuditEventSecurityAlert,
		AuditEventAccessDenied,
		AuditEventKeyCreate,
		AuditEventKeyRevoke,
	}

	for _, event := range securityEvents {
		if a.EventType == event {
			return true
		}
	}

	return false
}

// ShouldAlert determines if this audit event should trigger an alert
func (a *Audit) ShouldAlert() bool {
	// Alert on failures for security events
	if a.IsSecurityEvent() && a.EventResult == AuditResultFailure {
		return true
	}

	// Alert on specific critical events
	criticalEvents := []AuditEventType{
		AuditEventSecurityAlert,
		AuditEventBudgetExceeded,
		AuditEventAccessDenied,
	}

	for _, event := range criticalEvents {
		if a.EventType == event {
			return true
		}
	}

	return false
}
