package use_cases_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

func TestRecordAndListScopeEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Triage",
	})
	require.NoError(t, err)

	rec := svc.Events()
	keyID := uuid.New()

	// Mix of MCP + LLM, denied + flagged so the filters have something to sort.
	rec.Record(context.Background(), use_cases.RecordInput{
		UseCaseID:  uc.UseCase.ID,
		KeyID:      &keyID,
		KeyName:    "test-key",
		Kind:       models.ScopeEventKindMCPTool,
		TargetName: "fs__delete_file",
		Outcome:    models.ScopeOutcomeDenied,
		Reason:     "not declared",
	})
	rec.Record(context.Background(), use_cases.RecordInput{
		UseCaseID:  uc.UseCase.ID,
		KeyID:      &keyID,
		KeyName:    "test-key",
		Kind:       models.ScopeEventKindLLM,
		TargetName: "openai/gpt-4o",
		Outcome:    models.ScopeOutcomeFlagged,
		Reason:     "flag mode",
	})

	all, err := rec.ListEvents(context.Background(), use_cases.EventListFilter{})
	require.NoError(t, err)
	require.Len(t, all, 2)

	denied, err := rec.ListEvents(context.Background(), use_cases.EventListFilter{
		Outcome: models.ScopeOutcomeDenied,
	})
	require.NoError(t, err)
	require.Len(t, denied, 1)
	require.Equal(t, "fs__delete_file", denied[0].TargetName)

	onlyLLM, err := rec.ListEvents(context.Background(), use_cases.EventListFilter{
		Kind: models.ScopeEventKindLLM,
	})
	require.NoError(t, err)
	require.Len(t, onlyLLM, 1)
	require.Equal(t, "openai/gpt-4o", onlyLLM[0].TargetName)

	// Stats aggregate both buckets.
	stats, err := rec.Stats(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Last24hDenied)
	require.Equal(t, int64(1), stats.Last24hFlagged)
}

// Recording a nil-UseCaseID row is a no-op — we should never generate
// orphan rows (the constraint has not-null on use_case_id anyway).
func TestRecordSkipsEmptyUseCase(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	rec := use_cases.NewEventRecorder(db, nil)
	rec.Record(context.Background(), use_cases.RecordInput{
		TargetName: "x",
		Outcome:    models.ScopeOutcomeDenied,
		Kind:       models.ScopeEventKindLLM,
	})
	var count int64
	require.NoError(t, db.Model(&models.ScopeEvent{}).Count(&count).Error)
	require.Zero(t, count)
}
