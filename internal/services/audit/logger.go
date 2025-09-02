package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

type Logger struct {
	db *gorm.DB
}

func NewLogger(db *gorm.DB) *Logger {
	return &Logger{db: db}
}

// AuditEvent represents the structure of audit log data
type AuditEvent struct {
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID *uuid.UUID             `json:"resource_id,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Method     string                 `json:"method,omitempty"`
	Path       string                 `json:"path,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
}

// LogEvent records an audit event
func (l *Logger) LogEvent(ctx context.Context, userID *uuid.UUID, teamID *uuid.UUID, event AuditEvent) error {
	// Serialize event details
	detailsJSON, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal event details: %w", err)
	}

	// Map our event to the existing Audit model structure
	auditLog := models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New(), CreatedAt: time.Now()},
		EventType:    l.mapActionToEventType(event.Action),
		EventAction:  event.Action,
		EventResult:  models.AuditResultSuccess,
		UserID:       userID,
		TeamID:       teamID,
		ResourceType: event.Resource,
		ResourceID:   event.ResourceID,
		IPAddress:    event.IPAddress,
		UserAgent:    event.UserAgent,
		Method:       event.Method,
		Path:         event.Path,
		Metadata:     detailsJSON,
		Timestamp:    time.Now(),
	}

	if event.StatusCode >= 400 {
		auditLog.EventResult = models.AuditResultFailure
	}

	if err := l.db.WithContext(ctx).Create(&auditLog).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// mapActionToEventType maps our action strings to existing audit event types
func (l *Logger) mapActionToEventType(action string) models.AuditEventType {
	switch action {
	case ActionCreate:
		return models.AuditEventTeamCreate // Default to team create for create actions
	case ActionUpdate:
		return models.AuditEventTeamUpdate
	case ActionDelete:
		return models.AuditEventTeamDelete
	case ActionLogin:
		return models.AuditEventLogin
	case ActionLogout:
		return models.AuditEventLogout
	case ActionAccess:
		return models.AuditEventAPIRequest
	default:
		return models.AuditEventSystemAccess
	}
}

// LogUserAction logs a user-initiated action
func (l *Logger) LogUserAction(ctx context.Context, userID uuid.UUID, action, resource string, resourceID *uuid.UUID, details map[string]interface{}) error {
	return l.LogEvent(ctx, &userID, nil, AuditEvent{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
	})
}

// LogTeamAction logs a team-scoped action
func (l *Logger) LogTeamAction(ctx context.Context, userID, teamID uuid.UUID, action, resource string, resourceID *uuid.UUID, details map[string]interface{}) error {
	return l.LogEvent(ctx, &userID, &teamID, AuditEvent{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
	})
}

// LogSystemAction logs a system-initiated action (no user)
func (l *Logger) LogSystemAction(ctx context.Context, action, resource string, resourceID *uuid.UUID, details map[string]interface{}) error {
	return l.LogEvent(ctx, nil, nil, AuditEvent{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
	})
}

// LogHTTPRequest logs an HTTP request for audit purposes
func (l *Logger) LogHTTPRequest(ctx context.Context, r *http.Request, userID *uuid.UUID, teamID *uuid.UUID, statusCode int, action, resource string, resourceID *uuid.UUID) error {
	event := AuditEvent{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  getClientIP(r),
		UserAgent:  r.UserAgent(),
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: statusCode,
	}

	return l.LogEvent(ctx, userID, teamID, event)
}

// Pre-defined audit actions
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionLogin  = "login"
	ActionLogout = "logout"
	ActionAccess = "access"
	ActionExport = "export"
	ActionImport = "import"
)

// Pre-defined resource types
const (
	ResourceUser       = "user"
	ResourceTeam       = "team"
	ResourceKey        = "key"
	ResourceUsage      = "usage"
	ResourceBudget     = "budget"
	ResourcePermission = "permission"
	ResourceSession    = "session"
	ResourceAPI        = "api"
	ResourceLLM        = "llm"
)

// Convenience methods for common audit events

// LogKeyCreated logs when a new key is created
func (l *Logger) LogKeyCreated(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID, keyID uuid.UUID, keyType string) error {
	details := map[string]interface{}{
		"key_type": keyType,
	}
	return l.LogEvent(ctx, &userID, teamID, AuditEvent{
		Action:     ActionCreate,
		Resource:   ResourceKey,
		ResourceID: &keyID,
		Details:    details,
	})
}

