# Lifecycle Hygiene Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an in-process lifecycle daemon that automatically flags or revokes inactive keys, revokes ownerless keys, and blocks registry servers whose OSV score has dropped below threshold — emitting webhooks and writing audit-log entries for every action.

**Architecture:** Mirror the existing `enrichment.Runner` pattern: a single Go service in `internal/services/lifecycle/` started from `internal/api/router/router.go`. The service holds a slice of `Rule` implementations and runs them on a periodic ticker plus on-demand via an admin endpoint. Each rule reads its inputs (Keys with stale `LastUsedAt`, Keys whose User has `IsActive=false`, RegistryServers whose latest `EnrichmentScore` is below threshold), takes its action via direct GORM updates on existing models, writes an Audit row, and pushes a `webhooks.Event` to the existing `webhooks.Dispatcher`. No new persistent table is added; the audit log already gives us a queryable history.

**Tech Stack:** Go 1.22, GORM, Chi router, Zap logger, React + TypeScript (frontend), shadcn/ui components.

---

## File structure

**Backend (Go):**
- Create: `internal/services/lifecycle/service.go` — Service struct, Rule interface, run loop
- Create: `internal/services/lifecycle/inactive_keys.go` — `InactiveKeysRule`
- Create: `internal/services/lifecycle/ownerless_keys.go` — `OwnerlessKeysRule`
- Create: `internal/services/lifecycle/risky_servers.go` — `RiskyServersRule`
- Create: `internal/services/lifecycle/service_test.go`
- Create: `internal/services/lifecycle/inactive_keys_test.go`
- Create: `internal/services/lifecycle/ownerless_keys_test.go`
- Create: `internal/services/lifecycle/risky_servers_test.go`
- Create: `internal/api/handlers/lifecycle/handler.go` — admin endpoints
- Modify: `internal/core/models/webhook.go` — add 3 new event-type constants
- Modify: `internal/core/models/registry.go` — add `RegistryStatusBlocked` constant
- Modify: `internal/core/models/audit.go` — add `AuditEventLifecycle`
- Modify: `internal/core/config/config.go` — add `LifecycleConfig`
- Modify: `internal/api/router/router.go` — wire service init + start
- Modify: `internal/api/router/admin.go` — mount handler routes

**Frontend (TypeScript):**
- Modify: `web/src/pages/Compliance.tsx` — add "Hygiene" tab
- Modify: `web/src/lib/api.ts` — add `runLifecycle()` and `getLifecycleStatus()` calls
- Modify: `web/src/types/api.ts` — add `LifecycleStatus`, `LifecycleAction` types

**Smoke test:**
- Modify: `scripts/test-mcp-registry.sh` — append a lifecycle section that creates a stale key, runs lifecycle, asserts it was revoked

**No new GORM model is added in this plan**, so the three AutoMigrate lists (`database/connection.go`, `database/migrate.go`, `testutil/database.go`) do not need updating.

---

## Task 1: Add new constants for webhook events, registry status, and audit type

**Files:**
- Modify: `internal/core/models/webhook.go:30` (after the Registry section block)
- Modify: `internal/core/models/registry.go:55-58`
- Modify: `internal/core/models/audit.go:97`
- Test: `internal/core/models/webhook_test.go` (create)

- [ ] **Step 1: Write failing test for the new event types**

Create `internal/core/models/webhook_test.go`:

```go
package models

import "testing"

func TestSubscribesAcceptsLifecycleEvents(t *testing.T) {
	w := &Webhook{Events: []string{
		string(WebhookEventKeyRevokedInactive),
		string(WebhookEventKeyRevokedOwnerless),
		string(WebhookEventRegistryServerBlockedRisky),
	}}
	for _, evt := range []WebhookEventType{
		WebhookEventKeyRevokedInactive,
		WebhookEventKeyRevokedOwnerless,
		WebhookEventRegistryServerBlockedRisky,
	} {
		if !w.Subscribes(evt) {
			t.Fatalf("expected webhook to subscribe to %q", evt)
		}
	}
}

func TestRegistryStatusBlockedDefined(t *testing.T) {
	if RegistryStatusBlocked != "blocked" {
		t.Fatalf("expected RegistryStatusBlocked == \"blocked\", got %q", RegistryStatusBlocked)
	}
}

func TestAuditEventLifecycleDefined(t *testing.T) {
	if AuditEventLifecycle != "lifecycle" {
		t.Fatalf("expected AuditEventLifecycle == \"lifecycle\", got %q", AuditEventLifecycle)
	}
}
```

- [ ] **Step 2: Run test, verify compile failure**

Run: `go test ./internal/core/models/ -run TestSubscribes -v`
Expected: FAIL with "undefined: WebhookEventKeyRevokedInactive" (or similar) — does not compile.

- [ ] **Step 3: Add the three new webhook event constants**

In `internal/core/models/webhook.go`, after the existing `WebhookEventRegistryServerSaved` constant (around line 28), add:

```go
	// Lifecycle hygiene — automatic key/server cleanup actions taken by
	// the lifecycle daemon. Subscribers typically forward these to SIEM
	// or to a "weekly cleanup digest" channel.
	WebhookEventKeyRevokedInactive         WebhookEventType = "key.revoked.inactive"
	WebhookEventKeyRevokedOwnerless        WebhookEventType = "key.revoked.ownerless"
	WebhookEventRegistryServerBlockedRisky WebhookEventType = "registry.server.blocked.risky"
```

- [ ] **Step 4: Add the new RegistryStatus constant**

In `internal/core/models/registry.go`, replace the existing `RegistryStatus` const block (around line 54-58):

```go
const (
	RegistryStatusActive     RegistryStatus = "active"
	RegistryStatusDeprecated RegistryStatus = "deprecated"
	RegistryStatusDeleted    RegistryStatus = "deleted"
	// Blocked is a non-deletable, machine-set state used by the lifecycle
	// daemon when a server's vulnerability score falls below the configured
	// floor. Operators clear it manually after triage.
	RegistryStatusBlocked RegistryStatus = "blocked"
)
```

- [ ] **Step 5: Add the new AuditEventType**

In `internal/core/models/audit.go`, after `AuditEventAccessDenied` (around line 97), add:

```go
	// Lifecycle hygiene — every action the lifecycle daemon takes is logged
	// as a single AuditEventLifecycle row whose EventAction names the rule
	// (e.g. "key_revoked_inactive", "server_blocked_risky").
	AuditEventLifecycle AuditEventType = "lifecycle"
```

- [ ] **Step 6: Run tests, verify pass**

Run: `go test ./internal/core/models/ -run "TestSubscribes|TestRegistryStatus|TestAuditEventLifecycle" -v`
Expected: PASS, 3 tests run.

- [ ] **Step 7: Commit**

```bash
git add internal/core/models/webhook.go internal/core/models/registry.go internal/core/models/audit.go internal/core/models/webhook_test.go
git commit -m "feat(models): add lifecycle event types, blocked registry status, and lifecycle audit type"
```

---

## Task 2: Add LifecycleConfig to config

**Files:**
- Modify: `internal/core/config/config.go` (add new struct + field on `Config`)
- Test: `internal/core/config/config_test.go` (add new test, file may exist; if not, create)

