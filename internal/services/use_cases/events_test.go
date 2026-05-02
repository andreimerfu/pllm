package use_cases_test

import (
	"context"
	"testing"
	"time"

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

// BulkInsert backdates events for the demo seed. Caller-supplied
// timestamps are honoured so the Compliance timeline has shape.
func TestBulkInsertScopeEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Bulk insert test",
	})
	require.NoError(t, err)

	rec := svc.Events()
	now := time.Now().UTC()
	in := []use_cases.BulkRecordInput{
		{
			UseCaseID:  uc.UseCase.ID,
			Kind:       models.ScopeEventKindLLM,
			Outcome:    models.ScopeOutcomeDenied,
			TargetName: "gpt-4-turbo",
			KeyName:    "demo-key",
			Reason:     "test reason",
			OccurredAt: now.Add(-48 * time.Hour),
		},
		{
			UseCaseID:  uc.UseCase.ID,
			Kind:       models.ScopeEventKindLLM,
			Outcome:    models.ScopeOutcomeFlagged,
			TargetName: "tavily-search",
			KeyName:    "demo-key",
			Reason:     "test flag",
			OccurredAt: now.Add(-1 * time.Hour),
		},
	}
	n, err := rec.BulkInsert(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	var rows []models.ScopeEvent
	require.NoError(t, db.Order("created_at ASC").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.WithinDuration(t, now.Add(-48*time.Hour), rows[0].CreatedAt, time.Second)
	require.WithinDuration(t, now.Add(-1*time.Hour), rows[1].CreatedAt, time.Second)
	require.Equal(t, "gpt-4-turbo", rows[0].TargetName)
	require.Equal(t, models.ScopeOutcomeDenied, rows[0].Outcome)
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
