package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

// fakeEmitter records every Emit so tests can assert payloads. Shared
// across rule tests in this package.
type fakeEmitter struct {
	events []WebhookEventEnvelope
}

func (f *fakeEmitter) Emit(env WebhookEventEnvelope) {
	f.events = append(f.events, env)
}

func TestInactiveKeysRevokesOldKeyInLiveMode(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	uid := uuid.New()
	old := time.Now().Add(-100 * 24 * time.Hour)
	user := models.User{BaseModel: models.BaseModel{ID: uid}, Email: "u@x", Username: "u", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	key := models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		Key:        "pllm_ak_stale",
		KeyHash:    "hash_stale",
		KeyPrefix:  "stale",
		Name:       "stale",
		IsActive:   true,
		UserID:     &uid,
		LastUsedAt: &old,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatal(err)
	}

	emitter := &fakeEmitter{}
	cfg := Config{InactiveKeyDays: 90, DryRun: false}
	rule := &InactiveKeysRule{}
	res := rule.Run(context.Background(), db, cfg, emitter)

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	var got models.Key
	db.First(&got, "id = ?", key.ID)
	if got.IsActive {
		t.Fatal("expected key to be revoked (IsActive=false)")
	}
	if got.RevokedAt == nil {
		t.Fatal("expected RevokedAt set")
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 webhook emit, got %d", len(emitter.events))
	}
	if emitter.events[0].Type != models.WebhookEventKeyRevokedInactive {
		t.Fatalf("expected event type %q, got %q",
			models.WebhookEventKeyRevokedInactive, emitter.events[0].Type)
	}

	var auditCount int64
	db.Model(&models.Audit{}).
		Where("event_type = ? AND key_id = ?", models.AuditEventLifecycle, key.ID).
		Count(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

func TestInactiveKeysRespectsDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	old := time.Now().Add(-100 * 24 * time.Hour)
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "u2@x", Username: "u2", IsActive: true})
	key := models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		Key:        "pllm_ak_dryrun",
		KeyHash:    "hash_dryrun",
		KeyPrefix:  "dryrun",
		IsActive:   true,
		UserID:     &uid,
		LastUsedAt: &old,
	}
	db.Create(&key)

	emitter := &fakeEmitter{}
	rule := &InactiveKeysRule{}
	res := rule.Run(context.Background(), db, Config{InactiveKeyDays: 90, DryRun: true}, emitter)

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action (logged), got %d", len(res.Actions))
	}
	var got models.Key
	db.First(&got, "id = ?", key.ID)
	if !got.IsActive {
		t.Fatal("dry-run must NOT mutate IsActive")
	}
	if len(emitter.events) != 0 {
		t.Fatalf("dry-run must NOT emit webhooks, got %d", len(emitter.events))
	}
}

func TestInactiveKeysSkipsRecentlyUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	recent := time.Now().Add(-1 * 24 * time.Hour)
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "u3@x", Username: "u3", IsActive: true})
	db.Create(&models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		Key:        "pllm_ak_fresh",
		KeyHash:    "hash_fresh",
		KeyPrefix:  "fresh",
		IsActive:   true,
		UserID:     &uid,
		LastUsedAt: &recent,
	})

	rule := &InactiveKeysRule{}
	res := rule.Run(context.Background(), db, Config{InactiveKeyDays: 90, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(res.Actions))
	}
}

func TestInactiveKeysSkipsAlreadyRevoked(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	old := time.Now().Add(-200 * 24 * time.Hour)
	revokedAt := time.Now().Add(-30 * 24 * time.Hour)
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "u4@x", Username: "u4", IsActive: true})
	// Note: IsActive uses gorm default:true, so a struct literal with
	// IsActive:false is silently overridden on insert. Pair with
	// RevokedAt so the rule's filter (revoked_at IS NULL) excludes it.
	db.Create(&models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		Key:        "pllm_ak_dead",
		KeyHash:    "hash_dead",
		KeyPrefix:  "dead",
		UserID:     &uid,
		LastUsedAt: &old,
		RevokedAt:  &revokedAt,
	})

	rule := &InactiveKeysRule{}
	res := rule.Run(context.Background(), db, Config{InactiveKeyDays: 90, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions on already-revoked, got %d", len(res.Actions))
	}
}