- [ ] **Step 1: Inspect current config layout**

Run: `grep -n "type Config struct\|Deployment.*DeploymentConfig\|MCP.*MCPConfig" internal/core/config/config.go`
Note the exact location of the top-level `Config` struct so the new field slots into the same style.

- [ ] **Step 2: Write failing test**

Append to `internal/core/config/config_test.go` (create if missing):

```go
package config

import "testing"

func TestDefaultLifecycleConfig(t *testing.T) {
	c := DefaultLifecycleConfig()
	if !c.Enabled {
		t.Fatal("expected Enabled=true by default")
	}
	if c.Interval.String() != "1h0m0s" {
		t.Fatalf("expected default Interval=1h, got %s", c.Interval)
	}
	if c.InactiveKeyDays != 90 {
		t.Fatalf("expected default InactiveKeyDays=90, got %d", c.InactiveKeyDays)
	}
	if c.RiskyServerScoreFloor != 40 {
		t.Fatalf("expected default RiskyServerScoreFloor=40, got %f", c.RiskyServerScoreFloor)
	}
	if !c.DryRun {
		t.Fatal("expected DryRun=true by default — operators must opt in to enforcement")
	}
	if !c.OwnerlessKeysEnabled {
		t.Fatal("expected OwnerlessKeysEnabled=true by default")
	}
}
```

- [ ] **Step 3: Run test, verify it fails to compile**

Run: `go test ./internal/core/config/ -run TestDefaultLifecycleConfig -v`
Expected: FAIL, "undefined: DefaultLifecycleConfig".

- [ ] **Step 4: Add the LifecycleConfig struct + DefaultLifecycleConfig constructor**

In `internal/core/config/config.go`, near the other `*Config` struct definitions, add:

```go
// LifecycleConfig governs the lifecycle hygiene daemon: how often it runs,
// what thresholds it applies, and whether it actually mutates data.
//
// DryRun=true is the canonical first-install posture — the daemon should
// not revoke half a fleet of keys before the operator has reviewed the
// audit log. Operators flip DryRun=false explicitly via config.
type LifecycleConfig struct {
	Enabled  bool          `mapstructure:"enabled" yaml:"enabled"`
	Interval time.Duration `mapstructure:"interval" yaml:"interval"`
	DryRun   bool          `mapstructure:"dry_run" yaml:"dry_run"`

	// InactiveKeyDays — keys whose LastUsedAt (or CreatedAt for never-used
	// keys) is older than this are revoked. 0 disables the rule.
	InactiveKeyDays int `mapstructure:"inactive_key_days" yaml:"inactive_key_days"`

	// OwnerlessKeysEnabled — when true, revoke keys whose User row has
	// IsActive=false or has been soft-deleted. Team-owned keys with no
	// user are unaffected.
	OwnerlessKeysEnabled bool `mapstructure:"ownerless_keys_enabled" yaml:"ownerless_keys_enabled"`

	// RiskyServerScoreFloor — RegistryServers whose latest OSV
	// EnrichmentScore is BELOW this float are auto-blocked. 0 disables.
	RiskyServerScoreFloor float64 `mapstructure:"risky_server_score_floor" yaml:"risky_server_score_floor"`
}

// DefaultLifecycleConfig returns the default lifecycle settings. Used as
// the starting point that viper merges YAML on top of, so an operator
// who sets `dry_run: false` actually gets DryRun=false (no override bug).
func DefaultLifecycleConfig() LifecycleConfig {
	return LifecycleConfig{
		Enabled:               true,
		Interval:              time.Hour,
		DryRun:                true,
		InactiveKeyDays:       90,
		OwnerlessKeysEnabled:  true,
		RiskyServerScoreFloor: 40,
	}
}
```

If `time` is not yet imported, add `"time"` to the import block.

Then add a `Lifecycle LifecycleConfig` field on the top-level `Config` struct, and seed it from the loader. Find the loader:

```bash
grep -n "func Load\|viper.SetDefault\|SetDefault.*[A-Z]" internal/core/config/config.go | head -20
```

If the codebase uses `viper.SetDefault`, add (next to other SetDefault lines):

```go
	v.SetDefault("lifecycle.enabled", true)
	v.SetDefault("lifecycle.interval", time.Hour)
	v.SetDefault("lifecycle.dry_run", true)
	v.SetDefault("lifecycle.inactive_key_days", 90)
	v.SetDefault("lifecycle.ownerless_keys_enabled", true)
	v.SetDefault("lifecycle.risky_server_score_floor", 40.0)
```

If the codebase uses zero-value structs filled by viper.Unmarshal, instead initialize `cfg.Lifecycle = DefaultLifecycleConfig()` *before* the unmarshal call so viper can override individual fields. Either pattern works — pick the one already used for, e.g., `cfg.MCP` or `cfg.Deployment`.

- [ ] **Step 5: Run test, verify pass**

Run: `go test ./internal/core/config/ -run TestDefaultLifecycleConfig -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/core/config/config.go internal/core/config/config_test.go
git commit -m "feat(config): add LifecycleConfig with DryRun=true install default"
```

---

## Task 3: Create lifecycle service skeleton + Rule interface

**Files:**
- Create: `internal/services/lifecycle/service.go`
- Create: `internal/services/lifecycle/service_test.go`

- [ ] **Step 1: Write failing test for empty service**

Create `internal/services/lifecycle/service_test.go`:

```go
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
	db := testutil.NewTestDB(t)
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
	db := testutil.NewTestDB(t)
	svc := NewService(db, zap.NewNop(), Config{DryRun: true})
	if _, err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := svc.LastSummary()
	if got.RanAt.IsZero() {
		t.Fatal("expected LastSummary.RanAt populated after RunOnce")
	}
}
```

- [ ] **Step 2: Run test, verify compile failure**

Run: `go test ./internal/services/lifecycle/ -v`
Expected: FAIL, "no Go files" or "undefined".

- [ ] **Step 3: Implement minimal Service + Rule interface**

Create `internal/services/lifecycle/service.go`:

