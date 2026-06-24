package use_cases

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// remoteDecider is the (Decision, error)-returning shape of an external
// PDP client. PDPDecider satisfies it.
type remoteDecider interface {
	AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) (Decision, error)
	AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) (Decision, error)
	AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) (Decision, error)
	AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) (Decision, error)
}

var _ remoteDecider = (*PDPDecider)(nil)

// OnError selects what CompositeDecider does when the remote PDP call fails.
type OnError string

const (
	// OnErrorFallbackLocal degrades to the built-in scope resolver — PDP
	// down still yields baseline name-based enforcement. This is the default.
	OnErrorFallbackLocal OnError = "fallback_local"
	// OnErrorDeny fails closed (block out-of-an-answer calls).
	OnErrorDeny OnError = "deny"
	// OnErrorAllow fails open (let calls through when the PDP is unreachable).
	OnErrorAllow OnError = "allow"
)

// CompositeDecider prefers a remote OPA PDP (e.g. Beacon) and applies an
// OnError policy when the PDP call fails. It implements Decider, so the
// router can drop it in behind the existing per-handler scope adapters.
type CompositeDecider struct {
	remote  remoteDecider
	local   Decider
	onError OnError
	log     *zap.Logger
}

// NewCompositeDecider wires a composite. An unrecognised onError value
// defaults to fallback_local; a nil logger is replaced with a no-op.
func NewCompositeDecider(remote remoteDecider, local Decider, onError string, log *zap.Logger) *CompositeDecider {
	mode := OnError(onError)
	switch mode {
	case OnErrorDeny, OnErrorAllow, OnErrorFallbackLocal:
	default:
		mode = OnErrorFallbackLocal
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &CompositeDecider{remote: remote, local: local, onError: mode, log: log.With(zap.String("component", "pdp"))}
}

var _ Decider = (*CompositeDecider)(nil)

func (c *CompositeDecider) AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) Decision {
	return c.resolve(
		func() (Decision, error) { return c.remote.AllowLLM(ctx, useCaseID, modelName) },
		func() Decision { return c.local.AllowLLM(ctx, useCaseID, modelName) },
		"llm", modelName,
	)
}

func (c *CompositeDecider) AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) Decision {
	return c.resolve(
		func() (Decision, error) { return c.remote.AllowMCPTool(ctx, useCaseID, serverSlug) },
		func() Decision { return c.local.AllowMCPTool(ctx, useCaseID, serverSlug) },
		"server", serverSlug,
	)
}

func (c *CompositeDecider) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) Decision {
	return c.resolve(
		func() (Decision, error) { return c.remote.AllowAgent(ctx, useCaseID, agentName) },
		func() Decision { return c.local.AllowAgent(ctx, useCaseID, agentName) },
		"agent", agentName,
	)
}

func (c *CompositeDecider) AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) Decision {
	return c.resolve(
		func() (Decision, error) { return c.remote.AllowPrompt(ctx, useCaseID, name) },
		func() Decision { return c.local.AllowPrompt(ctx, useCaseID, name) },
		"prompt", name,
	)
}

// resolve runs the remote PDP call and, on error, applies the OnError
// policy. kind/target are used only for diagnostic logging.
func (c *CompositeDecider) resolve(remoteFn func() (Decision, error), localFn func() Decision, kind, target string) Decision {
	d, err := remoteFn()
	if err == nil {
		return d
	}

	switch c.onError {
	case OnErrorDeny:
		c.log.Warn("pdp unavailable, failing closed",
			zap.String("kind", kind), zap.String("target", target), zap.Error(err))
		return Decision{Allowed: false, EnforceMode: true, Reason: "policy decision point unavailable"}
	case OnErrorAllow:
		c.log.Warn("pdp unavailable, failing open",
			zap.String("kind", kind), zap.String("target", target), zap.Error(err))
		return Decision{Allowed: true, Reason: "policy decision point unavailable (fail-open)"}
	default: // OnErrorFallbackLocal
		c.log.Warn("pdp unavailable, falling back to local scope",
			zap.String("kind", kind), zap.String("target", target), zap.Error(err))
		return localFn()
	}
}
