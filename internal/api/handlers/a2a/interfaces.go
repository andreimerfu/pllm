// Package a2a holds the HTTP handlers for the A2A gateway surface.
//
// Per repo convention (CLAUDE.md — "Key pattern: narrow interfaces at
// layer boundaries"), this file declares the dependencies this package
// expects — ScopeChecker and ScopeEventRecorder — so the package doesn't
// import `use_cases` directly. The concrete adapters live in the main
// router.go and bridge these interfaces to use_cases.ScopeResolver +
// use_cases.EventRecorder.
package a2a

import (
	"context"

	"github.com/google/uuid"
)

// ScopeDecision is the handler-facing mirror of use_cases.Decision.
type ScopeDecision struct {
	Allowed     bool
	EnforceMode bool
	Reason      string
}

// ScopeChecker decides whether a caller's key may invoke a named agent.
type ScopeChecker interface {
	AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) ScopeDecision
}

// ScopeEventRecorder persists a scope-enforcement event to the audit log.
type ScopeEventRecorder interface {
	RecordAgentDenial(
		ctx context.Context,
		useCaseID uuid.UUID,
		keyID *uuid.UUID, keyName, target, reason string, enforced bool,
		remoteAddr, requestID string,
	)
}