```go
// Package lifecycle implements automated hygiene rules that run on a
// periodic ticker (and on-demand via the admin handler). Each Rule is a
// small struct that scans the database for stale state and either logs
// (DryRun) or mutates rows + writes Audit + emits a webhook.
package lifecycle

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config is the runtime view of internal/core/config.LifecycleConfig.
// Kept here as a plain struct so this package doesn't import the global
// config package (matches the rest of internal/services).
type Config struct {
	DryRun                bool
	Interval              time.Duration
	InactiveKeyDays       int
	OwnerlessKeysEnabled  bool
	RiskyServerScoreFloor float64
}

// WebhookEmitter is the small surface this package needs from
// webhooks.Dispatcher. Defined here so we don't import the concrete
// dispatcher type — same pattern as use_cases.WebhookEmitter.
type WebhookEmitter interface {
	Emit(typ string, data any)
}

// Action is one mutation taken by a rule, recorded in the run summary
// and returned to the admin UI. ResourceID is the row that was changed.
type Action struct {
	Rule       string    `json:"rule"`
	ResourceID string    `json:"resource_id"`
	Subject    string    `json:"subject"` // human-readable label (key prefix, server name)
	Reason     string    `json:"reason"`
	DryRun     bool      `json:"dry_run"`
	At         time.Time `json:"at"`
}

// RuleResult is what a single Rule returns from Run.
type RuleResult struct {
	Rule    string   `json:"rule"`
	Scanned int      `json:"scanned"`
	Actions []Action `json:"actions"`
	Err     string   `json:"error,omitempty"`
}

// RunSummary is the response to admin "run now" + the value cached on
// the service for "last status" queries.
type RunSummary struct {
	RanAt        time.Time    `json:"ran_at"`
	Duration     time.Duration `json:"duration"`
	TotalActions int          `json:"total_actions"`
	Rules        []RuleResult `json:"rules"`
}

// Rule is a single hygiene check. Implementations live in their own
// files in this package (inactive_keys.go, ownerless_keys.go, etc.).
type Rule interface {
	Name() string
	Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult
}

// Service holds the rule list and orchestrates ticker-driven runs.
// One instance per process; safe to call RunOnce concurrently with the
// background ticker (each run is its own DB transaction-per-action).
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	cfg    Config
	hooks  WebhookEmitter
	rules  []Rule

	mu          sync.RWMutex
	lastSummary RunSummary
}

// NewService builds a service. Rules are added with Register; this lets
// tests construct a service with a single rule.
func NewService(db *gorm.DB, logger *zap.Logger, cfg Config) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger.With(zap.String("component", "lifecycle")), cfg: cfg}
}

// WithWebhooks attaches an emitter. Returns the receiver for chaining.
func (s *Service) WithWebhooks(e WebhookEmitter) *Service {
	s.hooks = e
	return s
}

// Register appends a Rule to the run order. Idempotent across names —
// last registration wins.
func (s *Service) Register(r Rule) {
	for i, existing := range s.rules {
		if existing.Name() == r.Name() {
			s.rules[i] = r
			return
		}
	}
	s.rules = append(s.rules, r)
}

// RunOnce executes every rule in order, captures the summary, and
// returns it. Errors from a single rule are recorded on the result but
// don't abort other rules.
func (s *Service) RunOnce(ctx context.Context) (RunSummary, error) {
	start := time.Now()
	summary := RunSummary{RanAt: start, Rules: make([]RuleResult, 0, len(s.rules))}
	for _, r := range s.rules {
		res := r.Run(ctx, s.db, s.cfg, s.hooks)
		summary.Rules = append(summary.Rules, res)
		summary.TotalActions += len(res.Actions)
	}
	summary.Duration = time.Since(start)
	s.mu.Lock()
	s.lastSummary = summary
	s.mu.Unlock()
	s.logger.Info("lifecycle run complete",
		zap.Int("rules", len(summary.Rules)),
		zap.Int("actions", summary.TotalActions),
		zap.Duration("duration", summary.Duration),
		zap.Bool("dry_run", s.cfg.DryRun))
	return summary, nil
}

// LastSummary returns the most recent run summary (zero value if never run).
func (s *Service) LastSummary() RunSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSummary
}

// Start spawns the periodic ticker. Runs once immediately, then on
// Config.Interval. Stops when ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	if s.cfg.Interval <= 0 {
		s.logger.Warn("lifecycle disabled — Interval <= 0")
		return
	}
	go func() {
		t := time.NewTicker(s.cfg.Interval)
		defer t.Stop()
		// fire once at startup so a misconfigured prod doesn't sit stale for an hour
		_, _ = s.RunOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _ = s.RunOnce(ctx)
			}
		}
	}()
	s.logger.Info("lifecycle daemon started",
		zap.Duration("interval", s.cfg.Interval),
		zap.Bool("dry_run", s.cfg.DryRun))
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/services/lifecycle/ -v`
Expected: PASS, 2 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/lifecycle/service.go internal/services/lifecycle/service_test.go
git commit -m "feat(lifecycle): add service skeleton with Rule interface and run loop"
```

---

## Task 4: Implement InactiveKeysRule

**Files:**
- Create: `internal/services/lifecycle/inactive_keys.go`
- Create: `internal/services/lifecycle/inactive_keys_test.go`

- [ ] **Step 1: Write failing tests for the rule**

Create `internal/services/lifecycle/inactive_keys_test.go`:

```go
package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

// fakeEmitter records every Emit so tests can assert payloads.
type fakeEmitter struct {
	events []struct {
		typ  string
		data any
	}
}

func (f *fakeEmitter) Emit(typ string, data any) {
	f.events = append(f.events, struct {
		typ  string
		data any
	}{typ, data})
}

func TestInactiveKeysRevokesOldKeyInLiveMode(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db := testutil.NewTestDB(t)
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
	if emitter.events[0].typ != string(models.WebhookEventKeyRevokedInactive) {
		t.Fatalf("expected event type %q, got %q",
			models.WebhookEventKeyRevokedInactive, emitter.events[0].typ)
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
	db := testutil.NewTestDB(t)
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
	db := testutil.NewTestDB(t)
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
	db := testutil.NewTestDB(t)
	uid := uuid.New()
	old := time.Now().Add(-200 * 24 * time.Hour)
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "u4@x", Username: "u4", IsActive: true})
	db.Create(&models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		Key:        "pllm_ak_dead",
		KeyHash:    "hash_dead",
		KeyPrefix:  "dead",
		IsActive:   false, // already revoked
		UserID:     &uid,
		LastUsedAt: &old,
	})

	rule := &InactiveKeysRule{}
	res := rule.Run(context.Background(), db, Config{InactiveKeyDays: 90, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions on already-revoked, got %d", len(res.Actions))
	}
}
```

- [ ] **Step 2: Run tests, verify failure**

Run: `go test ./internal/services/lifecycle/ -run TestInactiveKeys -v`
Expected: FAIL — `InactiveKeysRule` undefined.

- [ ] **Step 3: Implement InactiveKeysRule**

Create `internal/services/lifecycle/inactive_keys.go`:

```go
package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// InactiveKeysRule revokes Keys whose LastUsedAt (or CreatedAt for
// never-used keys) is older than Config.InactiveKeyDays. Master keys
// are never touched by this rule — operators rotate them by hand.
type InactiveKeysRule struct{}

// Name is the canonical identifier in the run summary + audit row.
func (r *InactiveKeysRule) Name() string { return "inactive_keys" }

