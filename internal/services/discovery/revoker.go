package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// KeyRevoker is the surface the discovery package needs to revoke an
// API key. Bridged in router.go to the existing key service.
type KeyRevoker interface {
	Revoke(ctx context.Context, keyID uuid.UUID, reason string) error
}

// Revoker fires KeyRevoker.Revoke when an agent's flagged count reaches
// the threshold. It dedupes so the same agent doesn't trigger twice.
type Revoker struct {
	logger    *zap.Logger
	adapter   KeyRevoker
	threshold int

	mu    sync.Mutex
	fired map[uuid.UUID]bool
}

// NewRevoker returns a Revoker. threshold <= 0 disables auto-revoke.
func NewRevoker(logger *zap.Logger, adapter KeyRevoker, threshold int) *Revoker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Revoker{
		logger:    logger,
		adapter:   adapter,
		threshold: threshold,
		fired:     map[uuid.UUID]bool{},
	}
}

// Forget drops the latch for an agent so a future flagged-count
// crossing can fire revocation again. Used by Service.Reset so the
// demo can iterate without restarting pllm.
func (r *Revoker) Forget(agentID uuid.UUID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.fired, agentID)
	r.mu.Unlock()
}

// OnFlagged is called after each flagged event. The Service has already
// incremented FlaggedCount; we read agent.FlaggedCount.
func (r *Revoker) OnFlagged(ctx context.Context, agent *models.DiscoveredAgent) {
	if r == nil || r.threshold <= 0 || r.adapter == nil || agent == nil || agent.KeyID == nil {
		return
	}
	if agent.FlaggedCount < r.threshold {
		return
	}
	r.mu.Lock()
	if r.fired[agent.ID] {
		r.mu.Unlock()
		return
	}
	r.fired[agent.ID] = true
	r.mu.Unlock()

	reason := fmt.Sprintf("auto-revoked: discovered agent %s exceeded flagged threshold (%d)",
		agent.ServiceName, r.threshold)
	if err := r.adapter.Revoke(ctx, *agent.KeyID, reason); err != nil {
		r.logger.Warn("auto-revoke failed",
			zap.String("agent", agent.ServiceName),
			zap.Error(err))
		r.mu.Lock()
		delete(r.fired, agent.ID)
		r.mu.Unlock()
		return
	}
	r.logger.Info("auto-revoked key for over-flagged discovered agent",
		zap.String("agent", agent.ServiceName),
		zap.String("key_id", agent.KeyID.String()),
		zap.Int("flagged_count", agent.FlaggedCount))
}
