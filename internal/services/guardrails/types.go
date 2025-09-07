package guardrails

import (
	"github.com/amerfu/pllm/internal/services/guardrails/types"
)

// Re-export types for convenience
type (
	GuardrailMode   = types.GuardrailMode
	GuardrailType   = types.GuardrailType
	GuardrailInput  = types.GuardrailInput
	GuardrailResult = types.GuardrailResult
	Guardrail       = types.Guardrail
	GuardrailStats  = types.GuardrailStats
	HealthStatus    = types.HealthStatus
)

// Re-export constants
const (
	PreCall     = types.PreCall
	PostCall    = types.PostCall
	DuringCall  = types.DuringCall
	LoggingOnly = types.LoggingOnly

	PII        = types.PII
	Security   = types.Security
	Moderation = types.Moderation
	Compliance = types.Compliance
)

// ParseGuardrailMode converts string to GuardrailMode
func ParseGuardrailMode(mode string) GuardrailMode {
	switch mode {
	case "pre_call":
		return PreCall
	case "post_call":
		return PostCall
	case "during_call":
		return DuringCall
	case "logging_only":
		return LoggingOnly
	default:
		return PreCall // Default fallback
	}
}

// GuardrailError represents errors from guardrail execution
type GuardrailError struct {
	GuardrailName string
	GuardrailType string
	Reason        string
	Details       map[string]interface{}
	Blocked       bool
}

func (e *GuardrailError) Error() string {
	if e.Blocked {
		return "Request blocked by guardrail '" + e.GuardrailName + "': " + e.Reason
	}
	return "Guardrail '" + e.GuardrailName + "' failed: " + e.Reason
}