// Run scans candidate keys, takes action (or dry-runs), and returns the
// per-key Action list. Each mutation is its own UPDATE — no bulk update —
// so the audit and webhook side-effects stay aligned with the row state.
func (r *InactiveKeysRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name()}
	if cfg.InactiveKeyDays <= 0 {
		return res
	}

	cutoff := time.Now().Add(-time.Duration(cfg.InactiveKeyDays) * 24 * time.Hour)

	var keys []models.Key
	// "Inactive" = active row (IsActive=true), not master, not already revoked,
	// AND (LastUsedAt < cutoff OR (LastUsedAt is null AND CreatedAt < cutoff)).
	err := db.WithContext(ctx).
		Where("is_active = ? AND revoked_at IS NULL AND type <> ?", true, models.KeyTypeMaster).
		Where("(last_used_at IS NOT NULL AND last_used_at < ?) OR (last_used_at IS NULL AND created_at < ?)",
			cutoff, cutoff).
		Find(&keys).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(keys)

	for i := range keys {
		k := keys[i]
		reason := fmt.Sprintf("no activity since %s (threshold=%d days)",
			lastSeen(&k), cfg.InactiveKeyDays)
		act := Action{
			Rule:       r.Name(),
			ResourceID: k.ID.String(),
			Subject:    fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         time.Now(),
		}
		res.Actions = append(res.Actions, act)
		if cfg.DryRun {
			continue
		}
		if err := revokeKey(ctx, db, &k, reason); err != nil {
			// best-effort: log via Action.Reason and move on
			res.Actions[len(res.Actions)-1].Reason = "revoke failed: " + err.Error()
			continue
		}
		writeAudit(ctx, db, &k, r.Name(), reason)
		if hooks != nil {
			hooks.Emit(string(models.WebhookEventKeyRevokedInactive), map[string]any{
				"key_id":     k.ID,
				"key_prefix": k.KeyPrefix,
				"name":       k.Name,
				"user_id":    k.UserID,
				"team_id":    k.TeamID,
				"reason":     reason,
				"occurred_at": time.Now().UTC(),
			})
		}
	}
	return res
}

// revokeKey is the shared mutation used by all key-revoking rules.
// We don't call k.Revoke() because that requires a userID we don't have
// — lifecycle is system-driven; the audit row carries the "by whom".
func revokeKey(ctx context.Context, db *gorm.DB, k *models.Key, reason string) error {
	now := time.Now()
	return db.WithContext(ctx).Model(&models.Key{}).
		Where("id = ?", k.ID).
		Updates(map[string]any{
			"is_active":         false,
			"revoked_at":        now,
			"revocation_reason": reason,
		}).Error
}

// writeAudit persists a single AuditEventLifecycle row keyed to the
// affected resource. For Keys we set KeyID; ResourceType and ResourceID
// give callers a generic shape that works for non-key resources too.
func writeAudit(ctx context.Context, db *gorm.DB, k *models.Key, action, reason string) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("key_%s", action),
		EventResult:  models.AuditResult("success"),
		KeyID:        &k.ID,
		ResourceType: "key",
		ResourceID:   &k.ID,
		Message:      reason,
		Timestamp:    time.Now(),
	}
	if k.UserID != nil {
		row.UserID = k.UserID
	}
	// New values is the post-state of the row.
	if b, err := json.Marshal(map[string]any{"is_active": false, "revoked_at": time.Now()}); err == nil {
		row.NewValues = b
	}
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		// audit is best-effort; intentionally silent
		_ = err
	}
}

