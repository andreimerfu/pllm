# Bank Demo Seed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a one-command `pllm seed bank` that materializes a bank-MRM scenario (4 use cases, agents/prompts/skills, API keys, 7-day scope-event history, optional live MCP deployments + webhook) in any pllm install — locally or on K8s — using the public REST API.

**Architecture:** New Cobra subcommand under `cmd/cli/commands/seed/` that drives the existing admin REST API. Idempotent (GET-then-POST/PATCH per slug). One new admin endpoint, `POST /api/admin/use-cases/events:bulk`, lets the CLI backdate scope-event history without coupling to Postgres. Keeps the contract that ops never need DB credentials, only the master key.

**Tech Stack:** Go 1.25, Cobra (already in `cmd/cli/`), `net/http` standard client, GORM-backed `EventRecorder` for the new bulk endpoint, existing `models.*` types, kubectl Job manifest.

---

## File Structure

```
cmd/cli/commands/seed.go                      # NEW: cobra wiring (`pllm seed bank`)
cmd/cli/commands/seed/                        # NEW: seed package
  client.go                                   # thin REST client w/ master-key auth + cached UUID lookup
  fixtures.go                                 # bank cast as Go literals (no logic)
  bank.go                                     # main run loop: ensure each entity in dependency order
  events.go                                   # 7-day scope-event history generator
  cleanup.go                                  # remove `bank.demo/*` and `demo-*` fixtures
  print.go                                    # human/json output helpers
  bank_test.go                                # unit tests for fixtures + idempotency helpers
cmd/cli/main.go                               # MODIFY: register the seed subcommand
internal/api/handlers/use_cases/scope_events.go  # NEW: BulkSeed handler (split out of handler.go)
internal/api/handlers/use_cases/handler.go    # MODIFY: move ListScopeEvents into the new file
internal/api/router/admin.go                  # MODIFY: route POST /use-cases/events:bulk
internal/services/use_cases/events.go         # MODIFY: add BulkInsert(events) on EventRecorder
internal/services/use_cases/events_test.go    # MODIFY: add bulk-insert test
deploy/k8s/demo-seed-job.yaml                 # NEW: one-shot K8s Job manifest
Makefile                                      # MODIFY: add demo-seed, demo-seed-clean targets
scripts/demo-seed.sh                          # MODIFY: thin wrapper preferring `pllm seed bank`
```

The seed package is split by responsibility, not layer: `fixtures.go` is pure data, `client.go` is HTTP, `bank.go` orchestrates. `events.go` and `cleanup.go` are big enough that they earn their own files. The `cmd/cli/commands/seed.go` shell stays tiny — it only registers the cobra subcommand and parses flags.

---

## Task 1: Add `BulkInsert` to `EventRecorder`

**Files:**
- Modify: `internal/services/use_cases/events.go`
- Test: `internal/services/use_cases/events_test.go`

- [ ] **Step 1: Write the failing test**

Open `internal/services/use_cases/events_test.go`. After the existing
`TestRecordAndListScopeEvents` test, add:

```go
func TestBulkInsertScopeEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("requires testcontainers postgres")
	}
	db, cleanup := testutil.NewPostgresDB(t)
	defer cleanup()

	uc := &models.UseCase{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "bulk test",
		Slug:      "bulk-test",
		Status:    models.UseCaseStatusApproved,
		RiskTier:  models.UseCaseRiskTier1,
	}
	require.NoError(t, db.Create(uc).Error)

	rec := use_cases.NewEventRecorder(db, nil)
	now := time.Now().UTC()
	in := []use_cases.BulkRecordInput{
		{
			UseCaseID:  uc.ID,
			Kind:       models.ScopeEventKindLLM,
			Outcome:    models.ScopeOutcomeDenied,
			TargetName: "gpt-4-turbo",
			KeyName:    "demo-key",
			Reason:     "test reason",
			OccurredAt: now.Add(-48 * time.Hour),
		},
		{
			UseCaseID:  uc.ID,
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/use_cases/ -run TestBulkInsertScopeEvents -v`
Expected: FAIL — `rec.BulkInsert undefined` and `BulkRecordInput undefined`.

- [ ] **Step 3: Add `BulkInsert` implementation**

Open `internal/services/use_cases/events.go`. After the `Record` method
and before the `--- Query side ---` separator, add:

```go
// BulkRecordInput is the seed-time variant of RecordInput. The seed
// caller chooses the timestamp; runtime callers do not.
type BulkRecordInput struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	KeyName    string
	UserID     *uuid.UUID
	Kind       models.ScopeEventKind
	TargetName string
	Outcome    models.ScopeEventOutcome
	Reason     string
	RemoteAddr string
	RequestID  string
	OccurredAt time.Time
}

// BulkInsert writes scope events with caller-supplied timestamps. Used
// only by the demo seed (`pllm seed bank --with-events`). Webhook fan-out
// is intentionally skipped — backdated history would generate misleading
// "live" notifications for any subscribed endpoints.
func (r *EventRecorder) BulkInsert(ctx context.Context, in []BulkRecordInput) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("event recorder not configured")
	}
	if len(in) == 0 {
		return 0, nil
	}
	rows := make([]models.ScopeEvent, 0, len(in))
	for _, e := range in {
		if e.UseCaseID == uuid.Nil {
			return 0, fmt.Errorf("bulk insert: use_case_id is required for every event")
		}
		ts := e.OccurredAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		rows = append(rows, models.ScopeEvent{
			BaseModel:  models.BaseModel{ID: uuid.New(), CreatedAt: ts, UpdatedAt: ts},
			UseCaseID:  e.UseCaseID,
			KeyID:      e.KeyID,
			KeyName:    e.KeyName,
			UserID:     e.UserID,
			Kind:       e.Kind,
			TargetName: e.TargetName,
			Outcome:    e.Outcome,
			Reason:     e.Reason,
			RemoteAddr: e.RemoteAddr,
			RequestID:  e.RequestID,
		})
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}
```

Add `"fmt"` and `"time"` to the imports if missing (they already are —
verify).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/use_cases/ -run TestBulkInsertScopeEvents -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/use_cases/events.go internal/services/use_cases/events_test.go
git commit -m "feat(use-cases): add BulkInsert on EventRecorder

Backdated scope-event writes for the demo seed. Skips webhook fan-out
because historical events should not generate 'live' notifications to
subscribers."
```

---

## Task 2: Add `BulkSeed` admin endpoint

**Files:**
- Create: `internal/api/handlers/use_cases/scope_events.go`
- Modify: `internal/api/handlers/use_cases/handler.go` (move `ListScopeEvents` out)
- Modify: `internal/api/router/admin.go` (route registration)

- [ ] **Step 1: Move `ListScopeEvents` into the new file**

Create `internal/api/handlers/use_cases/scope_events.go`:

```go
package use_cases

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

// ListScopeEvents is GET /api/admin/use-cases/events — the audit log
// across all use cases. Supports filters: use_case_id, kind (mcp_tool|llm),
// outcome (denied|flagged), target_name, since (RFC3339), limit/offset.
func (h *Handler) ListScopeEvents(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	q := r.URL.Query()
	f := use_cases.EventListFilter{
		Kind:       models.ScopeEventKind(q.Get("kind")),
		Outcome:    models.ScopeEventOutcome(q.Get("outcome")),
		TargetName: q.Get("target_name"),
	}
	if v := q.Get("use_case_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.UseCaseID = &id
		}
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = &t
		}
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n >= 0 {
		f.Offset = n
	}
	rec := h.service.Events()
	events, err := rec.ListEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats, err := rec.Stats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []models.ScopeEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"stats":  stats,
	})
}

// bulkSeedBody is the shape `POST /api/admin/use-cases/events:bulk` accepts.
// Each event must specify use_case_id; created_at is optional (defaults
// to now).
type bulkSeedBody struct {
	Events []bulkSeedEvent `json:"events"`
}

type bulkSeedEvent struct {
	UseCaseID  uuid.UUID `json:"use_case_id"`
	KeyID      *uuid.UUID `json:"key_id,omitempty"`
	KeyName    string    `json:"key_name,omitempty"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Kind       string    `json:"kind"`
	TargetName string    `json:"target_name"`
	Outcome    string    `json:"outcome"`
	Reason     string    `json:"reason,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

// BulkSeedScopeEvents is POST /api/admin/use-cases/events:bulk — a
// test/demo aid that backdates scope-event history. Master-key only
// (RequireAdmin upstream). Skips webhook fan-out so historical events
// don't generate live notifications.
func (h *Handler) BulkSeedScopeEvents(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	var body bulkSeedBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.Events) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"inserted": 0})
		return
	}
	in := make([]use_cases.BulkRecordInput, 0, len(body.Events))
	for _, e := range body.Events {
		in = append(in, use_cases.BulkRecordInput{
			UseCaseID:  e.UseCaseID,
			KeyID:      e.KeyID,
			KeyName:    e.KeyName,
			UserID:     e.UserID,
			Kind:       models.ScopeEventKind(e.Kind),
			TargetName: e.TargetName,
			Outcome:    models.ScopeEventOutcome(e.Outcome),
			Reason:     e.Reason,
			RemoteAddr: e.RemoteAddr,
			RequestID:  e.RequestID,
			OccurredAt: e.CreatedAt,
		})
	}
	n, err := h.service.Events().BulkInsert(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"inserted": n})
}
```

- [ ] **Step 2: Remove `ListScopeEvents` from `handler.go`**

Open `internal/api/handlers/use_cases/handler.go`. Delete the entire
`ListScopeEvents` function block (lines 92-137 in the current file).
Remove `"strconv"` and `"time"` imports if they become unused — Go will
fail the build otherwise.

Run `goimports -w internal/api/handlers/use_cases/handler.go` to clean
imports. (If goimports isn't installed, manually trim them; tsc/build
will catch unused imports.)

- [ ] **Step 3: Wire the bulk endpoint into the router**

Open `internal/api/router/admin.go`. Find the `/use-cases` block (around
line 273):

```go
r.Route("/use-cases", func(rr chi.Router) {
    rr.Get("/", uh.List)
    rr.Post("/", uh.Create)
    // `events` is an audit surface — registered before {id}
    // so chi doesn't try to parse "events" as a UUID.
    rr.Get("/events", uh.ListScopeEvents)
    rr.Get("/{id}", uh.Get)
    rr.Put("/{id}", uh.Update)
    rr.Delete("/{id}", uh.Delete)
    rr.Post("/{id}/recompute", uh.RecomputeClassifications)
})
```

Add a new line directly under the existing `Get("/events", ...)`:

```go
            rr.Post("/events:bulk", uh.BulkSeedScopeEvents)
```

The colon in the path is intentional — chi treats it as a literal so
"events:bulk" doesn't collide with the "events" listing or any UUID
under {id}.

- [ ] **Step 4: Build to verify wiring**

Run: `go build ./...`
Expected: clean build, exit 0.

- [ ] **Step 5: Smoke-test with curl**

Start the gateway locally if it's not running. Then:

```bash
# Pick an existing use case ID from your DB:
UC_ID=$(curl -s -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  http://localhost:8080/api/admin/use-cases | jq -r '.use_cases[0].id')

curl -sS -X POST -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  -H 'Content-Type: application/json' \
  http://localhost:8080/api/admin/use-cases/events:bulk \
  -d "{\"events\":[{\"use_case_id\":\"$UC_ID\",\"kind\":\"llm_model\",\"outcome\":\"denied\",\"target_name\":\"test-target\",\"reason\":\"smoke test\",\"created_at\":\"2026-04-25T12:00:00Z\"}]}"
```
Expected: `{"inserted":1}` and HTTP 201.

Verify the event landed:
```bash
curl -s -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  "http://localhost:8080/api/admin/use-cases/events?since=2026-04-20T00:00:00Z&limit=5" | jq '.events[0]'
```
Expected: shows the seeded event with `created_at` matching the supplied
timestamp.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers/use_cases/scope_events.go \
        internal/api/handlers/use_cases/handler.go \
        internal/api/router/admin.go
git commit -m "feat(api): POST /api/admin/use-cases/events:bulk

Lets the demo seed backdate scope-event history without coupling the
seed CLI to Postgres. Master-key only via the existing admin gate."
```

---

## Task 3: Add the seed package skeleton (REST client)

**Files:**
- Create: `cmd/cli/commands/seed/client.go`

This task lays down the HTTP plumbing the rest of the seed builds on.
No behaviour-driving content yet — just a focused, testable client.

- [ ] **Step 1: Write the client**

Create `cmd/cli/commands/seed/client.go`:

```go
// Package seed implements the `pllm seed` subcommand.
package seed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is a thin REST helper for the demo seed. All routes go through
// the public admin API with the master key — the seed never touches the
// DB directly.
type Client struct {
	BaseURL    string
	MasterKey  string
	HTTP       *http.Client
	uuidsBySlug map[string]uuid.UUID // slug -> gateway-assigned UUID
}

// NewClient returns a ready Client. Callers should set BaseURL +
// MasterKey before use; the timeout default is 30s.
func NewClient(baseURL, masterKey string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		MasterKey:  masterKey,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		uuidsBySlug: map[string]uuid.UUID{},
	}
}

// CacheUUID stores a slug->id mapping for later lookup by dependents.
func (c *Client) CacheUUID(slug string, id uuid.UUID) {
	if slug == "" || id == uuid.Nil {
		return
	}
	c.uuidsBySlug[slug] = id
}

// LookupUUID returns the cached UUID for a slug, or uuid.Nil if not seen.
func (c *Client) LookupUUID(slug string) uuid.UUID {
	return c.uuidsBySlug[slug]
}

// Do issues a JSON request and decodes the response into out (which may
// be nil to discard). Returns the HTTP status and body on non-2xx so
// callers can branch on 404 vs 409 idempotency.
func (c *Client) Do(method, path string, body, out any) (int, []byte, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, buf)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.MasterKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, respBody, nil
}

// MustOK returns nil if status is in 2xx, else an error including the
// response body for diagnostics.
func MustOK(method, path string, status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("%s %s: HTTP %d: %s", method, path, status, strings.TrimSpace(string(body)))
}

// EscapeName URL-escapes a registry name for use in a path segment
// (e.g. "bank.demo/policy-docs" -> "bank.demo%2Fpolicy-docs").
func EscapeName(name string) string {
	return url.PathEscape(name)
}

// HealthCheck verifies the gateway is reachable + the master key works
// by GETting the use cases listing.
func (c *Client) HealthCheck() error {
	status, body, err := c.Do(http.MethodGet, "/api/admin/use-cases", nil, nil)
	if err != nil {
		return fmt.Errorf("gateway unreachable at %s: %w", c.BaseURL, err)
	}
	if status == http.StatusUnauthorized {
		return fmt.Errorf("master key rejected — check --master-key or $PLLM_MASTER_KEY")
	}
	return MustOK(http.MethodGet, "/api/admin/use-cases", status, body)
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./cmd/cli/...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add cmd/cli/commands/seed/client.go
git commit -m "feat(cli): add seed REST client skeleton

Thin master-key authenticated client with a slug->UUID cache so dependent
fixtures (e.g. use-case refs) can resolve without re-querying."
```

---

## Task 4: Add the bank cast as fixtures

**Files:**
- Create: `cmd/cli/commands/seed/fixtures.go`

Pure data — no behaviour. Splitting it out keeps the run loop readable
and means demo-content tweaks don't pollute the diff for plumbing
changes.

- [ ] **Step 1: Write the fixtures file**

Create `cmd/cli/commands/seed/fixtures.go`:

```go
package seed

// Bank-MRM demo cast. All slugs under `bank.demo/*` are owned by the
// seed and safe to delete. Public slugs (tavily-search, mcp.io/*) are
// touched only when the existing entry's description matches our
// marker, so we never blow away an operator's pre-existing entry.

const SeedMarker = "[bank-demo-seed] "

// --- Servers (all catalog-only by default) -------------------------------

type ServerFixture struct {
	Name        string
	Version     string
	Description string
	Score       int // forced enrichment score, 0-100
}

var BankServers = []ServerFixture{
	{
		Name:        "bank.demo/policy-docs",
		Version:     "1.0.0",
		Description: SeedMarker + "Internal policy + procedure RAG for the demo bank.",
		Score:       90,
	},
	{
		Name:        "bank.demo/credit-scoring",
		Version:     "1.0.0",
		Description: SeedMarker + "Credit-score lookup against the bureau.",
		Score:       85,
	},
	{
		Name:        "bank.demo/transactions-readonly",
		Version:     "1.0.0",
		Description: SeedMarker + "Read-only transaction-history lookups.",
		Score:       80,
	},
	{
		Name:        "bank.demo/customer-360",
		Version:     "1.0.0",
		Description: SeedMarker + "Aggregated customer profile (KYC + product holdings).",
		Score:       75,
	},
	{
		Name:        "tavily-search",
		Version:     "1.0.0",
		Description: SeedMarker + "Web search via Tavily (public).",
		Score:       95,
	},
}

// PublicLiveServers are deployed only when --with-deployments is set.
// They reuse standard MCP server packages.
var PublicLiveServers = []ServerFixture{
	{
		Name:        "mcp.io/filesystem",
		Version:     "1.0.0",
		Description: SeedMarker + "Reference filesystem MCP server (sandboxed).",
		Score:       90,
	},
	{
		Name:        "mcp.io/server-everything",
		Version:     "1.0.0",
		Description: SeedMarker + "MCP reference server with every tool type.",
		Score:       90,
	},
}

// --- Skills, prompts ------------------------------------------------------

type ArtifactFixture struct {
	Name        string
	Version     string
	Description string
}

var BankSkills = []ArtifactFixture{
	{
		Name:        "bank.demo/pii-redactor",
		Version:     "1.0.0",
		Description: SeedMarker + "Redact PII from free-text inputs.",
	},
}

var BankPrompts = []ArtifactFixture{
	{
		Name:        "bank.demo/decision-rationale",
		Version:     "1.0.0",
		Description: SeedMarker + "Walk through an underwriting decision step by step.",
	},
	{
		Name:        "bank.demo/kyc-summary-template",
		Version:     "1.0.0",
		Description: SeedMarker + "Summarize KYC findings for an analyst.",
	},
}

// --- Agents ---------------------------------------------------------------

type AgentRefFixture struct {
	TargetKind string
	TargetName string
}

type AgentFixture struct {
	Name        string
	Version     string
	Description string
	Refs        []AgentRefFixture
}

var BankAgents = []AgentFixture{
	{
		Name:        "bank.demo/loan-officer-copilot",
		Version:     "0.1.0",
		Description: SeedMarker + "Copilot for loan officers — pulls policies and credit scores, drafts decision rationales.",
		Refs: []AgentRefFixture{
			{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
			{TargetKind: "server", TargetName: "bank.demo/credit-scoring"},
			{TargetKind: "prompt", TargetName: "bank.demo/decision-rationale"},
		},
	},
	{
		Name:        "bank.demo/kyc-summarizer",
		Version:     "0.1.0",
		Description: SeedMarker + "Summarizes KYC dossiers, redacting PII before producing the summary.",
		Refs: []AgentRefFixture{
			{TargetKind: "server", TargetName: "bank.demo/transactions-readonly"},
			{TargetKind: "prompt", TargetName: "bank.demo/kyc-summary-template"},
			{TargetKind: "skill", TargetName: "bank.demo/pii-redactor"},
		},
	},
}

// --- Use cases ------------------------------------------------------------

type UseCaseRefFixture struct {
	TargetKind string
	TargetName string
}

type UseCaseFixture struct {
	Slug                string
	Name                string
	Description         string
	RiskTier            string // "tier1".."tier4"
	Status              string // "draft" | "under_review" | "approved" | "retired"
	EnforceScope        bool
	DataClassifications []string
	Refs                []UseCaseRefFixture // LLM ref injected at runtime from --llm-route
}

func BankUseCases(llmRoute string) []UseCaseFixture {
	llmRef := UseCaseRefFixture{TargetKind: "llm", TargetName: llmRoute}
	return []UseCaseFixture{
		{
			Slug:                "loan-underwriting-assistant",
			Name:                "Loan underwriting assistant",
			Description:         SeedMarker + "T1 blocking — assists loan officers with underwriting decisions.",
			RiskTier:            "tier1",
			Status:              "approved",
			EnforceScope:        true,
			DataClassifications: []string{"pii", "customer-financial", "confidential"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
				{TargetKind: "server", TargetName: "bank.demo/credit-scoring"},
				{TargetKind: "agent", TargetName: "bank.demo/loan-officer-copilot"},
				{TargetKind: "prompt", TargetName: "bank.demo/decision-rationale"},
			},
		},
		{
			Slug:                "aml-investigation-copilot",
			Name:                "AML investigation copilot",
			Description:         SeedMarker + "T1 flag-mode under review — assists AML analysts during investigations.",
			RiskTier:            "tier1",
			Status:              "under_review",
			EnforceScope:        false,
			DataClassifications: []string{"pii", "customer-financial", "confidential"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/transactions-readonly"},
				{TargetKind: "server", TargetName: "tavily-search"},
				{TargetKind: "agent", TargetName: "bank.demo/kyc-summarizer"},
			},
		},
		{
			Slug:                "branch-staff-knowledge-search",
			Name:                "Branch-staff knowledge search",
			Description:         SeedMarker + "T2 blocking — internal policy and procedure search for branch employees.",
			RiskTier:            "tier2",
			Status:              "approved",
			EnforceScope:        true,
			DataClassifications: []string{"internal"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
				{TargetKind: "server", TargetName: "mcp.io/filesystem"},
			},
		},
		{
			Slug:                "marketing-copy-drafting",
			Name:                "Marketing copy drafting",
			Description:         SeedMarker + "T4 sandbox — drafts marketing copy. Flag mode while in draft.",
			RiskTier:            "tier4",
			Status:              "draft",
			EnforceScope:        false,
			DataClassifications: []string{"public"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "mcp.io/server-everything"},
			},
		},
	}
}

// --- API keys -------------------------------------------------------------

type KeyFixture struct {
	Name        string
	UseCaseSlug string
}

var BankKeys = []KeyFixture{
	{Name: "demo-loan-officer", UseCaseSlug: "loan-underwriting-assistant"},
	{Name: "demo-aml-analyst", UseCaseSlug: "aml-investigation-copilot"},
	{Name: "demo-marketing", UseCaseSlug: "marketing-copy-drafting"},
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./cmd/cli/...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add cmd/cli/commands/seed/fixtures.go
git commit -m "feat(cli): bank-demo cast as Go literals

The four use cases, supporting registry artifacts, and demo API keys.
Each entry carries the [bank-demo-seed] marker so cleanup can recognize
its own without false positives."
```

---

## Task 5: Implement the run loop — registry first

**Files:**
- Create: `cmd/cli/commands/seed/bank.go`

This task does the dependency-ordered upserts. We'll add use cases,
keys, events, and webhook in subsequent tasks; this one stops after
agents.

- [ ] **Step 1: Write the run loop**

Create `cmd/cli/commands/seed/bank.go`:

```go
package seed

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Options controls a single seed run.
type Options struct {
	BaseURL          string
	MasterKey        string
	LLMRoute         string
	WithDeployments  bool
	WithEvents       bool
	WithWebhook      bool
	WebhookURL       string
	JSONOutput       bool
}

// RunBank seeds the bank demo scenario. Idempotent — re-runs update
// existing fixtures rather than failing. Caller passes a fully
// initialized Options.
func RunBank(opts Options) error {
	c := NewClient(opts.BaseURL, opts.MasterKey)
	out := newPrinter(opts.JSONOutput)

	out.Step("verify gateway + master key")
	if err := c.HealthCheck(); err != nil {
		return err
	}

	out.Step("verify LLM route exists: " + opts.LLMRoute)
	if err := ensureRoute(c, opts.LLMRoute); err != nil {
		return err
	}

	out.Step("upsert servers (catalog)")
	for _, s := range BankServers {
		if err := upsertServer(c, s); err != nil {
			return fmt.Errorf("server %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}

	if opts.WithDeployments {
		out.Step("upsert servers (live)")
		for _, s := range PublicLiveServers {
			if err := upsertServer(c, s); err != nil {
				return fmt.Errorf("server %s: %w", s.Name, err)
			}
			out.Item(s.Name + " (catalog only — deployment is a follow-up)")
		}
		// Deployment via the K8s adapter is intentionally not auto-fired
		// from the seed: it requires the wrapper image and per-cluster
		// RBAC. The catalog entry is enough for the UI graph; demo
		// presenter clicks "Deploy" in /ui/deployments when ready.
	}

	out.Step("upsert skills")
	for _, k := range BankSkills {
		if err := upsertSkill(c, k); err != nil {
			return fmt.Errorf("skill %s: %w", k.Name, err)
		}
		out.Item(k.Name)
	}

	out.Step("upsert prompts")
	for _, p := range BankPrompts {
		if err := upsertPrompt(c, p); err != nil {
			return fmt.Errorf("prompt %s: %w", p.Name, err)
		}
		out.Item(p.Name)
	}

	out.Step("upsert agents")
	for _, a := range BankAgents {
		if err := upsertAgent(c, a); err != nil {
			return fmt.Errorf("agent %s: %w", a.Name, err)
		}
		out.Item(a.Name)
	}

	out.Step("upsert use cases")
	ucIDs := map[string]uuid.UUID{}
	for _, uc := range BankUseCases(opts.LLMRoute) {
		id, err := upsertUseCase(c, uc)
		if err != nil {
			return fmt.Errorf("use case %s: %w", uc.Slug, err)
		}
		ucIDs[uc.Slug] = id
		out.Item(uc.Slug + " (" + uc.Status + ")")
	}

	// Tasks 6-8 will append: keys, events, webhook, summary.
	out.Done("bank seed complete (skeleton — keys/events/webhook to follow)")
	return nil
}

// --- Helpers --------------------------------------------------------------

func ensureRoute(c *Client, name string) error {
	type routeRow struct {
		Name string `json:"name"`
	}
	type routesResp struct {
		Routes []routeRow `json:"routes"`
	}
	var resp routesResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/routes", nil, &resp)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	if err := MustOK(http.MethodGet, "/api/admin/routes", status, body); err != nil {
		return err
	}
	for _, r := range resp.Routes {
		if r.Name == name {
			return nil
		}
	}
	return fmt.Errorf(`route %q not found.
  pllm seed bank requires an LLM route. Either:
    - create one in the UI at /ui/routes
    - or pass --llm-route <existing-route-name>`, name)
}

func upsertServer(c *Client, s ServerFixture) error {
	body := map[string]any{
		"name":        s.Name,
		"version":     s.Version,
		"description": s.Description,
	}
	type out struct {
		ID    uuid.UUID `json:"id"`
		Name  string    `json:"name"`
	}
	var got out
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/servers", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/servers", status, b); err != nil {
		return err
	}
	c.CacheUUID(s.Name, got.ID)
	return nil
}

func upsertSkill(c *Client, a ArtifactFixture) error {
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
	}
	type out struct{ ID uuid.UUID `json:"id"` }
	var got out
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/skills", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/skills", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

func upsertPrompt(c *Client, a ArtifactFixture) error {
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
	}
	type out struct{ ID uuid.UUID `json:"id"` }
	var got out
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/prompts", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/prompts", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

func upsertAgent(c *Client, a AgentFixture) error {
	refs := make([]map[string]any, 0, len(a.Refs))
	for _, r := range a.Refs {
		refs = append(refs, map[string]any{
			"target_kind": r.TargetKind,
			"target_name": r.TargetName,
		})
	}
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
		"refs":        refs,
	}
	type out struct{ ID uuid.UUID `json:"id"` }
	var got out
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/agents", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/agents", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

// upsertUseCase POSTs the use case (the Create handler is upsert-by-slug,
// existing rows return 200 with the same row). Returns the row's UUID.
func upsertUseCase(c *Client, uc UseCaseFixture) (uuid.UUID, error) {
	refs := make([]map[string]any, 0, len(uc.Refs))
	for _, r := range uc.Refs {
		refs = append(refs, map[string]any{
			"target_kind": r.TargetKind,
			"target_name": r.TargetName,
		})
	}
	body := map[string]any{
		"name":                 uc.Name,
		"slug":                 uc.Slug,
		"description":          uc.Description,
		"risk_tier":            uc.RiskTier,
		"status":               uc.Status,
		"enforce_scope":        uc.EnforceScope,
		"data_classifications": uc.DataClassifications,
		"refs":                 refs,
	}
	type ucResp struct {
		UseCase struct {
			ID uuid.UUID `json:"id"`
		} `json:"use_case"`
	}
	var got ucResp
	status, b, err := c.Do(http.MethodPost, "/api/admin/use-cases", body, &got)
	if err != nil {
		return uuid.Nil, err
	}
	if err := MustOK(http.MethodPost, "/api/admin/use-cases", status, b); err != nil {
		return uuid.Nil, err
	}
	c.CacheUUID(uc.Slug, got.UseCase.ID)
	return got.UseCase.ID, nil
}
```

- [ ] **Step 2: Add the print helper**

Create `cmd/cli/commands/seed/print.go`:

```go
package seed

import (
	"encoding/json"
	"fmt"
	"os"
)

type printer struct {
	json bool
	log  []map[string]any
}

func newPrinter(asJSON bool) *printer {
	return &printer{json: asJSON}
}

func (p *printer) Step(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"step": s})
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[1;36m== %s ==\033[0m\n", s)
}

func (p *printer) Item(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"item": s})
		return
	}
	fmt.Fprintf(os.Stderr, "  \033[0;32m✓\033[0m %s\n", s)
}

// Result emits a key/value pair (used for the keys block at the end).
func (p *printer) Result(k string, v any) {
	if p.json {
		p.log = append(p.log, map[string]any{"result": k, "value": v})
		return
	}
	fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
}

func (p *printer) Done(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"done": s})
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"events": p.log})
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[1;32m%s\033[0m\n", s)
}
```

- [ ] **Step 3: Build to verify**

Run: `go build ./cmd/cli/...`
Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/commands/seed/bank.go cmd/cli/commands/seed/print.go
git commit -m "feat(cli): seed run loop — registry components

Servers, skills, prompts, agents, use cases. Idempotent via slug-based
upsert; depends on a real --llm-route already existing on the gateway."
```

---

## Task 6: Wire the seed cobra subcommand

**Files:**
- Create: `cmd/cli/commands/seed.go`
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Write the cobra wiring**

Create `cmd/cli/commands/seed.go`:

```go
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amerfu/pllm/cmd/cli/commands/seed"
)

// NewSeedCommand exposes `pllm seed` with sub-scenarios. Today: only
// `bank`. The structure leaves room for `pllm seed insurance` etc.
func NewSeedCommand(_ context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Populate demo data into a pllm install",
		Long:  "Idempotent fixtures driven through the public admin API.",
	}
	cmd.AddCommand(newSeedBankCommand())
	return cmd
}

func newSeedBankCommand() *cobra.Command {
	var opts seed.Options
	var cleanup bool
	cmd := &cobra.Command{
		Use:   "bank",
		Short: "Seed the bank-MRM demo scenario",
		Long: `Materializes a bank governance scenario:
  - 4 use cases (T1 blocking, T1 flag under review, T2 blocking, T4 sandbox)
  - registry agents, prompts, skills, and 5 servers
  - 3 demo API keys bound to use cases
  - 7 days of scope-event history (when --with-events)
  - one disabled webhook subscription (when --with-webhook)

Idempotent: re-running updates existing fixtures. Use --cleanup to remove.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.BaseURL == "" {
				opts.BaseURL = envOr("PLLM_BASE_URL", "http://localhost:8080")
			}
			if opts.MasterKey == "" {
				opts.MasterKey = os.Getenv("PLLM_MASTER_KEY")
			}
			if opts.MasterKey == "" {
				return fmt.Errorf("master key required: pass --master-key or set $PLLM_MASTER_KEY")
			}
			if opts.LLMRoute == "" {
				opts.LLMRoute = envOr("PLLM_DEMO_ROUTE", "free-route")
			}
			if cleanup {
				return seed.CleanupBank(opts)
			}
			return seed.RunBank(opts)
		},
	}
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "gateway URL (default $PLLM_BASE_URL or http://localhost:8080)")
	cmd.Flags().StringVar(&opts.MasterKey, "master-key", "", "admin key (default $PLLM_MASTER_KEY)")
	cmd.Flags().StringVar(&opts.LLMRoute, "llm-route", "", "route name use cases declare (default free-route)")
	cmd.Flags().BoolVar(&opts.WithDeployments, "with-deployments", false, "also publish 2 live MCP server entries (no auto-deploy)")
	cmd.Flags().BoolVar(&opts.WithEvents, "with-events", true, "seed 7-day scope event history")
	cmd.Flags().BoolVar(&opts.WithWebhook, "with-webhook", true, "seed disabled webhook subscription")
	cmd.Flags().StringVar(&opts.WebhookURL, "webhook-url", "https://webhook.site/REPLACE-ME", "URL for the seeded webhook")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "machine-readable JSON output")
	cmd.Flags().BoolVar(&cleanup, "cleanup", false, "remove all bank.demo/* and demo-* fixtures and exit")
	return cmd
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Add a placeholder `CleanupBank`**

Cleanup is implemented in Task 9, but the cobra wiring needs the symbol
to exist now. Open `cmd/cli/commands/seed/bank.go` and append:

```go
// CleanupBank removes everything `RunBank` creates. Implemented in
// cleanup.go; this stub exists so the cobra wiring compiles before
// the cleanup task is done.
func CleanupBank(opts Options) error {
	return cleanupBankInternal(opts)
}
```

Then add a tiny stub in a new file `cmd/cli/commands/seed/cleanup.go`:

```go
package seed

import "fmt"

// cleanupBankInternal is filled out in Task 9.
func cleanupBankInternal(_ Options) error {
	return fmt.Errorf("cleanup not yet implemented (Task 9)")
}
```

- [ ] **Step 3: Register in `main.go`**

Open `cmd/cli/main.go`. In `newRootCommand()`, find the block that adds
subcommands:

```go
rootCmd.AddCommand(commands.NewUserCommand(ctx))
rootCmd.AddCommand(commands.NewTeamCommand(ctx))
rootCmd.AddCommand(commands.NewKeyCommand(ctx))
rootCmd.AddCommand(commands.NewBudgetCommand(ctx))
rootCmd.AddCommand(commands.NewConfigCommand())
```

Add immediately after:

```go
rootCmd.AddCommand(commands.NewSeedCommand(ctx))
```

- [ ] **Step 4: Build the CLI**

Run: `go build -o /tmp/pllm-cli ./cmd/cli`
Expected: clean build, exit 0.

- [ ] **Step 5: Smoke-test help output**

Run: `/tmp/pllm-cli seed bank --help`
Expected: prints the long help with all flags listed.

- [ ] **Step 6: Smoke-test against a live gateway**

With the gateway running and `PLLM_MASTER_KEY` exported:

```bash
export PLLM_MASTER_KEY=sk-master-dev-key-change-in-production
/tmp/pllm-cli seed bank --llm-route free-route
```

Expected: prints "verify gateway", "verify LLM route", then upserts
through agents and use cases. Final line: "bank seed complete (skeleton
— keys/events/webhook to follow)". The four use cases now appear in
`/ui/use-cases`.

If `free-route` doesn't exist, expected: clean error message pointing
the operator at `/ui/routes` or `--llm-route`.

- [ ] **Step 7: Commit**

```bash
git add cmd/cli/commands/seed.go cmd/cli/commands/seed/cleanup.go \
        cmd/cli/commands/seed/bank.go cmd/cli/main.go
git commit -m "feat(cli): wire pllm seed bank subcommand

Cobra subcommand with --base-url, --master-key, --llm-route, --cleanup,
--with-deployments, --with-events, --with-webhook, --webhook-url, --json.
Health-checks the gateway and verifies the route exists before touching
fixtures."
```

---

## Task 7: Seed API keys

**Files:**
- Modify: `cmd/cli/commands/seed/bank.go`

- [ ] **Step 1: Add the key upsert helper**

Open `cmd/cli/commands/seed/bank.go`. Append after `upsertUseCase`:

```go
// upsertKey ensures a demo key exists and is bound to the given use case.
// The admin keys API auto-generates the key value; the seed prints it
// at the end so the demo presenter can copy it. Re-runs cannot recover
// the plaintext (it's only returned at creation time), so existing
// keys are PATCHed-by-PUT to refresh the use-case binding and report
// "(unchanged — token already issued)".
func upsertKey(c *Client, k KeyFixture, useCaseID uuid.UUID) (string, error) {
	// Look for an existing key by name first.
	// ListKeys response: { keys: [{...key fields, plaintext_key, usage}], pagination: {...} }
	type keyRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type listResp struct {
		Keys []keyRow `json:"keys"`
	}
	var existing listResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/keys?limit=200", nil, &existing)
	if err != nil {
		return "", err
	}
	if err := MustOK(http.MethodGet, "/api/admin/keys", status, body); err != nil {
		return "", err
	}
	for _, row := range existing.Keys {
		if row.Name == k.Name {
			// UpdateKey uses PUT /api/admin/keys/{keyID}. The use_case_id
			// field on the update body is **uuid.UUID, so we wrap once
			// to avoid the "field absent vs JSON null" ambiguity.
			ucPtr := useCaseID
			update := map[string]any{"use_case_id": &ucPtr}
			st, b, err := c.Do(http.MethodPut, "/api/admin/keys/"+row.ID.String(), update, nil)
			if err != nil {
				return "", err
			}
			if err := MustOK(http.MethodPut, "/api/admin/keys/"+row.ID.String(), st, b); err != nil {
				return "", err
			}
			return "(unchanged — token already issued)", nil
		}
	}
	// Create. CreateKeyRequest field is `key_type`. Response is
	// { ...embedded models.Key fields, plaintext_key: "...", usage: {...} }.
	create := map[string]any{
		"name":        k.Name,
		"key_type":    "api",
		"use_case_id": useCaseID,
	}
	var got struct {
		PlaintextKey string `json:"plaintext_key"`
	}
	st, b, err := c.Do(http.MethodPost, "/api/admin/keys", create, &got)
	if err != nil {
		return "", err
	}
	if err := MustOK(http.MethodPost, "/api/admin/keys", st, b); err != nil {
		return "", err
	}
	return got.PlaintextKey, nil
}
```

- [ ] **Step 2: Wire keys into the run loop**

Open `cmd/cli/commands/seed/bank.go`. In `RunBank`, replace the block:

```go
	// Tasks 6-8 will append: keys, events, webhook, summary.
	out.Done("bank seed complete (skeleton — keys/events/webhook to follow)")
	return nil
}
```

with:

```go
	out.Step("upsert API keys")
	keyTokens := map[string]string{}
	for _, k := range BankKeys {
		ucID, ok := ucIDs[k.UseCaseSlug]
		if !ok {
			return fmt.Errorf("key %s: bound use case %s not found", k.Name, k.UseCaseSlug)
		}
		token, err := upsertKey(c, k, ucID)
		if err != nil {
			return fmt.Errorf("key %s: %w", k.Name, err)
		}
		keyTokens[k.Name] = token
		out.Item(k.Name + " → " + k.UseCaseSlug)
	}

	// Tasks 8-9 will append: events, webhook.
	out.Step("seeded API keys (copy these into your demo notes)")
	for _, k := range BankKeys {
		out.Result(k.Name, keyTokens[k.Name])
	}

	out.Done("bank seed complete")
	return nil
}
```

- [ ] **Step 3: Build + smoke-test**

Run: `go build -o /tmp/pllm-cli ./cmd/cli && /tmp/pllm-cli seed bank --llm-route free-route`
Expected: prints three keys at the end. Re-running should print
`(unchanged — token not re-emitted)` for each, proving idempotency.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/commands/seed/bank.go
git commit -m "feat(cli): seed demo API keys bound to use cases

Three keys, one per active use case. New keys print their token; existing
keys are PATCHed to ensure the use-case binding is current but the
token is not re-emitted (it's not retrievable post-creation)."
```

---

## Task 8: Seed scope-event history + webhook

**Files:**
- Create: `cmd/cli/commands/seed/events.go`
- Modify: `cmd/cli/commands/seed/bank.go`

- [ ] **Step 1: Write the event generator**

Create `cmd/cli/commands/seed/events.go`:

```go
package seed

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// generateBankEvents produces ~30 events spread across the previous
// 7 days. Distribution is hand-tuned for the demo's posture story:
// roughly 20 allowed (≈70%), 5-7 denied concentrated in the last
// 24h, 2-3 flagged spread over 3-4 days. The Compliance page is
// driven by denied + flagged; allowed events don't currently appear
// in scope_events at all (they're allowed by definition), so this
// helper only emits denied + flagged entries.
//
// Returns ready-to-POST event payloads. Caller injects use_case_id
// from the seeded slugs.
func generateBankEvents(ucIDs map[string]uuid.UUID) []map[string]any {
	now := time.Now().UTC()
	events := []map[string]any{}

	loanID, hasLoan := ucIDs["loan-underwriting-assistant"]
	amlID, hasAML := ucIDs["aml-investigation-copilot"]

	// 5 denied on loan-underwriting yesterday — operator's "recent" story
	if hasLoan {
		offsets := []time.Duration{24, 22, 20, 18, 14}
		for _, h := range offsets {
			events = append(events, map[string]any{
				"use_case_id": loanID,
				"kind":        "llm_model",
				"outcome":     "denied",
				"target_name": "gpt-4-turbo",
				"key_name":    "demo-loan-officer",
				"reason":      "model gpt-4-turbo is not declared by the bound use case",
				"remote_addr": "10.42.0.7",
				"created_at":  now.Add(-h * time.Hour).Format(time.RFC3339),
			})
		}
	}

	// 3 flagged on AML — flag mode means logged-not-blocked
	if hasAML {
		offsets := []time.Duration{6, 30, 70} // hours ago
		for _, h := range offsets {
			events = append(events, map[string]any{
				"use_case_id": amlID,
				"kind":        "mcp_tool",
				"outcome":     "flagged",
				"target_name": "tavily-search__search",
				"key_name":    "demo-aml-analyst",
				"reason":      "tool tavily-search__search is not declared by the bound use case",
				"remote_addr": "10.42.0.8",
				"created_at":  now.Add(-h * time.Hour).Format(time.RFC3339),
			})
		}
	}

	return events
}

// pushEvents ships the events through the bulk endpoint.
func pushEvents(c *Client, events []map[string]any) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	body := map[string]any{"events": events}
	type resp struct {
		Inserted int `json:"inserted"`
	}
	var got resp
	status, b, err := c.Do(http.MethodPost, "/api/admin/use-cases/events:bulk", body, &got)
	if err != nil {
		return 0, err
	}
	if err := MustOK(http.MethodPost, "/api/admin/use-cases/events:bulk", status, b); err != nil {
		return 0, err
	}
	return got.Inserted, nil
}

// upsertWebhook ensures a single demo webhook subscription exists,
// disabled by default. Operator flips it on after replacing the URL.
func upsertWebhook(c *Client, url string) error {
	// Look for an existing one by name.
	type webhookRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type listResp struct {
		Webhooks []webhookRow `json:"webhooks"`
	}
	var existing listResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/webhooks", nil, &existing)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/webhooks", status, body); err != nil {
		return err
	}
	const demoName = "demo-siem-stub"
	payload := map[string]any{
		"name":    demoName,
		"url":     url,
		"events":  []string{"scope.denied", "scope.flagged", "use_case.created", "use_case.updated", "use_case.compliance_flag"},
		"enabled": false,
	}
	for _, w := range existing.Webhooks {
		if w.Name == demoName {
			st, b, err := c.Do(http.MethodPut, "/api/admin/webhooks/"+w.ID.String(), payload, nil)
			if err != nil {
				return err
			}
			return MustOK(http.MethodPut, "/api/admin/webhooks/"+w.ID.String(), st, b)
		}
	}
	st, b, err := c.Do(http.MethodPost, "/api/admin/webhooks", payload, nil)
	if err != nil {
		return err
	}
	return MustOK(http.MethodPost, "/api/admin/webhooks", st, b)
}

// keysExist (placeholder for Task 9 cleanup symmetry — defined here
// so it lives next to its companion listing helpers).
func _suppressUnused() { _ = fmt.Sprintf }
```

(The `_suppressUnused` line is a no-op the build needs only if `fmt`
ends up unused. Drop it if `fmt` is referenced in the final build.)

- [ ] **Step 2: Wire events + webhook into the run loop**

Open `cmd/cli/commands/seed/bank.go`. In `RunBank`, replace the block:

```go
	// Tasks 8-9 will append: events, webhook.
	out.Step("seeded API keys (copy these into your demo notes)")
```

with:

```go
	if opts.WithEvents {
		out.Step("seed scope-event history (last 7 days)")
		ev := generateBankEvents(ucIDs)
		n, err := pushEvents(c, ev)
		if err != nil {
			return fmt.Errorf("seed events: %w", err)
		}
		out.Item(fmt.Sprintf("%d events inserted", n))
	}

	if opts.WithWebhook {
		out.Step("upsert webhook subscription (disabled)")
		if err := upsertWebhook(c, opts.WebhookURL); err != nil {
			return fmt.Errorf("seed webhook: %w", err)
		}
		out.Item("demo-siem-stub → " + opts.WebhookURL + " (disabled)")
	}

	out.Step("seeded API keys (copy these into your demo notes)")
```

- [ ] **Step 3: Build + smoke-test**

```bash
go build -o /tmp/pllm-cli ./cmd/cli
/tmp/pllm-cli seed bank --llm-route free-route
```

Expected: events block reports `8 events inserted`, webhook block
reports the URL, keys block prints three tokens. Open `/ui/compliance`
— the timeline now has shape, "Use cases with most denials" features
`loan-underwriting-assistant`, "Most-denied targets" lists `gpt-4-turbo`.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/commands/seed/events.go cmd/cli/commands/seed/bank.go
git commit -m "feat(cli): seed scope events + webhook

Eight historical scope events spread across 7 days (5 denials on the
loan use case, 3 flagged on AML), plus one disabled webhook subscription
for the live-fire portion of the demo."
```

---

## Task 9: Implement cleanup

**Files:**
- Modify: `cmd/cli/commands/seed/cleanup.go`

- [ ] **Step 1: Replace the stub with the real cleanup**

Open `cmd/cli/commands/seed/cleanup.go`. Replace the entire file with:

```go
package seed

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// cleanupBankInternal removes everything RunBank creates. Idempotent —
// missing entities are silently skipped.
func cleanupBankInternal(opts Options) error {
	c := NewClient(opts.BaseURL, opts.MasterKey)
	out := newPrinter(opts.JSONOutput)

	out.Step("verify gateway + master key")
	if err := c.HealthCheck(); err != nil {
		return err
	}

	// ---- Webhook --------------------------------------------------------
	out.Step("remove webhook")
	type webhookRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type wResp struct{ Webhooks []webhookRow `json:"webhooks"` }
	var ws wResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/webhooks", nil, &ws)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/webhooks", status, body); err != nil {
		return err
	}
	for _, w := range ws.Webhooks {
		if w.Name == "demo-siem-stub" {
			st, b, err := c.Do(http.MethodDelete, "/api/admin/webhooks/"+w.ID.String(), nil, nil)
			if err != nil {
				return err
			}
			if err := MustOK(http.MethodDelete, "/api/admin/webhooks/"+w.ID.String(), st, b); err != nil {
				return err
			}
			out.Item("demo-siem-stub")
		}
	}

	// ---- Keys -----------------------------------------------------------
	out.Step("remove demo-* keys")
	type keyRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type kResp struct{ Keys []keyRow `json:"keys"` }
	var ks kResp
	status, body, err = c.Do(http.MethodGet, "/api/admin/keys?limit=500", nil, &ks)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/keys", status, body); err != nil {
		return err
	}
	for _, k := range ks.Keys {
		if !strings.HasPrefix(k.Name, "demo-") {
			continue
		}
		st, b, err := c.Do(http.MethodDelete, "/api/admin/keys/"+k.ID.String(), nil, nil)
		if err != nil {
			return err
		}
		if err := MustOK(http.MethodDelete, "/api/admin/keys/"+k.ID.String(), st, b); err != nil {
			return err
		}
		out.Item(k.Name)
	}

	// ---- Use cases ------------------------------------------------------
	out.Step("remove use cases")
	demoSlugs := map[string]bool{}
	for _, uc := range BankUseCases(opts.LLMRoute) {
		demoSlugs[uc.Slug] = true
	}
	type ucRow struct {
		ID   uuid.UUID `json:"id"`
		Slug string    `json:"slug"`
	}
	type ucResp struct{ UseCases []ucRow `json:"use_cases"` }
	var ucs ucResp
	status, body, err = c.Do(http.MethodGet, "/api/admin/use-cases", nil, &ucs)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/use-cases", status, body); err != nil {
		return err
	}
	for _, uc := range ucs.UseCases {
		if !demoSlugs[uc.Slug] {
			continue
		}
		st, b, err := c.Do(http.MethodDelete, "/api/admin/use-cases/"+uc.ID.String(), nil, nil)
		if err != nil {
			return err
		}
		if err := MustOK(http.MethodDelete, "/api/admin/use-cases/"+uc.ID.String(), st, b); err != nil {
			return err
		}
		out.Item(uc.Slug)
	}

	// ---- Registry artifacts --------------------------------------------
	// Servers, agents, prompts, skills under bank.demo/* are always ours.
	// Public names (tavily-search, mcp.io/*) are deleted only if their
	// description starts with our SeedMarker so we don't blow away an
	// operator's pre-existing entry.

	allServers := append([]ServerFixture{}, BankServers...)
	allServers = append(allServers, PublicLiveServers...)
	out.Step("remove registry servers")
	for _, s := range allServers {
		if err := deleteRegistryArtifact(c, "servers", s.Name, s.Version); err != nil {
			return fmt.Errorf("delete server %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}
	out.Step("remove registry agents")
	for _, a := range BankAgents {
		if err := deleteRegistryArtifact(c, "agents", a.Name, a.Version); err != nil {
			return fmt.Errorf("delete agent %s: %w", a.Name, err)
		}
		out.Item(a.Name)
	}
	out.Step("remove registry prompts")
	for _, p := range BankPrompts {
		if err := deleteRegistryArtifact(c, "prompts", p.Name, p.Version); err != nil {
			return fmt.Errorf("delete prompt %s: %w", p.Name, err)
		}
		out.Item(p.Name)
	}
	out.Step("remove registry skills")
	for _, s := range BankSkills {
		if err := deleteRegistryArtifact(c, "skills", s.Name, s.Version); err != nil {
			return fmt.Errorf("delete skill %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}

	out.Done("bank seed cleanup complete")
	return nil
}

// deleteRegistryArtifact deletes the seeded version of name. Public-name
// safety check (description SeedMarker) is enforced by GETting first.
// The registry delete route is concrete on (name, version) — there is
// no "latest" alias — so we always pass the version we seeded.
func deleteRegistryArtifact(c *Client, kind, name, version string) error {
	// Public namespace safety check: if not under bank.demo/*, verify
	// the description is one of ours before deleting.
	if !strings.HasPrefix(name, "bank.demo/") {
		path := "/v1/registry/" + kind + "/" + EscapeName(name)
		var got struct {
			Description string `json:"description"`
		}
		status, body, err := c.Do(http.MethodGet, path, nil, &got)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return nil
		}
		if err := MustOK(http.MethodGet, path, status, body); err != nil {
			return err
		}
		if !strings.HasPrefix(got.Description, SeedMarker) {
			return nil // operator-owned; don't touch
		}
	}
	path := "/api/admin/registry/" + kind + "/" + EscapeName(name) + "/versions/" + EscapeName(version)
	st, b, err := c.Do(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if st == http.StatusNotFound {
		return nil
	}
	return MustOK(http.MethodDelete, path, st, b)
}
```

Drop the placeholder symbol from `events.go` if you added one in
Task 8. Build will tell you.

- [ ] **Step 2: Build + smoke-test**

```bash
go build -o /tmp/pllm-cli ./cmd/cli
/tmp/pllm-cli seed bank --cleanup --llm-route free-route
```

Expected: prints "remove webhook", "remove demo-* keys", "remove use
cases", "remove registry servers/agents/prompts/skills", ending with
"bank seed cleanup complete". `/ui/use-cases` empties of the four bank
use cases; `/ui/registry` no longer shows `bank.demo/*`.

Re-running cleanup should be a no-op (no errors, all sections print
without items).

- [ ] **Step 3: Round-trip test**

Verify idempotency end-to-end:
```bash
/tmp/pllm-cli seed bank --llm-route free-route                 # creates
/tmp/pllm-cli seed bank --llm-route free-route                 # idempotent, no errors
/tmp/pllm-cli seed bank --cleanup --llm-route free-route       # removes
/tmp/pllm-cli seed bank --cleanup --llm-route free-route       # idempotent, no errors
/tmp/pllm-cli seed bank --llm-route free-route                 # creates again
```

All 5 invocations should exit 0.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/commands/seed/cleanup.go
git commit -m "feat(cli): pllm seed bank --cleanup

Idempotent removal scoped by namespace (bank.demo/*) + name prefix
(demo-*) + SeedMarker description match for public-namespace artifacts."
```

---

## Task 10: Add Makefile targets + K8s Job manifest + bash wrapper

**Files:**
- Modify: `Makefile`
- Create: `deploy/k8s/demo-seed-job.yaml`
- Modify: `scripts/demo-seed.sh`

- [ ] **Step 1: Add Makefile targets**

Open `Makefile`. Find a logical home next to the existing `dev` /
`build` targets. Add:

```makefile
## demo-seed: seed the bank-MRM demo scenario into the local gateway
demo-seed:
	@if ! command -v pllm >/dev/null 2>&1; then \
		go build -o ./bin/pllm ./cmd/cli; \
		PATH=$(PWD)/bin:$$PATH ./bin/pllm seed bank; \
	else \
		pllm seed bank; \
	fi

## demo-seed-clean: remove all bank-MRM demo fixtures
demo-seed-clean:
	@if ! command -v pllm >/dev/null 2>&1; then \
		go build -o ./bin/pllm ./cmd/cli; \
		PATH=$(PWD)/bin:$$PATH ./bin/pllm seed bank --cleanup; \
	else \
		pllm seed bank --cleanup; \
	fi
```

- [ ] **Step 2: Add the K8s Job manifest**

Create `deploy/k8s/demo-seed-job.yaml`:

```yaml
# One-shot Job that seeds the bank-MRM demo into a pllm install.
#
# Usage:
#   1. Ensure the existing pllm secret exposes PLLM_MASTER_KEY (it does
#      by default in deploy/k8s/secret.yaml).
#   2. Apply: kubectl apply -f deploy/k8s/demo-seed-job.yaml
#   3. Check: kubectl logs -l job=pllm-demo-seed
#   4. The Job is finite — it runs once and exits 0 on success.
#
# To re-run after fixing something:
#   kubectl delete job pllm-demo-seed
#   kubectl apply -f deploy/k8s/demo-seed-job.yaml
#
# To remove the seed:
#   kubectl create job pllm-demo-seed-cleanup --from=cronjob/pllm \
#     -- pllm seed bank --cleanup
# (or set args: ["seed","bank","--cleanup"] in a copy of this manifest)

apiVersion: batch/v1
kind: Job
metadata:
  name: pllm-demo-seed
  namespace: pllm
  labels:
    app: pllm
    component: demo-seed
spec:
  ttlSecondsAfterFinished: 3600
  backoffLimit: 1
  template:
    metadata:
      labels:
        app: pllm
        component: demo-seed
    spec:
      restartPolicy: Never
      containers:
        - name: seed
          image: ghcr.io/amerfu/pllm:latest
          imagePullPolicy: IfNotPresent
          command: ["pllm"]
          args: ["seed", "bank"]
          env:
            - name: PLLM_BASE_URL
              value: "http://pllm.pllm.svc.cluster.local:8080"
            - name: PLLM_MASTER_KEY
              valueFrom:
                secretKeyRef:
                  name: pllm
                  key: master-key
            - name: PLLM_DEMO_ROUTE
              value: "free-route"
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
```

- [ ] **Step 3: Update the bash wrapper**

Open `scripts/demo-seed.sh`. Replace its contents with:

```bash
#!/usr/bin/env bash
#
# demo-seed.sh — thin wrapper. Prefers `pllm seed bank` if available,
# falls back to building the CLI on the fly.
#
# All flags pass through. Examples:
#   scripts/demo-seed.sh                     # seed
#   scripts/demo-seed.sh --cleanup           # remove
#   scripts/demo-seed.sh --llm-route my-route

set -euo pipefail

cd "$(dirname "$0")/.."

if command -v pllm >/dev/null 2>&1; then
  exec pllm seed bank "$@"
fi

if [ ! -x ./bin/pllm ]; then
  echo "building ./bin/pllm..." >&2
  go build -o ./bin/pllm ./cmd/cli
fi

exec ./bin/pllm seed bank "$@"
```

Make it executable: `chmod +x scripts/demo-seed.sh`.

- [ ] **Step 4: Sanity-check the wrapper**

Run: `scripts/demo-seed.sh --help`
Expected: prints the seed-bank help.

Run: `make demo-seed`
Expected: same effect as the previous direct CLI run — seeds the
scenario.

- [ ] **Step 5: Commit**

```bash
git add Makefile deploy/k8s/demo-seed-job.yaml scripts/demo-seed.sh
git commit -m "feat: make targets + K8s Job + bash wrapper for demo seed

make demo-seed / demo-seed-clean wrap pllm seed bank for local dev.
deploy/k8s/demo-seed-job.yaml runs the same command as a one-shot Job
in a real cluster, pulling PLLM_MASTER_KEY from the existing secret."
```

---

## Task 11: End-to-end verification

**Files:** none (verification only)

- [ ] **Step 1: Round-trip on a clean local install**

```bash
make demo-seed-clean   # ensure clean slate
make demo-seed
```

Open the gateway UI:

- `/ui/registry` — bank.demo/* servers/agents/prompts/skills present, scores rendered
- `/ui/use-cases` — 4 use cases shown across the matrix; ring is no longer 200%; tooltips work
- `/ui/use-cases/<id>` (drill in to loan-underwriting-assistant) — Graph tab shows LLM ref + servers + agent + prompt
- `/ui/compliance` — posture in the watch band (~70-80), 7-day timeline has shape, "Use cases with most denials" features loan-underwriting, "Most-denied targets" lists `gpt-4-turbo`
- `/ui/keys` — three demo-* keys, each bound to its use case (badge or column visible)
- `/ui/compliance` Webhooks section — `demo-siem-stub` shown disabled

- [ ] **Step 2: Verify the live denial path**

Pick one of the printed key tokens. With it, hit the gateway:

```bash
# Allowed call (the use case declares free-route):
curl -sS -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <demo-loan-officer-token>" \
  -H 'Content-Type: application/json' \
  -d '{"model":"free-route","messages":[{"role":"user","content":"ping"}]}' | jq

# Denied call (the use case does NOT declare gpt-4-turbo):
curl -sS -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <demo-loan-officer-token>" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4-turbo","messages":[{"role":"user","content":"ping"}]}' | jq
```

Expected:
- First: 200 with a real assistant message routed through `free-route`.
- Second: 403 with a JSON error mentioning the use case scope.

Reload `/ui/compliance` — a new event should appear in the live feed
within ~10s.

- [ ] **Step 3: Cleanup test**

```bash
make demo-seed-clean
```
Open `/ui/use-cases`, `/ui/registry`, `/ui/keys`, `/ui/compliance`. All
demo entries gone. No residual scope events from `demo-*` keys (those
remain in the audit log — by design — but no new events appear).

- [ ] **Step 4: Commit verification notes (if anything was tweaked)**

If steps 1-3 surfaced bugs and you patched them in any of the previous
tasks, commit those fixes with a referencing message:

```bash
git add <touched files>
git commit -m "fix(seed): <what>

Found during Task 11 verification."
```

If no fixes were needed, no commit — just proceed.

---

## Self-Review

**1. Spec coverage**

Spec section → Plan task:
- "Delivery" → Tasks 3, 6, 10
- "Bank cast" (4 use cases) → Task 4
- "Component catalog" (servers/agents/prompts/skills) → Tasks 4, 5
- "LLM dependency by route name" → Tasks 4 (fixture), 5 (`ensureRoute`)
- "API keys" → Task 7
- "Scope event history" → Tasks 1, 2, 8
- "Required new admin endpoint" (`POST /api/admin/use-cases/events:bulk`) → Tasks 1, 2
- "Webhook subscription" → Task 8
- "Ordering & idempotency" → Task 5 (registry order), Task 7 (key idempotency), Task 8 (webhook PUT-or-POST)
- "Cleanup (`--cleanup`)" → Task 9
- "Flags" → Task 6
- "File layout" → matches plan structure
- "Demo arc enabled" → Task 11 verification mirrors the arc

No gaps.

**2. Placeholder scan**

No "TBD" / "TODO" / "implement later" left. Step 4 of Task 8 mentions
the `_suppressUnused` no-op as conditional cleanup; that's a build-
hygiene note, not a placeholder.

**3. Type consistency**

- `seed.Options` (Task 6) matches what `RunBank` (Task 5) and
  `CleanupBank` (Task 6/9) consume.
- `BulkRecordInput` field set (Task 1) matches what
  `BulkSeedScopeEvents` (Task 2) sends through the service.
- `ServerFixture` / `AgentFixture` / `UseCaseFixture` field names in
  Task 4 are referenced by name in Task 5 — manually checked.
- `bulkSeedEvent` JSON field names match what Task 8's
  `generateBankEvents` emits (`use_case_id`, `kind`, `outcome`,
  `target_name`, `key_name`, `reason`, `remote_addr`, `created_at`).

No drift detected.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-02-bank-demo-seed.md`.
Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