// LogKeyDeleted logs when a key is deleted. userID may be nil for system/master actions.
func (l *Logger) LogKeyDeleted(ctx context.Context, userID *uuid.UUID, teamID *uuid.UUID, keyID uuid.UUID, keyType string) error {
    details := map[string]interface{}{
        "key_type": keyType,
    }
    return l.LogEvent(ctx, userID, teamID, AuditEvent{
        Action:     ActionDelete,
        Resource:   ResourceKey,
        ResourceID: &keyID,
        Details:    details,
    })
}

// LogTeamCreated logs when a new team is created
func (l *Logger) LogTeamCreated(ctx context.Context, userID, teamID uuid.UUID, teamName string) error {
	details := map[string]interface{}{
		"team_name": teamName,
	}
	return l.LogEvent(ctx, &userID, &teamID, AuditEvent{
		Action:     ActionCreate,
		Resource:   ResourceTeam,
		ResourceID: &teamID,
		Details:    details,
	})
}

// LogTeamUpdated logs when a team is updated
func (l *Logger) LogTeamUpdated(ctx context.Context, userID, teamID uuid.UUID, changes map[string]interface{}) error {
	return l.LogEvent(ctx, &userID, &teamID, AuditEvent{
		Action:     ActionUpdate,
		Resource:   ResourceTeam,
		ResourceID: &teamID,
		Details:    changes,
	})
}

// LogUserLogin logs when a user logs in
func (l *Logger) LogUserLogin(ctx context.Context, userID uuid.UUID, provider, ipAddress, userAgent string) error {
	details := map[string]interface{}{
		"provider": provider,
	}
	return l.LogEvent(ctx, &userID, nil, AuditEvent{
		Action:    ActionLogin,
		Resource:  ResourceSession,
		Details:   details,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	})
}

// LogLLMRequest logs when an LLM request is made
func (l *Logger) LogLLMRequest(ctx context.Context, userID *uuid.UUID, teamID *uuid.UUID, keyID uuid.UUID, model string, tokens int, cost float64) error {
	details := map[string]interface{}{
		"model":  model,
		"tokens": tokens,
		"cost":   cost,
		"key_id": keyID,
	}
	return l.LogEvent(ctx, userID, teamID, AuditEvent{
		Action:   ActionAccess,
		Resource: ResourceLLM,
		Details:  details,
	})
}

// LogBudgetExceeded logs when a budget limit is exceeded
func (l *Logger) LogBudgetExceeded(ctx context.Context, userID *uuid.UUID, teamID *uuid.UUID, budgetType string, limit, usage float64) error {
	details := map[string]interface{}{
		"budget_type": budgetType,
		"limit":       limit,
		"usage":       usage,
		"exceeded_by": usage - limit,
	}
	return l.LogEvent(ctx, userID, teamID, AuditEvent{
		Action:   "budget_exceeded",
		Resource: ResourceBudget,
		Details:  details,
	})
}

// LogPermissionDenied logs when access is denied due to insufficient permissions
func (l *Logger) LogPermissionDenied(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID, permission, resource string, resourceID *uuid.UUID) error {
	details := map[string]interface{}{
		"permission": permission,
		"reason":     "insufficient_permissions",
	}
	return l.LogEvent(ctx, &userID, teamID, AuditEvent{
		Action:     "access_denied",
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
	})
}

// GetAuditLogs retrieves audit logs with filtering and pagination
func (l *Logger) GetAuditLogs(ctx context.Context, filters AuditLogFilters) ([]models.Audit, int64, error) {
	query := l.db.WithContext(ctx).Model(&models.Audit{})

	// Apply filters
	if filters.UserID != nil {
		query = query.Where("user_id = ?", *filters.UserID)
	}
	if filters.TeamID != nil {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.Action != "" {
		query = query.Where("event_action = ?", filters.Action)
	}
	if filters.Resource != "" {
		query = query.Where("resource_type = ?", filters.Resource)
	}
	if filters.ResourceID != nil {
		query = query.Where("resource_id = ?", *filters.ResourceID)
	}
	if !filters.StartDate.IsZero() {
		query = query.Where("timestamp >= ?", filters.StartDate)
	}
	if !filters.EndDate.IsZero() {
		query = query.Where("timestamp <= ?", filters.EndDate)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination and ordering
	var logs []models.Audit
	query = query.Order("timestamp DESC")

	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return logs, total, nil
}

// AuditLogFilters represents filters for querying audit logs
type AuditLogFilters struct {
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	TeamID     *uuid.UUID `json:"team_id,omitempty"`
	Action     string     `json:"action,omitempty"`
	Resource   string     `json:"resource,omitempty"`
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`
	StartDate  time.Time  `json:"start_date,omitempty"`
	EndDate    time.Time  `json:"end_date,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	Limit      int        `json:"limit,omitempty"`
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the chain
		return forwarded
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