// lastSeen returns a human-readable last-activity hint for the audit message.
func lastSeen(k *models.Key) string {
	if k.LastUsedAt != nil {
		return k.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return "never"
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/services/lifecycle/ -run TestInactiveKeys -v`
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/lifecycle/inactive_keys.go internal/services/lifecycle/inactive_keys_test.go
git commit -m "feat(lifecycle): add InactiveKeysRule with dry-run support and audit trail"
```

---

## Task 5: Implement OwnerlessKeysRule

**Files:**
- Create: `internal/services/lifecycle/ownerless_keys.go`
- Create: `internal/services/lifecycle/ownerless_keys_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/lifecycle/ownerless_keys_test.go`:

```go
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
	db := testutil.NewTestDB(t)
	uid := uuid.New()
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "off@x", Username: "off", IsActive: false})
	key := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_owner",
		KeyHash:   "hash_owner",
		KeyPrefix: "owner",
		IsActive:  true,
		UserID:    &uid,
	}
	db.Create(&key)

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
	if len(emitter.events) != 1 ||
		emitter.events[0].typ != string(models.WebhookEventKeyRevokedOwnerless) {
		t.Fatalf("expected one ownerless event, got %+v", emitter.events)
	}
}

func TestOwnerlessKeysRevokesKeyOfDeletedUser(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db := testutil.NewTestDB(t)
	uid := uuid.New()
	user := models.User{BaseModel: models.BaseModel{ID: uid}, Email: "del@x", Username: "del", IsActive: true}
	db.Create(&user)
	now := time.Now()
	// soft-delete via DeletedAt
	db.Model(&models.User{}).Where("id = ?", uid).Update("deleted_at", now)

	key := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_del",
		KeyHash:   "hash_del",
		KeyPrefix: "del",
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
	db := testutil.NewTestDB(t)
	tid := uuid.New()
	db.Create(&models.Team{BaseModel: models.BaseModel{ID: tid}, Name: "T"})
	db.Create(&models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_team",
		KeyHash:   "hash_team",
		KeyPrefix: "team",
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
	db := testutil.NewTestDB(t)
	uid := uuid.New()
	db.Create(&models.User{BaseModel: models.BaseModel{ID: uid}, Email: "x@x", Username: "x", IsActive: false})
	db.Create(&models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Key:       "pllm_ak_off",
		KeyHash:   "hash_off",
		KeyPrefix: "off2",
		IsActive:  true,
		UserID:    &uid,
	})
	rule := &OwnerlessKeysRule{}
	res := rule.Run(context.Background(), db, Config{OwnerlessKeysEnabled: false, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("expected disabled rule to no-op, got %d actions", len(res.Actions))
	}
}
```

- [ ] **Step 2: Run tests, verify failure**

Run: `go test ./internal/services/lifecycle/ -run TestOwnerless -v`
Expected: FAIL, `OwnerlessKeysRule` undefined.

- [ ] **Step 3: Implement OwnerlessKeysRule**

Create `internal/services/lifecycle/ownerless_keys.go`:

```go
package lifecycle

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// OwnerlessKeysRule revokes Keys whose user has IsActive=false OR has
// been soft-deleted. Team-owned keys with no UserID are NOT touched —
// they're owned by the team, not by a user. Master keys are skipped.
type OwnerlessKeysRule struct{}

func (r *OwnerlessKeysRule) Name() string { return "ownerless_keys" }

func (r *OwnerlessKeysRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name()}
	if !cfg.OwnerlessKeysEnabled {
		return res
	}

	type row struct {
		Key      models.Key
		UserNil  bool   // true when the LEFT JOIN returned no user
		Inactive bool
	}

	// Use Unscoped on User so soft-deleted rows still join — we want to
	// detect deleted users. Then check IsActive AND whether DeletedAt was
	// set, in Go.
	type joined struct {
		KeyID         string     `gorm:"column:key_id"`
		Type          string     `gorm:"column:type"`
		IsActive      bool       `gorm:"column:is_active"`
		KeyName       string     `gorm:"column:name"`
		KeyPrefix     string     `gorm:"column:key_prefix"`
		UserID        *string    `gorm:"column:user_id"`
		TeamID        *string    `gorm:"column:team_id"`
		UserActive    *bool      `gorm:"column:user_active"`
		UserDeletedAt *time.Time `gorm:"column:user_deleted_at"`
	}
	var rows []joined
	err := db.WithContext(ctx).Raw(`
		SELECT k.id::text AS key_id, k.type, k.is_active, k.name, k.key_prefix,
		       k.user_id::text AS user_id, k.team_id::text AS team_id,
		       u.is_active AS user_active, u.deleted_at AS user_deleted_at
		FROM keys k
		LEFT JOIN users u ON u.id = k.user_id
		WHERE k.is_active = true
		  AND k.revoked_at IS NULL
		  AND k.type <> ?
		  AND k.user_id IS NOT NULL
	`, models.KeyTypeMaster).Scan(&rows).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(rows)

	for _, r2 := range rows {
		ownerless := false
		why := ""
		switch {
		case r2.UserDeletedAt != nil:
			ownerless = true
			why = "owner user soft-deleted"
		case r2.UserActive != nil && !*r2.UserActive:
			ownerless = true
			why = "owner user inactive"
		case r2.UserActive == nil:
			// LEFT JOIN found no user row at all — orphaned reference
			ownerless = true
			why = "owner user missing"
		}
		if !ownerless {
			continue
		}

		var k models.Key
		if err := db.WithContext(ctx).First(&k, "id = ?", r2.KeyID).Error; err != nil {
			continue
		}
		reason := fmt.Sprintf("ownerless: %s", why)
		res.Actions = append(res.Actions, Action{
			Rule:       r.Name(),
			ResourceID: k.ID.String(),
			Subject:    fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         time.Now(),
		})
		if cfg.DryRun {
			continue
		}
		if err := revokeKey(ctx, db, &k, reason); err != nil {
			res.Actions[len(res.Actions)-1].Reason = "revoke failed: " + err.Error()
			continue
		}
		writeAudit(ctx, db, &k, r.Name(), reason)
		if hooks != nil {
			hooks.Emit(string(models.WebhookEventKeyRevokedOwnerless), map[string]any{
				"key_id":      k.ID,
				"key_prefix":  k.KeyPrefix,
				"name":        k.Name,
				"user_id":     k.UserID,
				"team_id":     k.TeamID,
				"reason":      reason,
				"occurred_at": time.Now().UTC(),
			})
		}
	}
	return res
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/services/lifecycle/ -run TestOwnerless -v`
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/lifecycle/ownerless_keys.go internal/services/lifecycle/ownerless_keys_test.go
git commit -m "feat(lifecycle): add OwnerlessKeysRule for inactive/deleted owner detection"
```

---

## Task 6: Implement RiskyServersRule

**Files:**
- Create: `internal/services/lifecycle/risky_servers.go`
- Create: `internal/services/lifecycle/risky_servers_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/lifecycle/risky_servers_test.go`:

```go
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
	db := testutil.NewTestDB(t)
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
	if len(emitter.events) != 1 ||
		emitter.events[0].typ != string(models.WebhookEventRegistryServerBlockedRisky) {
		t.Fatalf("expected risky-block webhook, got %+v", emitter.events)
	}
}

func TestRiskyServersSkipsAcceptableScore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db := testutil.NewTestDB(t)
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
	db := testutil.NewTestDB(t)
	srv := models.RegistryServer{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "io.pllm.test/blocked",
		Version:     "1.0.0",
		Status:      models.RegistryStatusBlocked, // already blocked
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
	db := testutil.NewTestDB(t)
	rule := &RiskyServersRule{}
	res := rule.Run(context.Background(), db, Config{RiskyServerScoreFloor: 0, DryRun: false}, &fakeEmitter{})
	if len(res.Actions) != 0 {
		t.Fatalf("floor=0 must disable rule, got %d actions", len(res.Actions))
	}
}
```

- [ ] **Step 2: Run tests, verify failure**

Run: `go test ./internal/services/lifecycle/ -run TestRiskyServers -v`
Expected: FAIL, `RiskyServersRule` undefined.

- [ ] **Step 3: Implement RiskyServersRule**

Create `internal/services/lifecycle/risky_servers.go`:

```go
package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// RiskyServersRule sets RegistryServer.Status to "blocked" when the
// latest OSV EnrichmentScore is below Config.RiskyServerScoreFloor.
// Operators clear the block manually after triage; we never auto-unblock.
type RiskyServersRule struct{}

func (r *RiskyServersRule) Name() string { return "risky_servers" }

func (r *RiskyServersRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name()}
	if cfg.RiskyServerScoreFloor <= 0 {
		return res
	}

	// Join: every active server with its latest OSV score below floor.
	type row struct {
		ServerID uuid.UUID `gorm:"column:server_id"`
		Name     string    `gorm:"column:name"`
		Version  string    `gorm:"column:version"`
		Score    float64   `gorm:"column:score"`
		Summary  string    `gorm:"column:summary"`
	}
	var rows []row
	err := db.WithContext(ctx).Raw(`
		SELECT s.id AS server_id, s.name, s.version, e.score, e.summary
		FROM registry_servers s
		JOIN enrichment_scores e
		  ON e.resource_kind = ?
		 AND e.resource_id   = s.id
		 AND e.type          = ?
		WHERE s.status = ?
		  AND e.score < ?
	`, models.RegistryKindServer, models.EnrichmentTypeOSV,
		models.RegistryStatusActive, cfg.RiskyServerScoreFloor).
		Scan(&rows).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(rows)

	for _, x := range rows {
		reason := fmt.Sprintf("OSV score %.1f below floor %.1f: %s",
			x.Score, cfg.RiskyServerScoreFloor, x.Summary)
		res.Actions = append(res.Actions, Action{
			Rule:       r.Name(),
			ResourceID: x.ServerID.String(),
			Subject:    fmt.Sprintf("%s@%s", x.Name, x.Version),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         time.Now(),
		})
		if cfg.DryRun {
			continue
		}
		if err := db.WithContext(ctx).Model(&models.RegistryServer{}).
			Where("id = ?", x.ServerID).
			Update("status", models.RegistryStatusBlocked).Error; err != nil {
			res.Actions[len(res.Actions)-1].Reason = "block failed: " + err.Error()
			continue
		}
		writeServerAudit(ctx, db, x.ServerID, x.Name, x.Version, r.Name(), reason)
		if hooks != nil {
			hooks.Emit(string(models.WebhookEventRegistryServerBlockedRisky), map[string]any{
				"server_id":   x.ServerID,
				"name":        x.Name,
				"version":     x.Version,
				"score":       x.Score,
				"floor":       cfg.RiskyServerScoreFloor,
				"reason":      reason,
				"occurred_at": time.Now().UTC(),
			})
		}
	}
	return res
}

func writeServerAudit(ctx context.Context, db *gorm.DB, sid uuid.UUID, name, version, action, reason string) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("server_%s", action),
		EventResult:  models.AuditResult("success"),
		ResourceType: "registry_server",
		ResourceID:   &sid,
		Message:      reason,
		Timestamp:    time.Now(),
	}
	if b, err := json.Marshal(map[string]any{
		"name": name, "version": version, "status": "blocked",
	}); err == nil {
		row.NewValues = b
	}
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		_ = err
	}
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/services/lifecycle/ -run TestRiskyServers -v`
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/lifecycle/risky_servers.go internal/services/lifecycle/risky_servers_test.go
git commit -m "feat(lifecycle): add RiskyServersRule using OSV score floor"
```

---

## Task 7: Add admin handler with /run and /status endpoints

