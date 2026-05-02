package lifecycle

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestServiceRunOnceWithNoRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres testcontainer")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := NewService(db, zap.NewNop(), Config{DryRun: true})
	summary, err := svc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if summary.TotalActions != 0 {
		t.Fatalf("expected 0 actions, got %d", summary.TotalActions)
	}
	if len(summary.Rules) != 0 {
		t.Fatalf("expected 0 rule results, got %d", len(summary.Rules))
	}
}

func TestServiceLastSummaryAvailableAfterRun(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres testcontainer")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := NewService(db, zap.NewNop(), Config{DryRun: true})
	if _, err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := svc.LastSummary()
	if got.RanAt.IsZero() {
		t.Fatal("expected LastSummary.RanAt populated after RunOnce")
	}
}
