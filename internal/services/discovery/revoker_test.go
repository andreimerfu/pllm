package discovery_test

import (
	"context"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type revokeCall struct {
	KeyID  uuid.UUID
	Reason string
}

type fakeRevokeAdapter struct{ calls []revokeCall }

func (f *fakeRevokeAdapter) Revoke(_ context.Context, keyID uuid.UUID, reason string) error {
	f.calls = append(f.calls, revokeCall{KeyID: keyID, Reason: reason})
	return nil
}

func TestRevoker_FiresAtThreshold(t *testing.T) {
	adapter := &fakeRevokeAdapter{}
	rev := discovery.NewRevoker(zap.NewNop(), adapter, 3)

	keyID := uuid.New()
	agent := &models.DiscoveredAgent{
		BaseModel: models.BaseModel{ID: uuid.New()},
		KeyID:     &keyID,
	}

	agent.FlaggedCount = 1
	rev.OnFlagged(context.Background(), agent)
	agent.FlaggedCount = 2
	rev.OnFlagged(context.Background(), agent)
	require.Empty(t, adapter.calls)

	agent.FlaggedCount = 3
	rev.OnFlagged(context.Background(), agent)
	require.Len(t, adapter.calls, 1)
	assert.Equal(t, keyID, adapter.calls[0].KeyID)

	agent.FlaggedCount = 4
	rev.OnFlagged(context.Background(), agent)
	require.Len(t, adapter.calls, 1)
}

func TestRevoker_DisabledWhenThresholdZero(t *testing.T) {
	adapter := &fakeRevokeAdapter{}
	rev := discovery.NewRevoker(zap.NewNop(), adapter, 0)

	keyID := uuid.New()
	agent := &models.DiscoveredAgent{KeyID: &keyID, FlaggedCount: 99}
	rev.OnFlagged(context.Background(), agent)
	require.Empty(t, adapter.calls)
}