**Files:**
- Create: `internal/api/handlers/lifecycle/handler.go`
- Create: `internal/api/handlers/lifecycle/handler_test.go`

- [ ] **Step 1: Write failing handler test**

Create `internal/api/handlers/lifecycle/handler_test.go`:

```go
package lifecycle

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	lifesvc "github.com/amerfu/pllm/internal/services/lifecycle"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestStatusReturnsZeroSummaryBeforeAnyRun(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db := testutil.NewTestDB(t)
	svc := lifesvc.NewService(db, zap.NewNop(), lifesvc.Config{DryRun: true})
	h := NewHandler(zap.NewNop(), svc)

	r := chi.NewRouter()
	r.Get("/status", h.Status)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got lifesvc.RunSummary
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.TotalActions != 0 {
		t.Fatalf("expected 0 actions on first read, got %d", got.TotalActions)
	}
}

func TestRunNowExecutesRulesAndReturnsSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db := testutil.NewTestDB(t)
	svc := lifesvc.NewService(db, zap.NewNop(), lifesvc.Config{DryRun: true, InactiveKeyDays: 90})
	svc.Register(&lifesvc.InactiveKeysRule{})
	h := NewHandler(zap.NewNop(), svc)

	r := chi.NewRouter()
	r.Post("/run", h.RunNow)

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var got lifesvc.RunSummary
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(got.Rules))
	}
	if got.Rules[0].Rule != "inactive_keys" {
		t.Fatalf("expected inactive_keys rule, got %q", got.Rules[0].Rule)
	}
}
```

- [ ] **Step 2: Run test, verify failure**

Run: `go test ./internal/api/handlers/lifecycle/ -v`
Expected: FAIL, package is empty.

- [ ] **Step 3: Implement the handler**

Create `internal/api/handlers/lifecycle/handler.go`:

```go
// Package lifecycle exposes the lifecycle daemon over the admin API.
// Two endpoints: GET /status returns the cached last run, POST /run
// triggers a synchronous run and returns its summary.
package lifecycle

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	lifesvc "github.com/amerfu/pllm/internal/services/lifecycle"
)

// Handler is the HTTP edge for the lifecycle service. nil service is
// allowed at construction time (returns 503 from each handler) so the
// router can mount the routes regardless of feature gating.
type Handler struct {
	logger *zap.Logger
	svc    *lifesvc.Service
}

// NewHandler returns a Handler. Pass a nil service to disable.
func NewHandler(logger *zap.Logger, svc *lifesvc.Service) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

// Status returns the cached summary of the most recent run.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, "lifecycle disabled", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, h.svc.LastSummary())
}

// RunNow forces an out-of-band run and returns the resulting summary.
// Synchronous: callers should not invoke this on the request path of
// anything user-facing.
func (h *Handler) RunNow(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, "lifecycle disabled", http.StatusServiceUnavailable)
		return
	}
	summary, err := h.svc.RunOnce(r.Context())
	if err != nil {
		h.logger.Warn("lifecycle run failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/api/handlers/lifecycle/ -v`
