package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeScope struct {
	allowedLLM map[string]bool
	allowedMCP map[string]bool
}

func (f *fakeScope) AllowLLM(_ context.Context, _ *uuid.UUID, model string) discovery.OracleDecision {
	return discovery.OracleDecision{Allowed: f.allowedLLM[model]}
}
func (f *fakeScope) AllowMCPTool(_ context.Context, _ *uuid.UUID, slug string) discovery.OracleDecision {
	return discovery.OracleDecision{Allowed: f.allowedMCP[slug]}
}

type recordedEvent struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	Kind       models.ScopeEventKind
	Outcome    models.ScopeEventOutcome
	TargetName string
	Reason     string
}

type fakeRecorder struct{ events []recordedEvent }

func (f *fakeRecorder) Record(_ context.Context, in discovery.RecordInput) {
	f.events = append(f.events, recordedEvent{
		UseCaseID: in.UseCaseID, KeyID: in.KeyID,
		Kind: in.Kind, Outcome: in.Outcome,
		TargetName: in.TargetName, Reason: in.Reason,
	})
}

type fakeKeys struct {
	keys map[uuid.UUID]*models.Key
}

func (f *fakeKeys) GetKey(_ context.Context, id uuid.UUID) (*models.Key, error) {
	if k, ok := f.keys[id]; ok {
		return k, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func TestPolicy_VectorDB_Flagged(t *testing.T) {
	keyID, ucID := uuid.New(), uuid.New()
	keys := &fakeKeys{keys: map[uuid.UUID]*models.Key{
		keyID: {BaseModel: models.BaseModel{ID: keyID}, UseCaseID: &ucID},
	}}
	rec := &fakeRecorder{}
	pol := discovery.NewPolicy(zap.NewNop(), &fakeScope{}, rec, keys)

	agent := &models.DiscoveredAgent{
		BaseModel: models.BaseModel{ID: uuid.New()}, ServiceName: "fraud", KeyID: &keyID,
	}
	call := &models.DiscoveredCall{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Kind:      models.DiscoveredCallKindVectorDB, TargetName: "qdrant://customers",
		FirstSeenAt: time.Now(), LastSeenAt: time.Now(),
	}

	res := pol.Evaluate(context.Background(), agent, call)

	assert.Equal(t, models.ScopeOutcomeFlagged, res.Outcome)
	require.Len(t, rec.events, 1)
	assert.Equal(t, models.ScopeEventKindObserved, rec.events[0].Kind)
	assert.Equal(t, "qdrant://customers", rec.events[0].TargetName)
	assert.Equal(t, ucID, rec.events[0].UseCaseID)
}

func TestPolicy_LLM_AllowedSkipsRecord(t *testing.T) {
	keyID, ucID := uuid.New(), uuid.New()
	keys := &fakeKeys{keys: map[uuid.UUID]*models.Key{
		keyID: {BaseModel: models.BaseModel{ID: keyID}, UseCaseID: &ucID},
	}}
	rec := &fakeRecorder{}
	scope := &fakeScope{allowedLLM: map[string]bool{"gpt-4o": true}}
	pol := discovery.NewPolicy(zap.NewNop(), scope, rec, keys)

	res := pol.Evaluate(context.Background(),
		&models.DiscoveredAgent{BaseModel: models.BaseModel{ID: uuid.New()}, KeyID: &keyID},
		&models.DiscoveredCall{Kind: models.DiscoveredCallKindLLM, TargetName: "gpt-4o"},
	)

	assert.Equal(t, models.ScopeOutcomeAllowed, res.Outcome)
	assert.Len(t, rec.events, 0)
}

func TestPolicy_NoBoundKey_Skips(t *testing.T) {
	rec := &fakeRecorder{}
	pol := discovery.NewPolicy(zap.NewNop(), &fakeScope{}, rec, &fakeKeys{})
	res := pol.Evaluate(context.Background(),
		&models.DiscoveredAgent{},
		&models.DiscoveredCall{Kind: models.DiscoveredCallKindVectorDB, TargetName: "qdrant://x"},
	)
	assert.Equal(t, models.ScopeOutcomeAllowed, res.Outcome)
	assert.Empty(t, rec.events)
}
