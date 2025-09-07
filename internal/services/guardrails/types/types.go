package types

import (
	"context"
	"time"
)

// GuardrailMode defines when a guardrail should execute
type GuardrailMode string

const (
	PreCall     GuardrailMode = "pre_call"     // Execute before LLM call
	PostCall    GuardrailMode = "post_call"    // Execute after LLM call
	DuringCall  GuardrailMode = "during_call"  // Execute in parallel with LLM call
	LoggingOnly GuardrailMode = "logging_only" // Only log, don't block
)

func (m GuardrailMode) String() string {
	return string(m)
}

// GuardrailType defines the category of guardrail
type GuardrailType string

const (
	PII        GuardrailType = "pii"         // Personal Identifiable Information detection
	Security   GuardrailType = "security"   // Security threat detection
	Moderation GuardrailType = "moderation" // Content moderation
	Compliance GuardrailType = "compliance" // Regulatory compliance
)

func (t GuardrailType) String() string {
	return string(t)
}

// GuardrailInput contains the request data to be analyzed by guardrails
type GuardrailInput struct {
	// Request data - one or both will be present depending on execution mode
	Request  interface{} `json:"request,omitempty"`  // Chat request (pre_call, during_call)
	Response interface{} `json:"response,omitempty"` // Chat response (post_call, during_call)
	
	// Context information
	UserID    string `json:"user_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	
	// Request metadata
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GuardrailResult contains the analysis results and actions to take
type GuardrailResult struct {
	// Execution result
	Passed   bool `json:"passed"`
	Blocked  bool `json:"blocked"`
	Modified bool `json:"modified"`
	
	// Details
	Reason     string                 `json:"reason,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	
	// Modifications (for PII masking, etc.)
	ModifiedRequest  interface{} `json:"modified_request,omitempty"`
	ModifiedResponse interface{} `json:"modified_response,omitempty"`
	
	// Execution metadata
	ExecutionTime time.Duration `json:"execution_time"`
	GuardrailName string        `json:"guardrail_name"`
	GuardrailType string        `json:"guardrail_type"`
	
	// For logging and monitoring
	RawResult interface{} `json:"raw_result,omitempty"`
}

// Guardrail interface that all guardrails must implement
type Guardrail interface {
	Execute(ctx context.Context, input *GuardrailInput) (*GuardrailResult, error)
	GetName() string
	GetType() GuardrailType
	GetMode() GuardrailMode
	IsEnabled() bool
	HealthCheck(ctx context.Context) error
}

// GuardrailStats contains statistics about guardrail execution
type GuardrailStats struct {
	TotalExecutions int64         `json:"total_executions"`
	TotalPassed     int64         `json:"total_passed"`
	TotalBlocked    int64         `json:"total_blocked"`
	TotalErrors     int64         `json:"total_errors"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastExecuted    time.Time     `json:"last_executed"`
}

// HealthStatus represents the health status of a guardrail
type HealthStatus struct {
	Name      string                 `json:"name"`
	Healthy   bool                   `json:"healthy"`
	Status    string                 `json:"status"`
	LastCheck time.Time              `json:"last_check"`
	Details   map[string]interface{} `json:"details,omitempty"`
}