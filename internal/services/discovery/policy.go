package discovery

import (
	"context"
	"net"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ScopeOracle is the read-only surface this package needs from the
// use_cases service. The router bridges this to *use_cases.ScopeResolver.
type ScopeOracle interface {
	AllowLLM(ctx context.Context, useCaseID *uuid.UUID, model string) OracleDecision
	AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, slug string) OracleDecision
	AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) OracleDecision
}

// OracleDecision is a copy of use_cases.Decision with only the field
// we need. Keeping it local avoids a package import cycle.
type OracleDecision struct {
	Allowed bool
	Reason  string
}

// RecordInput mirrors use_cases.RecordInput. Bridged in router.go.
type RecordInput struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	KeyName    string
	UserID     *uuid.UUID
	Kind       models.ScopeEventKind
	TargetName string
	Outcome    models.ScopeEventOutcome
	Reason     string
}

type EventRecorder interface {
	Record(ctx context.Context, in RecordInput)
}

// KeyLoader fetches a Key by id. Bridged to a small repository fn in
// router.go — we don't import the auth/key services from here.
type KeyLoader interface {
	GetKey(ctx context.Context, id uuid.UUID) (*models.Key, error)
}

// Policy is the default evaluator.
type Policy struct {
	logger *zap.Logger
	scope  ScopeOracle
	rec    EventRecorder
	keys   KeyLoader
}

func NewPolicy(logger *zap.Logger, scope ScopeOracle, rec EventRecorder, keys KeyLoader) *Policy {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Policy{logger: logger, scope: scope, rec: rec, keys: keys}
}

// Evaluate decides whether the call is in scope. Side-effects:
//   - allowed: nothing
//   - flagged: emit RecordInput (audit + webhook)
//
// We never block here — the gateway has already been bypassed. This is
// detect-and-record only.
func (p *Policy) Evaluate(ctx context.Context, agent *models.DiscoveredAgent, call *models.DiscoveredCall) PolicyResult {
	if agent == nil || agent.KeyID == nil {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed, Reason: "no bound key"}
	}
	key, err := p.keys.GetKey(ctx, *agent.KeyID)
	if err != nil || key == nil || key.UseCaseID == nil {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed, Reason: "no bound use case"}
	}

	// SDK-side block takes precedence: the call never happened, but the
	// agent attempted it — record as ScopeOutcomeBlocked so the UI can
	// distinguish "prevented by SDK" from "observed too late to stop".
	if call.BlockedAtClient {
		reason := "blocked locally by client SDK"
		p.rec.Record(ctx, RecordInput{
			UseCaseID:  *key.UseCaseID,
			KeyID:      &key.ID,
			KeyName:    key.Name,
			UserID:     key.UserID,
			Kind:       models.ScopeEventKindObserved,
			TargetName: call.TargetName,
			Outcome:    models.ScopeOutcomeBlocked,
			Reason:     reason,
		})
		return PolicyResult{Outcome: models.ScopeOutcomeBlocked, Reason: reason}
	}

	allowed, reason := p.allow(ctx, key.UseCaseID, call)
	if allowed {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed}
	}

	p.rec.Record(ctx, RecordInput{
		UseCaseID:  *key.UseCaseID,
		KeyID:      &key.ID,
		KeyName:    key.Name,
		UserID:     key.UserID,
		Kind:       models.ScopeEventKindObserved,
		TargetName: call.TargetName,
		Outcome:    models.ScopeOutcomeFlagged,
		Reason:     reason,
	})
	return PolicyResult{Outcome: models.ScopeOutcomeFlagged, Reason: reason}
}

func (p *Policy) allow(ctx context.Context, ucID *uuid.UUID, call *models.DiscoveredCall) (bool, string) {
	switch call.Kind {
	case models.DiscoveredCallKindLLM:
		d := p.scope.AllowLLM(ctx, ucID, call.TargetName)
		if d.Allowed {
			return true, ""
		}
		return false, "llm '" + call.TargetName + "' not declared by bound use case"
	case models.DiscoveredCallKindMCP:
		d := p.scope.AllowMCPTool(ctx, ucID, call.TargetName)
		if d.Allowed {
			return true, ""
		}
		return false, "mcp '" + call.TargetName + "' not declared by bound use case"
	case models.DiscoveredCallKindAgent:
		d := p.scope.AllowAgent(ctx, ucID, call.TargetName)
		if d.Allowed {
			return true, ""
		}
		return false, "agent '" + call.TargetName + "' not declared by bound use case"
	case models.DiscoveredCallKindVectorDB:
		return false, "vector db call observed via OTel; cannot be governed at gateway"
	case models.DiscoveredCallKindHTTP:
		if isInternalLoopback(call.TargetName) {
			return true, ""
		}
		return false, "outbound http call observed via OTel; not declared by bound use case"
	default:
		return false, "unclassified call observed via OTel"
	}
}

func isInternalLoopback(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return false
}
