package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestRiskyServersBlocksLowScoredServer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	srv := models.RegistryServer{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "io.pllm.test/risky",
		Version:     "1.0.0",
		Status:      models.RegistryStatusActive,
		IsLatest:    true,
		PublishedAt: time.Now(),
	}
	if err := db.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}
	score := models.EnrichmentScore{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceKind: models.RegistryKindServer,
		ResourceID:   srv.ID,
		Type:         models.EnrichmentTypeOSV,
		Score:        20.0,
		Summary:      "3 high-severity CVEs",
		ScannedAt:    time.Now(),
	}
	db.Create(&score)

	emitter := &fakeEmitter{}
	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 40, DryRun: false}, emitter)

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	var after models.RegistryServer
	db.First(&after, "id = ?", srv.ID)
	if after.Status != models.RegistryStatusBlocked {
		t.Fatalf("expected status=blocked, got %q", after.Status)
	}
	if len(emitter.events) != 1 || emitter.events[0].Type != models.WebhookEventRegistryServerBlockedRisky {
		t.Fatalf("expected risky-block webhook, got %+v", emitter.events)
	}

	// Audit row uses canonical verb
	var auditRow models.Audit
	if err := db.Where("event_type = ? AND resource_id = ?", models.AuditEventLifecycle, srv.ID).First(&auditRow).Error; err != nil {
		t.Fatalf("load audit row: %v", err)
	}
	if auditRow.EventAction != "server_blocked_risky" {
		t.Fatalf("expected EventAction=server_blocked_risky, got %q", auditRow.EventAction)
	}
}

func TestRiskyServersSkipsAcceptableScore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	srv := models.RegistryServer{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "io.pllm.test/safe",
		Version:     "1.0.0",
		Status:      models.RegistryStatusActive,
		IsLatest:    true,
		PublishedAt: time.Now(),
	}
	db.Create(&srv)
	db.Create(&models.EnrichmentScore{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceKind: models.RegistryKindServer,
		ResourceID:   srv.ID,
		Type:         models.EnrichmentTypeOSV,
		Score:        85.0,
		ScannedAt:    time.Now(),
	})

	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 40, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(res.Actions))
	}
	var after models.RegistryServer
	db.First(&after, "id = ?", srv.ID)
	if after.Status != models.RegistryStatusActive {
		t.Fatalf("expected status untouched, got %q", after.Status)
	}
}

func TestRiskyServersIdempotentForAlreadyBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	srv := models.RegistryServer{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "io.pllm.test/blocked",
		Version:     "1.0.0",
		Status:      models.RegistryStatusBlocked,
		IsLatest:    true,
		PublishedAt: time.Now(),
	}
	db.Create(&srv)
	db.Create(&models.EnrichmentScore{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceKind: models.RegistryKindServer,
		ResourceID:   srv.ID,
		Type:         models.EnrichmentTypeOSV,
		Score:        10.0,
		ScannedAt:    time.Now(),
	})

	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 40, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("already-blocked must be skipped, got %d actions", len(res.Actions))
	}
}

func TestRiskyServersDisabledWhenFloorZero(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 0, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("floor=0 must disable rule, got %d actions", len(res.Actions))
	}
}

func TestRiskyServersDryRunDoesNotMutate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	srv := models.RegistryServer{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "io.pllm.test/dryrun",
		Version:     "1.0.0",
		Status:      models.RegistryStatusActive,
		IsLatest:    true,
		PublishedAt: time.Now(),
	}
	db.Create(&srv)
	db.Create(&models.EnrichmentScore{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceKind: models.RegistryKindServer,
		ResourceID:   srv.ID,
		Type:         models.EnrichmentTypeOSV,
		Score:        15.0,
		ScannedAt:    time.Now(),
	})

	emitter := &fakeEmitter{}
	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 40, DryRun: true}, emitter)
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 dry-run action, got %d", len(res.Actions))
	}
	var after models.RegistryServer
	db.First(&after, "id = ?", srv.ID)
	if after.Status != models.RegistryStatusActive {
		t.Fatalf("dry-run must NOT mutate status, got %q", after.Status)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("dry-run must NOT emit webhooks, got %d", len(emitter.events))
	}
}