Expected: PASS, 2 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/lifecycle/
git commit -m "feat(api): add admin handler for lifecycle status and run-now"
```

---

## Task 8: Wire lifecycle into router.go and admin.go

**Files:**
- Modify: `internal/api/router/router.go` (around line 360, near webhookDispatcher init)
- Modify: `internal/api/router/admin.go` (around line 280, after the use-cases block)

- [ ] **Step 1: Open router.go and confirm the integration anchor**

Run: `grep -n "useCaseHandler\|UseCase service initialized" internal/api/router/router.go`
You should see the block near line 369-383 that builds `useCaseHandler`. Insert the lifecycle service init **immediately after** that block.

- [ ] **Step 2: Add lifecycle imports to router.go**

In `internal/api/router/router.go`, add to the existing imports (alongside the other `internal/api/handlers/*` and `internal/services/*` imports):

```go
	lifecyclehandlers "github.com/amerfu/pllm/internal/api/handlers/lifecycle"
	lifecyclesvc "github.com/amerfu/pllm/internal/services/lifecycle"
```

- [ ] **Step 3: Add the webhookEmitterAdapter for lifecycle**

In `internal/api/router/router.go`, after the existing `webhookEmitterAdapter` for the use-case scope events (search for `// WebhookEmitter interface into the same webhooks.Dispatcher`), add a third adapter:

```go
// lifecycleWebhookAdapter bridges the lifecycle service's
// WebhookEmitter (which takes a string + any) onto webhooks.Dispatcher.
// String values are validated against models.WebhookEventType — unknown
// strings are emitted verbatim, which makes the dispatcher fan-out a
// no-op for any subscriber that filters by type.
type lifecycleWebhookAdapter struct {
	inner *webhooks.Dispatcher
}

// Emit converts a (typ, data) pair into a webhooks.Event.
func (a *lifecycleWebhookAdapter) Emit(typ string, data any) {
	if a == nil || a.inner == nil {
		return
	}
	a.inner.Emit(webhooks.Event{
		Type: models.WebhookEventType(typ),
		Data: data,
	})
}
```

If `models` isn't already imported in router.go, add `"github.com/amerfu/pllm/internal/core/models"` to the imports.

- [ ] **Step 4: Initialize the lifecycle service after the use-case service block**

After the closing brace of the `if db != nil { ucSvc := ... }` block in router.go (around line 383), add:

```go
	// Lifecycle hygiene daemon. Always-on when DB is present + feature
	// enabled in config; defaults to DryRun=true so a fresh deploy never
	// revokes anything before the operator reviews the audit log.
	var lifecycleHandler *lifecyclehandlers.Handler
	if db != nil && cfg.Lifecycle.Enabled {
		lifeCfg := lifecyclesvc.Config{
			DryRun:                cfg.Lifecycle.DryRun,
			Interval:              cfg.Lifecycle.Interval,
			InactiveKeyDays:       cfg.Lifecycle.InactiveKeyDays,
			OwnerlessKeysEnabled:  cfg.Lifecycle.OwnerlessKeysEnabled,
			RiskyServerScoreFloor: cfg.Lifecycle.RiskyServerScoreFloor,
		}
		lifeSvc := lifecyclesvc.NewService(db, logger, lifeCfg)
		lifeSvc.Register(&lifecyclesvc.InactiveKeysRule{})
		lifeSvc.Register(&lifecyclesvc.OwnerlessKeysRule{})
		lifeSvc.Register(&lifecyclesvc.RiskyServersRule{})
		if webhookDispatcher != nil {
			lifeSvc = lifeSvc.WithWebhooks(&lifecycleWebhookAdapter{inner: webhookDispatcher})
		}
		lifeSvc.Start(context.Background())
		lifecycleHandler = lifecyclehandlers.NewHandler(logger, lifeSvc)
		logger.Info("Lifecycle hygiene daemon initialized",
			zap.Bool("dry_run", lifeCfg.DryRun),
			zap.Duration("interval", lifeCfg.Interval))
	}
```

Then find the `Config{}` struct that this function returns or hands to `mountRoutes` (search for `cfg.UseCaseHandler` to find it) and add a `LifecycleHandler` field. Pass `lifecycleHandler` to it.

- [ ] **Step 5: Find the router config struct and add the field**

Run: `grep -n "UseCaseHandler\s*\*" internal/api/router/router.go internal/api/router/admin.go`
This locates the struct definition. Add to that struct:

```go
	LifecycleHandler *lifecyclehandlers.Handler
```

Add the import to that file too:

```go
	lifecyclehandlers "github.com/amerfu/pllm/internal/api/handlers/lifecycle"
```

- [ ] **Step 6: Mount the routes in admin.go**

In `internal/api/router/admin.go`, after the existing `r.Route("/use-cases", ...)` block (around line 270-285), add:

```go
		// Lifecycle hygiene admin endpoints. Always present (so the UI
		// can surface a "disabled" state); the handler itself returns
		// 503 when the service is nil.
		r.Route("/lifecycle", func(rr chi.Router) {
			rr.Get("/status", cfg.LifecycleHandler.Status)
			rr.Post("/run", cfg.LifecycleHandler.RunNow)
		})
```

Also, near the `systemHandler.SetFeature("webhooks", ...)` line (around line 73), add:

```go
	systemHandler.SetFeature("lifecycle", cfg.LifecycleHandler != nil)
```

(and add the equivalent line near line 410 if there's a second SetFeature block).

- [ ] **Step 7: Make sure the handler call doesn't panic when nil**

We must guard the chi route registration when `cfg.LifecycleHandler == nil` — chi will panic if you mount a nil HandlerFunc. Wrap the `r.Route("/lifecycle", ...)` block:

```go
		if cfg.LifecycleHandler != nil {
			r.Route("/lifecycle", func(rr chi.Router) {
				rr.Get("/status", cfg.LifecycleHandler.Status)
				rr.Post("/run", cfg.LifecycleHandler.RunNow)
			})
		}
```

- [ ] **Step 8: Build and confirm**

Run: `go build ./...`
Expected: success, no errors.

Run: `go test ./internal/services/lifecycle/ ./internal/api/handlers/lifecycle/ -v`
Expected: PASS for all tests in both packages.

- [ ] **Step 9: Commit**

```bash
git add internal/api/router/router.go internal/api/router/admin.go
git commit -m "feat(router): wire lifecycle daemon into server start + admin routes"
```

---

## Task 9: Frontend types and API client

**Files:**
- Modify: `web/src/types/api.ts` (add new types at end)
- Modify: `web/src/lib/api.ts` (add new fetch helpers)

- [ ] **Step 1: Look at existing API client style**

Run: `grep -n "useCases\|registry\|webhooks" web/src/lib/api.ts | head -20`
Note the calling convention (e.g. function names, request signatures) so the new helpers match.

- [ ] **Step 2: Add types to web/src/types/api.ts**

Append to `web/src/types/api.ts`:

```typescript
// --- Lifecycle hygiene -----------------------------------------------------
//
// Mirror of internal/services/lifecycle types. Kept here as an isolated
// block so removing the EE feature is a contiguous deletion.

export interface LifecycleAction {
  rule: string;
  resource_id: string;
  subject: string;
  reason: string;
  dry_run: boolean;
  at: string;
}

export interface LifecycleRuleResult {
  rule: string;
  scanned: number;
  actions: LifecycleAction[];
  error?: string;
}

export interface LifecycleSummary {
  ran_at: string;
  duration: number;
  total_actions: number;
  rules: LifecycleRuleResult[];
}
```

- [ ] **Step 3: Add fetch helpers to web/src/lib/api.ts**

Find an existing helper (e.g. `getWebhooks` or `getUseCases`) and follow the same pattern. Append:

```typescript
import type { LifecycleSummary } from "@/types/api";

// Returns the most recent lifecycle run summary. Empty (zero) summary
// before the daemon has run for the first time.
export async function getLifecycleStatus(): Promise<LifecycleSummary> {
  const res = await fetch("/api/admin/lifecycle/status", {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`getLifecycleStatus: ${res.status}`);
  return res.json();
}

// Triggers a synchronous run-now and returns its summary.
export async function runLifecycle(): Promise<LifecycleSummary> {
  const res = await fetch("/api/admin/lifecycle/run", {
    method: "POST",
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error(`runLifecycle: ${res.status}`);
  return res.json();
}
```

If `authHeaders` isn't the helper name in your codebase, look for the equivalent (`getAuthHeaders`, `withAuth`, etc.) by running `grep -n "Authorization" web/src/lib/api.ts` and use whatever pattern is already there.

- [ ] **Step 4: Type-check**

Run: `cd web && npm run lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/types/api.ts web/src/lib/api.ts
git commit -m "feat(web): add lifecycle types and admin API client helpers"
```

---

## Task 10: Add Hygiene tab to Compliance page

**Files:**
- Modify: `web/src/pages/Compliance.tsx`

- [ ] **Step 1: Inspect the existing tab layout**

Run: `grep -n "Tabs\|TabsList\|TabsTrigger\|TabsContent" web/src/pages/Compliance.tsx | head -30`
Find the existing tabs (Webhooks / Denial log / etc.) so the new tab matches.

- [ ] **Step 2: Add tab trigger + content block**

Inside the existing `<Tabs>` block in `web/src/pages/Compliance.tsx`, add a `<TabsTrigger value="hygiene">Hygiene</TabsTrigger>` next to the existing triggers, and a matching `<TabsContent value="hygiene">` block. Use a small component defined in the same file:

```tsx
function HygieneTab() {
  const [summary, setSummary] = React.useState<LifecycleSummary | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      setErr(null);
      const s = await getLifecycleStatus();
      setSummary(s);
    } catch (e: any) {
      setErr(e.message ?? "failed");
    }
  }, []);

  const runNow = React.useCallback(async () => {
    try {
      setLoading(true);
      setErr(null);
      const s = await runLifecycle();
      setSummary(s);
    } catch (e: any) {
      setErr(e.message ?? "failed");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="font-mono text-xs uppercase tracking-widest text-muted-foreground">
            Lifecycle Hygiene
          </p>
          <h2 className="text-2xl">
            Automatic <span className="font-serif italic">cleanup</span>
          </h2>
          <p className="text-sm text-muted-foreground mt-1">
            Inactive keys, ownerless keys, and risky servers — reviewed every
            run. Dry-run mode logs without mutating.
          </p>
        </div>
        <Button onClick={runNow} disabled={loading}>
          {loading ? "Running…" : "Run now"}
        </Button>
      </div>

      {err && <div className="text-sm text-rose-600">Error: {err}</div>}

      {summary && summary.ran_at && (
        <div className="text-xs text-muted-foreground">
          Last run {new Date(summary.ran_at).toLocaleString()} ·{" "}
          {summary.total_actions} action{summary.total_actions === 1 ? "" : "s"} ·{" "}
          {Math.round(summary.duration / 1_000_000)}ms
        </div>
      )}

      {summary?.rules?.map((r) => (
        <Card key={r.rule}>
          <CardHeader>
            <CardTitle className="text-base font-mono uppercase tracking-wide">
              {r.rule}
            </CardTitle>
            <CardDescription>
              scanned {r.scanned}, acted on {r.actions.length}
              {r.error ? ` · error: ${r.error}` : ""}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {r.actions.length === 0 ? (
              <p className="text-sm text-muted-foreground">No actions.</p>
            ) : (
              <ul className="divide-y">
                {r.actions.map((a) => (
                  <li key={a.resource_id} className="py-2 text-sm">
                    <div className="flex items-center justify-between">
                      <span className="font-mono">{a.subject}</span>
                      {a.dry_run && (
                        <span className="text-xs uppercase text-amber-600">
                          dry-run
                        </span>
                      )}
                    </div>
                    <div className="text-muted-foreground text-xs">
                      {a.reason}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
```

Add the imports at the top of the file:

```tsx
import { getLifecycleStatus, runLifecycle } from "@/lib/api";
import type { LifecycleSummary } from "@/types/api";
```

(If `Button`, `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent` aren't already imported in Compliance.tsx, add them from `@/components/ui/...` matching the rest of the file's imports.)

Mount the tab content:

```tsx
<TabsContent value="hygiene">
  <HygieneTab />
</TabsContent>
```

- [ ] **Step 3: Build the frontend**

Run: `cd web && npm run build`
Expected: success.

Run: `cd web && npm run lint`
Expected: PASS.

- [ ] **Step 4: Manual smoke (optional)**

Start `make dev`, log in, navigate to `/compliance`, click the Hygiene tab. Should render even with no run history (zero-summary state). Click "Run now" — summary populates.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/Compliance.tsx
git commit -m "feat(web): add Hygiene tab on Compliance page with run-now button"
```

---

## Task 11: Extend the smoke test script

**Files:**
- Modify: `scripts/test-mcp-registry.sh`

- [ ] **Step 1: Open the script and locate the "// END" marker**

Run: `tail -40 scripts/test-mcp-registry.sh`
Find the final exit / cleanup section. Append the new lifecycle tests **before** any cleanup line.

- [ ] **Step 2: Append the lifecycle stage**

Append to `scripts/test-mcp-registry.sh` (just before the closing summary/exit):

```bash
# ----------------------------------------------------------------------
# Lifecycle hygiene smoke
# ----------------------------------------------------------------------
echo
echo "=== lifecycle hygiene ==="

# Status before any run — should be 200 with empty summary or last-known.
STATUS=$(curl -sS -o /tmp/lc_status.json -w "%{http_code}" \
  -H "Authorization: Bearer ${MASTER_KEY}" \
  "${BASE_URL}/api/admin/lifecycle/status")
[[ "$STATUS" == "200" ]] || { echo "FAIL: lifecycle status got $STATUS"; cat /tmp/lc_status.json; exit 1; }
echo "lifecycle status OK"

# Force a run. Must return 200 and a JSON summary with a rules array.
RUN=$(curl -sS -o /tmp/lc_run.json -w "%{http_code}" \
  -X POST \
  -H "Authorization: Bearer ${MASTER_KEY}" \
  "${BASE_URL}/api/admin/lifecycle/run")
[[ "$RUN" == "200" ]] || { echo "FAIL: lifecycle run got $RUN"; cat /tmp/lc_run.json; exit 1; }

# Must contain at least the three known rules.
for rule in inactive_keys ownerless_keys risky_servers; do
  if ! grep -q "\"rule\":\"$rule\"" /tmp/lc_run.json; then
    echo "FAIL: lifecycle run missing rule $rule"
    cat /tmp/lc_run.json
    exit 1
  fi
done
echo "lifecycle run OK — all three rules executed"
```

- [ ] **Step 3: Run the smoke test against a local stack**

Run: `make docker-up && make run` in one terminal, then in another:
`bash scripts/test-mcp-registry.sh`
Expected: existing tests pass + new "lifecycle hygiene" stage prints OK.

If you don't have a local stack right now, skip the live run — CI will pick it up — but verify the bash syntax is sane:
`bash -n scripts/test-mcp-registry.sh`

- [ ] **Step 4: Commit**

```bash
git add scripts/test-mcp-registry.sh
git commit -m "test(scripts): smoke-test lifecycle status and run-now endpoints"
```

---

## Task 12: Update CLAUDE.md to document the new subsystem

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add a Lifecycle subsystem note**

In `CLAUDE.md`, in the "Subsystem notes" section (after the Webhooks block), append:

```markdown
### Lifecycle (`internal/services/lifecycle`)

Periodic hygiene daemon, started in-process from `router.go`. Three rules:

- **InactiveKeysRule** — revokes keys whose `LastUsedAt` (or `CreatedAt` for
  never-used) is older than `cfg.Lifecycle.InactiveKeyDays` (default 90).
  Master keys are exempt.
- **OwnerlessKeysRule** — revokes keys whose owner User has `IsActive=false`
  or has been soft-deleted. Team-only keys are not affected.
- **RiskyServersRule** — sets `RegistryServer.Status = "blocked"` when the
  latest OSV `EnrichmentScore` is below `cfg.Lifecycle.RiskyServerScoreFloor`
  (default 40). Operators clear the block manually.

Each action writes one `models.Audit` row (`event_type=lifecycle`) and emits
a webhook (`key.revoked.inactive`, `key.revoked.ownerless`,
`registry.server.blocked.risky`). `cfg.Lifecycle.DryRun=true` (the install
default) logs without mutating.

Admin: `GET /api/admin/lifecycle/status`, `POST /api/admin/lifecycle/run`.
UI: "Hygiene" tab on the Compliance page.
```

Also, in the "API surface" section, under the Admin block, append:

```markdown
- `/api/admin/lifecycle/status` — last hygiene run summary
- `/api/admin/lifecycle/run` — force a hygiene run
```

And in the "EE positioning" section, add lifecycle to the list of self-contained packages so the EE-gate guidance covers it.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): document lifecycle hygiene subsystem"
```

---

## Verification before declaring done

- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/services/lifecycle/ ./internal/api/handlers/lifecycle/ ./internal/core/models/ ./internal/core/config/ -v` all pass
- [ ] `cd web && npm run build && npm run lint` both succeed
- [ ] Manual: run `make dev`, hit `/api/admin/lifecycle/status` with the master key, get a 200
- [ ] Manual: hit `POST /api/admin/lifecycle/run`, see all three rules in the response
- [ ] Manual: open `/compliance` → Hygiene tab, "Run now" populates the panels

---

## Spec coverage check

| MS-Agent-365 bullet | Implemented by |
|---|---|
| "Expire inactive agents" | InactiveKeysRule (Task 4) |
| "Flag ownerless agents" | OwnerlessKeysRule (Task 5) — currently revokes; flag-only mode achievable via DryRun=true |
| "Block risky agents" | RiskyServersRule (Task 6) |
| Lifecycle policy templates / IT control | DryRun config + Admin "Run now" (Task 7) |
| Audit/visibility for hygiene actions | Audit table writes (Tasks 4–6) + Hygiene UI tab (Task 10) |
| Outbound to SIEM | Three new webhook event types fire on every action (Tasks 1, 4–6) |

Out of scope for this plan (deferred):
- Per-rule schedule overrides — single global interval is enough for v1
- Auto-unblock when an OSV score recovers — operators clear blocks manually
- Email/Slack-shaped digests — webhooks already enable any subscriber
- A standalone `LifecycleEvent` table — Audit covers the query shape

