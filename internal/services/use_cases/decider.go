package use_cases

import (
	"context"

	"github.com/google/uuid"
)

// Decider is the policy-decision contract behind every Policy Enforcement
// Point (the MCP gateway, chat/LLM, prompts, and A2A agent handlers). It
// answers allow/deny for a key's bound use case against a named target.
//
// Two implementations exist:
//   - ScopeResolver — the built-in, name-based scope check (local, no
//     external dependency).
//   - CompositeDecider — delegates to an external OPA PDP (e.g. Beacon) and
//     degrades to a local ScopeResolver when the PDP is unavailable.
//
// The per-handler router adapters call through this interface, so swapping
// the implementation is a wiring change in router.go — handler signatures
// never move.
type Decider interface {
	AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) Decision
	AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) Decision
	AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) Decision
	AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) Decision
}

// Compile-time assertion that the built-in resolver satisfies the contract.
var _ Decider = (*ScopeResolver)(nil)
