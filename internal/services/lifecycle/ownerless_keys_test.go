package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestOwnerlessKeysRevokesKeyOfInactiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	// User.IsActive uses gorm default:true; struct-literal false is silently
	// overridden on insert. Force the column post-create.
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "off@x", Username: "off"})
	if err := db.Model(&models.User{}).Where("id = ?", uid).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	key := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_owner",
		KeyHash:   "hash_owner",
		KeyPrefix: "owner",
		Name:      "owner",
		IsActive:  true,
		UserID:    &uid,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatal(err)
	}

	emitter := &fakeEmitter{}
	rule := &OwnerlessKeysRule{}
	res := rule.Run(context.Background(), db, Config{OwnerlessKeysEnabled: true, DryRun: false}, emitter)

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	var got models.Key
	db.First(&got, "id = ?", key.ID)
	if got.IsActive {
		t.Fatal("expected revoked")
	}
	if len(emitter.events) != 1 || emitter.events[0].Type != models.WebhookEventKeyRevokedOwnerless {
		t.Fatalf("expected one ownerless event, got %+v", emitter.events)
	}

	// Audit row uses the canonical verb
	var auditRow models.Audit
	if err := db.Where("event_type = ? AND key_id = ?", models.AuditEventLifecycle, key.ID).First(&auditRow).Error; err != nil {
		t.Fatalf("load audit row: %v", err)
	}
	if auditRow.EventAction != "key_revoked_ownerless" {
		t.Fatalf("expected EventAction=key_revoked_ownerless, got %q", auditRow.EventAction)
	}
}

func TestOwnerlessKeysRevokesKeyOfDeletedUser(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	user := models.User{BaseModel: models.BaseModel{ID: uid}, Email: "del@x", Username: "del", IsActive: true}
	db.Create(&user)
	now := time.Now()
	db.Model(&models.User{}).Where("id = ?", uid).Update("deleted_at", now)

	key := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_del",
		KeyHash:   "hash_del",
		KeyPrefix: "del",
		Name:      "del",
		IsActive:  true,
		UserID:    &uid,
	}
	db.Create(&key)

	rule := &OwnerlessKeysRule{}
	res := rule.Run(context.Background(), db, Config{OwnerlessKeysEnabled: true, DryRun: false}, &fakeEmitter{})

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
}

func TestOwnerlessKeysSkipsTeamOnlyKey(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	tid := uuid.New()
	db.Create(&models.Team{BaseModel: models.BaseModel{ID: tid}, Name: "T"})
	db.Create(&models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_team",
		KeyHash:   "hash_team",
		KeyPrefix: "team",
		Name:      "team",
		IsActive:  true,
		TeamID:    &tid,
	})

	rule := &OwnerlessKeysRule{}
	res := rule.Run(context.Background(), db, Config{OwnerlessKeysEnabled: true, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("team-only key with no user must be skipped, got %d", len(res.Actions))
	}
}

func TestOwnerlessKeysDisabledByConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	uid := uuid.New()
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "x@x", Username: "x"})
	db.Model(&models.User{}).Where("id = ?", uid).Update("is_active", false)
	db.Create(&models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_off",
		KeyHash:   "hash_off",
		KeyPrefix: "off2",
		Name:      "off",
		IsActive:  true,
		UserID:    &uid,
	})
	rule := &OwnerlessKeysRule{}
	res := rule.Run(context.Background(), db, Config{OwnerlessKeysEnabled: false, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected disabled rule to no-op, got %d actions", len(res.Actions))
	}
}
