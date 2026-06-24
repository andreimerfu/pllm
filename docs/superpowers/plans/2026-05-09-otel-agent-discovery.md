# OTel-Driven Agent Discovery & Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run a Python agent that emits OpenTelemetry traces, see it auto-onboard into pllm's registry, govern it via a UseCase that hard-blocks LLM/MCP calls outside scope and flags-then-revokes when it touches OTel-discovered targets (Qdrant) outside policy.

**Architecture:** New `internal/services/discovery/` package owns an OTLP/HTTP JSON receiver, span processor (extracts agent identity + call kind from resource/span attributes), policy evaluator (reuses `use_cases.ScopeResolver`), and a flag-threshold key revoker. Admin handlers list/promote `DiscoveredAgent` rows into existing `RegistryAgent` rows; `Compliance.tsx` gets a new section. A demo Python agent uses `pllm.key_id` as an OTel resource attribute so spans link directly to the bound `Key` and its `UseCase`. LLM/MCP enforcement reuses the existing `ScopeChecker` path — no changes there.

**Tech Stack:** Go (chi, GORM, no new deps — minimal OTLP JSON struct defined locally), Python 3.11 (openai, qdrant-client, opentelemetry-sdk + auto-instrumentation), Qdrant (docker), Vite/React frontend.

---

## File structure

**New backend files:**
- `internal/core/models/discovered_agent.go` — `DiscoveredAgent` GORM model
- `internal/core/models/discovered_call.go` — `DiscoveredCall` GORM model
- `internal/services/discovery/types.go` — minimal OTLP/HTTP JSON structs
- `internal/services/discovery/processor.go` — span → `(agent, call)` extraction
- `internal/services/discovery/processor_test.go`
- `internal/services/discovery/policy.go` — call → policy decision → ScopeEvent + webhook
- `internal/services/discovery/policy_test.go`
- `internal/services/discovery/revoker.go` — flag-count threshold key revocation
- `internal/services/discovery/revoker_test.go`
- `internal/services/discovery/service.go` — Service struct (Ingest entry point, persistence, query helpers)
- `internal/services/discovery/service_test.go`
- `internal/api/handlers/otel/handler.go` — POST `/v1/traces` OTLP receiver
- `internal/api/handlers/discovery/handler.go` — admin list/get/promote/ignore
- `examples/agents/fraud-investigator/main.py` — sample agent
- `examples/agents/fraud-investigator/requirements.txt`
- `examples/agents/fraud-investigator/docker-compose.yml` — Qdrant + agent
- `examples/agents/fraud-investigator/README.md`
- `scripts/test-otel-discovery.sh` — end-to-end smoke

**New frontend files:**
- `web/src/components/discovery/discovered-agents-list.tsx`
- `web/src/components/discovery/discovered-agent-drawer.tsx`

**Modified files:**
- `internal/core/models/scope_event.go` — add `ScopeEventKindObserved`
- `internal/core/database/connection.go` — AutoMigrate (add 2 models)
- `internal/core/database/migrate.go` — AutoMigrate (add 2 models)
- `internal/infrastructure/testutil/database.go` — AutoMigrate (add 2 models)
- `internal/api/router/router.go` — wire discovery service + handlers
- `internal/api/router/admin.go` — mount `/api/admin/discovery/*`
- `internal/core/config/config.go` — add `DiscoveryConfig` section
- `config.yaml` + `config.docker.yaml` — `discovery` block
- `web/src/types/api.ts` — `DiscoveredAgent`, `DiscoveredCall` types
- `web/src/lib/api.ts` — `listDiscoveredAgents`, `getDiscoveredAgent`, `promoteDiscoveredAgent`, `ignoreDiscoveredAgent`
- `web/src/pages/Compliance.tsx` — render `<DiscoveredAgentsList/>` section

Each file has a single responsibility; all package boundaries follow CLAUDE.md's "narrow consumer-side interfaces, bridged in router.go" rule. The discovery package never imports `use_cases` or `webhooks` directly — it depends on small interfaces wired in `router.go`.

---

## Task 1: Add `DiscoveredAgent` and `DiscoveredCall` models

**Files:**
- Create: `internal/core/models/discovered_agent.go`
- Create: `internal/core/models/discovered_call.go`
- Modify: `internal/core/models/scope_event.go`
- Modify: `internal/core/database/connection.go` (find `AutoMigrate(` call, add 2 lines)
- Modify: `internal/core/database/migrate.go` (same)
- Modify: `internal/infrastructure/testutil/database.go` (same)

- [ ] **Step 1: Write `discovered_agent.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// DiscoveredAgentStatus is the operator's lifecycle for an agent
// observed via OTel but not yet registered in the catalog.
type DiscoveredAgentStatus string

const (
	DiscoveredStatusPending  DiscoveredAgentStatus = "pending"
	DiscoveredStatusPromoted DiscoveredAgentStatus = "promoted"
	DiscoveredStatusIgnored  DiscoveredAgentStatus = "ignored"
)

// DiscoveredAgent represents an agent fingerprint that pllm has seen
// in OpenTelemetry traffic but that has not yet been published to the
// `RegistryAgent` catalog. Identity is keyed by service.name (the
// OTel resource attribute) plus the API key the agent uses for
// pllm-mediated traffic, when known.
type DiscoveredAgent struct {
	BaseModel

	// ServiceName is the OTel resource `service.name`. Required.
	ServiceName string `gorm:"size:255;index;not null" json:"service_name"`

	// ServiceVersion is the OTel resource `service.version`, if present.
	ServiceVersion string `gorm:"size:64" json:"service_version,omitempty"`

	// KeyID is the pllm API key that the agent attaches to OTel spans
	// via the `pllm.key_id` resource attribute. Optional — agents that
	// only call non-pllm targets won't have one. When set, governance
	// is derived from the bound key's UseCase.
	KeyID *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`

	// PromotedAgentID points at the RegistryAgent the operator created
	// from this discovery, once promoted.
	PromotedAgentID *uuid.UUID `gorm:"type:uuid" json:"promoted_agent_id,omitempty"`

	Status DiscoveredAgentStatus `gorm:"size:16;index;default:pending;not null" json:"status"`

	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	IgnoredAt   *time.Time `json:"ignored_at,omitempty"`
	PromotedAt  *time.Time `json:"promoted_at,omitempty"`

	// FlaggedCount is incremented every time policy evaluation produces
	// a flagged outcome. The revoker reads this; the API exposes it.
	FlaggedCount int `gorm:"default:0;not null" json:"flagged_count"`
}

func (DiscoveredAgent) TableName() string { return "discovered_agents" }
```

- [ ] **Step 2: Write `discovered_call.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// DiscoveredCallKind classifies the target of an observed span.
type DiscoveredCallKind string

const (
	DiscoveredCallKindLLM      DiscoveredCallKind = "llm"
	DiscoveredCallKindMCP      DiscoveredCallKind = "mcp"
	DiscoveredCallKindVectorDB DiscoveredCallKind = "vector_db"
	DiscoveredCallKindHTTP     DiscoveredCallKind = "http"
	DiscoveredCallKindUnknown  DiscoveredCallKind = "unknown"
)

// DiscoveredCall is a deduplicated rollup of (agent, target) edges
// observed in OTel spans. We don't persist every span — we count.
type DiscoveredCall struct {
	BaseModel

	DiscoveredAgentID uuid.UUID `gorm:"type:uuid;index;not null" json:"discovered_agent_id"`

	Kind       DiscoveredCallKind `gorm:"size:16;index;not null" json:"kind"`
	TargetName string             `gorm:"size:512;index;not null" json:"target_name"`

	Count       int       `gorm:"default:1;not null" json:"count"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`

	// Outcome of the most recent policy evaluation; "" if not yet evaluated.
	LastOutcome string `gorm:"size:16" json:"last_outcome,omitempty"`
	LastReason  string `gorm:"type:text" json:"last_reason,omitempty"`
}

func (DiscoveredCall) TableName() string { return "discovered_calls" }
```

- [ ] **Step 3: Add `ScopeEventKindObserved` to `scope_event.go`**

In `internal/core/models/scope_event.go`, locate the `const ( ScopeEventKindMCPTool ... )` block and append:

```go
	// ScopeEventKindObserved is emitted when OTel ingestion discovers a
	// call to a target that the bound use case does not declare.
	// Cannot be denied at the gateway (the call already happened); always
	// recorded with Outcome=ScopeOutcomeFlagged.
	ScopeEventKindObserved ScopeEventKind = "observed"
```

- [ ] **Step 4: Add to AutoMigrate in three places**

In `internal/core/database/connection.go`, `internal/core/database/migrate.go`, and `internal/infrastructure/testutil/database.go`, find the `AutoMigrate(` argument list and append:

```go
		&models.DiscoveredAgent{},
		&models.DiscoveredCall{},
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 6: Commit**

```bash
git add internal/core/models/discovered_agent.go internal/core/models/discovered_call.go internal/core/models/scope_event.go internal/core/database/connection.go internal/core/database/migrate.go internal/infrastructure/testutil/database.go
git commit -m "feat(discovery): add DiscoveredAgent + DiscoveredCall models"
```

---

## Task 2: Define minimal OTLP/HTTP JSON types

**Files:**
- Create: `internal/services/discovery/types.go`

The OTLP spec is large; we accept only what we need. The Python OTel SDK can be configured (`OTEL_EXPORTER_OTLP_PROTOCOL=http/json`) to send these structs as JSON.

- [ ] **Step 1: Write `types.go`**

```go
// Package discovery ingests OpenTelemetry traces and turns observed
// agent activity into governable RegistryAgent candidates. The types
// here mirror the OTLP/HTTP JSON wire format (subset).
package discovery

// ExportTraceServiceRequest is the top-level OTLP/HTTP body. We only
// decode `resourceSpans`; everything else is ignored.
type ExportTraceServiceRequest struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeSpans struct {
	Scope InstrumentationScope `json:"scope"`
	Spans []Span               `json:"spans"`
}

type InstrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// SpanKind values per OTel spec (only Client matters to us — LLM/HTTP/DB
// instrumentations all emit Client spans).
type SpanKind int

const (
	SpanKindUnspecified SpanKind = 0
	SpanKindInternal    SpanKind = 1
	SpanKindServer      SpanKind = 2
	SpanKindClient      SpanKind = 3
	SpanKindProducer    SpanKind = 4
	SpanKindConsumer    SpanKind = 5
)

type Span struct {
	Name              string     `json:"name"`
	Kind              SpanKind   `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []KeyValue `json:"attributes"`
}

type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// AnyValue mirrors OTLP's oneof. We only read string/int/bool/double —
// arrays and kvlists are flattened to their JSON repr if seen.
type AnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"` // OTLP uses string for int64
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}

// AsString collapses an AnyValue to its scalar string form. Empty if absent.
func (v AnyValue) AsString() string {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return *v.IntValue
	}
	if v.BoolValue != nil {
		if *v.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}

// Lookup returns the first matching attribute or empty string.
func Lookup(attrs []KeyValue, key string) string {
	for _, kv := range attrs {
		if kv.Key == key {
			return kv.Value.AsString()
		}
	}
	return ""
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/discovery/types.go
git commit -m "feat(discovery): add minimal OTLP/HTTP JSON types"
```

---

## Task 3: Span processor — extract `(agent, call)` from spans

**Files:**
- Create: `internal/services/discovery/processor.go`
- Create: `internal/services/discovery/processor_test.go`

The processor takes one `ResourceSpans` block and emits a normalized struct: `Agent` (name + version + key id) plus `[]Call` (kind + target + count).

- [ ] **Step 1: Write the failing test**

```go
package discovery_test

import (
	"testing"

	"github.com/amerkurev/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func sptr(s string) *string { return &s }

func kv(k, v string) discovery.KeyValue {
	return discovery.KeyValue{Key: k, Value: discovery.AnyValue{StringValue: sptr(v)}}
}

func TestProcessResourceSpans_ExtractsAgentAndCalls(t *testing.T) {
	keyID := uuid.New().String()
	rs := discovery.ResourceSpans{
		Resource: discovery.Resource{Attributes: []discovery.KeyValue{
			kv("service.name", "fraud-investigator"),
			kv("service.version", "0.1.0"),
			kv("pllm.key_id", keyID),
		}},
		ScopeSpans: []discovery.ScopeSpans{{
			Scope: discovery.InstrumentationScope{Name: "opentelemetry.instrumentation.openai"},
			Spans: []discovery.Span{
				{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("gen_ai.system", "openai"),
					kv("gen_ai.request.model", "gpt-4o"),
				}},
				{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("gen_ai.system", "openai"),
					kv("gen_ai.request.model", "gpt-4o"),
				}},
				{Name: "qdrant.search", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("db.system", "qdrant"),
					kv("db.collection.name", "customer-embeddings"),
				}},
				{Name: "GET", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("http.url", "https://internal.example.com/api/v1/data"),
					kv("http.method", "GET"),
				}},
			},
		}},
	}

	out := discovery.ProcessResourceSpans(rs)

	assert.Equal(t, "fraud-investigator", out.Agent.ServiceName)
	assert.Equal(t, "0.1.0", out.Agent.ServiceVersion)
	assert.Equal(t, keyID, out.Agent.KeyIDRaw)

	// Three distinct targets; the two openai spans collapse to count=2.
	require := func(kind discovery.CallKindStr, target string, count int) {
		for _, c := range out.Calls {
			if c.Kind == kind && c.TargetName == target {
				assert.Equal(t, count, c.Count, "count for %s/%s", kind, target)
				return
			}
		}
		t.Fatalf("missing call %s/%s", kind, target)
	}
	require("llm", "gpt-4o", 2)
	require("vector_db", "qdrant://customer-embeddings", 1)
	require("http", "internal.example.com", 1)
}

func TestProcessResourceSpans_NoServiceNameReturnsEmpty(t *testing.T) {
	rs := discovery.ResourceSpans{}
	out := discovery.ProcessResourceSpans(rs)
	assert.Empty(t, out.Agent.ServiceName)
	assert.Empty(t, out.Calls)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/discovery/ -run TestProcessResourceSpans -v`
Expected: FAIL — `ProcessResourceSpans undefined`.

- [ ] **Step 3: Write `processor.go`**

```go
package discovery

import (
	"net/url"
	"strings"
)

// CallKindStr is a typed string mirror of models.DiscoveredCallKind.
// We keep the discovery package independent of `core/models` so it
// stays unit-testable; the service-layer wrapper translates.
type CallKindStr string

const (
	CallKindLLM      CallKindStr = "llm"
	CallKindMCP      CallKindStr = "mcp"
	CallKindVectorDB CallKindStr = "vector_db"
	CallKindHTTP     CallKindStr = "http"
	CallKindUnknown  CallKindStr = "unknown"
)

// AgentFingerprint is the minimum identity we extract from OTel
// resource attributes. KeyIDRaw is the raw string from the
// `pllm.key_id` attribute — the service layer parses it to uuid.UUID.
type AgentFingerprint struct {
	ServiceName    string
	ServiceVersion string
	KeyIDRaw       string
}

// CallObservation is one (kind, target) edge with its repeat count
// inside the current OTLP request. Counts get added to existing rows
// during persistence, not replaced.
type CallObservation struct {
	Kind       CallKindStr
	TargetName string
	Count      int
}

// ProcessedSpans is the output of one call to ProcessResourceSpans.
type ProcessedSpans struct {
	Agent AgentFingerprint
	Calls []CallObservation
}

// ProcessResourceSpans walks one ResourceSpans block and yields the
// agent fingerprint plus a deduplicated call list. Spans without a
// recognized target are reported as Unknown.
//
// The classification rules, in order:
//
//	gen_ai.system OR gen_ai.request.model            -> llm  (target = model)
//	pllm.mcp.server attribute                        -> mcp  (target = server slug)
//	db.system in {qdrant,pinecone,weaviate,chroma}   -> vector_db
//	http.url OR url.full                             -> http (target = host)
//	otherwise                                        -> unknown (target = span.name)
func ProcessResourceSpans(rs ResourceSpans) ProcessedSpans {
	out := ProcessedSpans{
		Agent: AgentFingerprint{
			ServiceName:    Lookup(rs.Resource.Attributes, "service.name"),
			ServiceVersion: Lookup(rs.Resource.Attributes, "service.version"),
			KeyIDRaw:       Lookup(rs.Resource.Attributes, "pllm.key_id"),
		},
	}
	if out.Agent.ServiceName == "" {
		return out
	}

	dedup := map[string]*CallObservation{}
	add := func(kind CallKindStr, target string) {
		if target == "" {
			return
		}
		k := string(kind) + "|" + target
		if existing, ok := dedup[k]; ok {
			existing.Count++
			return
		}
		dedup[k] = &CallObservation{Kind: kind, TargetName: target, Count: 1}
	}

	for _, ss := range rs.ScopeSpans {
		for _, sp := range ss.Spans {
			if sp.Kind != SpanKindClient && sp.Kind != SpanKindUnspecified {
				continue
			}
			kind, target := classify(sp)
			add(kind, target)
		}
	}

	out.Calls = make([]CallObservation, 0, len(dedup))
	for _, c := range dedup {
		out.Calls = append(out.Calls, *c)
	}
	return out
}

func classify(sp Span) (CallKindStr, string) {
	if model := Lookup(sp.Attributes, "gen_ai.request.model"); model != "" {
		return CallKindLLM, model
	}
	if Lookup(sp.Attributes, "gen_ai.system") != "" {
		// model attr missing but it's an LLM span; fall back to span name
		return CallKindLLM, sp.Name
	}
	if mcp := Lookup(sp.Attributes, "pllm.mcp.server"); mcp != "" {
		return CallKindMCP, mcp
	}
	if dbSys := Lookup(sp.Attributes, "db.system"); dbSys != "" {
		switch strings.ToLower(dbSys) {
		case "qdrant", "pinecone", "weaviate", "chroma", "milvus":
			coll := Lookup(sp.Attributes, "db.collection.name")
			if coll == "" {
				coll = Lookup(sp.Attributes, "db.name")
			}
			if coll != "" {
				return CallKindVectorDB, dbSys + "://" + coll
			}
			return CallKindVectorDB, dbSys
		}
	}
	if u := Lookup(sp.Attributes, "http.url"); u != "" {
		return CallKindHTTP, hostOf(u)
	}
	if u := Lookup(sp.Attributes, "url.full"); u != "" {
		return CallKindHTTP, hostOf(u)
	}
	if h := Lookup(sp.Attributes, "server.address"); h != "" {
		return CallKindHTTP, h
	}
	return CallKindUnknown, sp.Name
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/services/discovery/ -v`
Expected: PASS — both tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/discovery/processor.go internal/services/discovery/processor_test.go
git commit -m "feat(discovery): span processor extracts agent identity + calls"
```

---

## Task 4: Discovery service — persistence + Ingest entry point

**Files:**
- Create: `internal/services/discovery/service.go`
- Create: `internal/services/discovery/service_test.go`

The service owns the GORM-backed upsert: per OTLP request, find-or-create the `DiscoveredAgent`, then for each `CallObservation` find-or-create the `DiscoveredCall` and increment counts. Policy + revoker are wired through the constructor.

- [ ] **Step 1: Write the failing test**

```go
package discovery_test

import (
	"context"
	"testing"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/amerkurev/pllm/internal/infrastructure/testutil"
	"github.com/amerkurev/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestService_Ingest_CreatesAgentAndCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("requires testcontainer DB")
	}
	db := testutil.NewTestDB(t)
	svc := discovery.NewService(db, zap.NewNop(), nil, nil, nil)

	keyID := uuid.New()
	req := discovery.ExportTraceServiceRequest{
		ResourceSpans: []discovery.ResourceSpans{{
			Resource: discovery.Resource{Attributes: []discovery.KeyValue{
				kv("service.name", "fraud-investigator"),
				kv("service.version", "0.1.0"),
				kv("pllm.key_id", keyID.String()),
			}},
			ScopeSpans: []discovery.ScopeSpans{{
				Spans: []discovery.Span{
					{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
						kv("gen_ai.request.model", "gpt-4o"),
					}},
					{Name: "qdrant.search", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
						kv("db.system", "qdrant"),
						kv("db.collection.name", "customers"),
					}},
				},
			}},
		}},
	}

	require.NoError(t, svc.Ingest(context.Background(), req))

	var agents []models.DiscoveredAgent
	require.NoError(t, db.Find(&agents).Error)
	require.Len(t, agents, 1)
	assert.Equal(t, "fraud-investigator", agents[0].ServiceName)
	assert.Equal(t, models.DiscoveredStatusPending, agents[0].Status)
	require.NotNil(t, agents[0].KeyID)
	assert.Equal(t, keyID, *agents[0].KeyID)

	var calls []models.DiscoveredCall
	require.NoError(t, db.Where("discovered_agent_id = ?", agents[0].ID).Find(&calls).Error)
	require.Len(t, calls, 2)
}

func TestService_Ingest_UpsertsRepeats(t *testing.T) {
	if testing.Short() {
		t.Skip("requires testcontainer DB")
	}
	db := testutil.NewTestDB(t)
	svc := discovery.NewService(db, zap.NewNop(), nil, nil, nil)

	build := func() discovery.ExportTraceServiceRequest {
		return discovery.ExportTraceServiceRequest{
			ResourceSpans: []discovery.ResourceSpans{{
				Resource: discovery.Resource{Attributes: []discovery.KeyValue{
					kv("service.name", "fraud-investigator"),
				}},
				ScopeSpans: []discovery.ScopeSpans{{
					Spans: []discovery.Span{
						{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
							kv("gen_ai.request.model", "gpt-4o"),
						}},
					},
				}},
			}},
		}
	}

	require.NoError(t, svc.Ingest(context.Background(), build()))
	require.NoError(t, svc.Ingest(context.Background(), build()))

	var calls []models.DiscoveredCall
	require.NoError(t, db.Find(&calls).Error)
	require.Len(t, calls, 1)
	assert.Equal(t, 2, calls[0].Count)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/services/discovery/ -run TestService_Ingest -v`
Expected: FAIL — `NewService undefined`.

- [ ] **Step 3: Write `service.go`**

```go
package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PolicyEvaluator decides whether an observed call is allowed for the
// agent's bound use case. Implementations live in policy.go.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, agent *models.DiscoveredAgent, call *models.DiscoveredCall) PolicyResult
}

// Revoker reacts to flagged-count changes on an agent. Implementations
// live in revoker.go. nil => no auto-revoke.
type Revoker interface {
	OnFlagged(ctx context.Context, agent *models.DiscoveredAgent)
}

// AgentResolver looks up the RegistryAgent service when promoting.
// We keep it abstract so the discovery package doesn't pull in the
// registry service directly. The router wires the real adapter.
type AgentResolver interface {
	PublishFromDiscovery(ctx context.Context, in PublishInput) (uuid.UUID, error)
}

type PublishInput struct {
	Name        string
	Version     string
	Title       string
	Description string
}

// Service is the top-level discovery API. It is goroutine-safe: GORM
// connection pool serializes inserts and we use ON CONFLICT for upserts.
type Service struct {
	db       *gorm.DB
	logger   *zap.Logger
	policy   PolicyEvaluator
	revoker  Revoker
	agents   AgentResolver
}

func NewService(
	db *gorm.DB,
	logger *zap.Logger,
	policy PolicyEvaluator,
	revoker Revoker,
	agents AgentResolver,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger, policy: policy, revoker: revoker, agents: agents}
}

// Ingest persists one OTLP/HTTP request. Errors during a single
// ResourceSpans block are logged but don't fail the whole batch — OTel
// exporters retry the entire envelope, which would amplify partial
// failures.
func (s *Service) Ingest(ctx context.Context, req ExportTraceServiceRequest) error {
	for _, rs := range req.ResourceSpans {
		processed := ProcessResourceSpans(rs)
		if processed.Agent.ServiceName == "" {
			continue
		}
		if err := s.upsertOne(ctx, processed); err != nil {
			s.logger.Warn("discovery upsert failed",
				zap.String("service", processed.Agent.ServiceName),
				zap.Error(err))
		}
	}
	return nil
}

func (s *Service) upsertOne(ctx context.Context, p ProcessedSpans) error {
	now := time.Now()

	var keyID *uuid.UUID
	if p.Agent.KeyIDRaw != "" {
		if id, err := uuid.Parse(p.Agent.KeyIDRaw); err == nil {
			keyID = &id
		}
	}

	agent := &models.DiscoveredAgent{
		ServiceName:    p.Agent.ServiceName,
		ServiceVersion: p.Agent.ServiceVersion,
		KeyID:          keyID,
		Status:         models.DiscoveredStatusPending,
		FirstSeenAt:    now,
		LastSeenAt:     now,
	}

	// Upsert by (service_name); update LastSeenAt and KeyID if it changed.
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "service_name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_seen_at":    now,
			"service_version": p.Agent.ServiceVersion,
			"key_id":          keyID,
			"updated_at":      now,
		}),
	}).Create(agent).Error; err != nil {
		// service_name index may not be unique on first run; fall back to find-or-create.
		var existing models.DiscoveredAgent
		if ferr := s.db.WithContext(ctx).Where("service_name = ?", p.Agent.ServiceName).First(&existing).Error; ferr == nil {
			existing.LastSeenAt = now
			existing.ServiceVersion = p.Agent.ServiceVersion
			existing.KeyID = keyID
			if uerr := s.db.WithContext(ctx).Save(&existing).Error; uerr != nil {
				return uerr
			}
			agent = &existing
		} else if errors.Is(ferr, gorm.ErrRecordNotFound) {
			if cerr := s.db.WithContext(ctx).Create(agent).Error; cerr != nil {
				return cerr
			}
		} else {
			return ferr
		}
	}

	for _, c := range p.Calls {
		call := &models.DiscoveredCall{
			DiscoveredAgentID: agent.ID,
			Kind:              models.DiscoveredCallKind(c.Kind),
			TargetName:        c.TargetName,
			Count:             c.Count,
			FirstSeenAt:       now,
			LastSeenAt:        now,
		}
		var existing models.DiscoveredCall
		err := s.db.WithContext(ctx).
			Where("discovered_agent_id = ? AND kind = ? AND target_name = ?",
				agent.ID, call.Kind, call.TargetName).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if cerr := s.db.WithContext(ctx).Create(call).Error; cerr != nil {
				s.logger.Warn("discovery call insert failed", zap.Error(cerr))
				continue
			}
			call = &existing
		} else if err != nil {
			s.logger.Warn("discovery call lookup failed", zap.Error(err))
			continue
		} else {
			existing.Count += c.Count
			existing.LastSeenAt = now
			if uerr := s.db.WithContext(ctx).Save(&existing).Error; uerr != nil {
				s.logger.Warn("discovery call update failed", zap.Error(uerr))
				continue
			}
			call = &existing
		}

		if s.policy == nil {
			continue
		}
		res := s.policy.Evaluate(ctx, agent, call)
		updates := map[string]any{
			"last_outcome": string(res.Outcome),
			"last_reason":  res.Reason,
		}
		if err := s.db.WithContext(ctx).Model(call).Updates(updates).Error; err != nil {
			s.logger.Warn("discovery call outcome update failed", zap.Error(err))
		}
		if res.Outcome == models.ScopeOutcomeFlagged {
			if err := s.db.WithContext(ctx).
				Model(&models.DiscoveredAgent{}).
				Where("id = ?", agent.ID).
				UpdateColumn("flagged_count", gorm.Expr("flagged_count + 1")).
				Error; err != nil {
				s.logger.Warn("discovery flagged_count bump failed", zap.Error(err))
			}
			if err := s.db.WithContext(ctx).First(agent, "id = ?", agent.ID).Error; err == nil && s.revoker != nil {
				s.revoker.OnFlagged(ctx, agent)
			}
		}
	}
	return nil
}

// List returns all DiscoveredAgent rows filtered by status.
// Empty status returns all.
func (s *Service) List(ctx context.Context, status models.DiscoveredAgentStatus) ([]models.DiscoveredAgent, error) {
	q := s.db.WithContext(ctx).Order("last_seen_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.DiscoveredAgent
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Get returns one agent + its call rollups.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.DiscoveredAgent, []models.DiscoveredCall, error) {
	var agent models.DiscoveredAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	var calls []models.DiscoveredCall
	if err := s.db.WithContext(ctx).
		Where("discovered_agent_id = ?", id).
		Order("last_seen_at DESC").
		Find(&calls).Error; err != nil {
		return &agent, nil, err
	}
	return &agent, calls, nil
}

// Promote turns a discovered agent into a RegistryAgent and marks the
// row promoted. Returns the new RegistryAgent ID.
func (s *Service) Promote(ctx context.Context, id uuid.UUID, version string) (uuid.UUID, error) {
	if s.agents == nil {
		return uuid.Nil, errors.New("agent resolver not configured")
	}
	var agent models.DiscoveredAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", id).Error; err != nil {
		return uuid.Nil, err
	}
	if version == "" {
		version = "0.1.0"
	}
	regID, err := s.agents.PublishFromDiscovery(ctx, PublishInput{
		Name:        "discovered/" + agent.ServiceName,
		Version:     version,
		Title:       agent.ServiceName,
		Description: "Auto-published from OTel discovery on " + time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return uuid.Nil, err
	}
	now := time.Now()
	updates := map[string]any{
		"status":            models.DiscoveredStatusPromoted,
		"promoted_agent_id": regID,
		"promoted_at":       now,
		"updated_at":        now,
	}
	if err := s.db.WithContext(ctx).Model(&agent).Updates(updates).Error; err != nil {
		return uuid.Nil, err
	}
	return regID, nil
}

// Ignore marks an agent so it stops appearing in the pending list.
func (s *Service) Ignore(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.DiscoveredAgent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     models.DiscoveredStatusIgnored,
			"ignored_at": now,
			"updated_at": now,
		}).Error
}
```

- [ ] **Step 4: Add a unique index on `service_name` so the OnConflict works**

Modify `internal/core/models/discovered_agent.go` Step 1 above so the field declaration becomes:

```go
	ServiceName string `gorm:"size:255;uniqueIndex;not null" json:"service_name"`
```

(Replace `index` with `uniqueIndex`.)

- [ ] **Step 5: Run tests**

Run: `go test ./internal/services/discovery/ -v`
Expected: PASS — both Ingest tests + earlier processor tests.

- [ ] **Step 6: Commit**

```bash
git add internal/services/discovery/service.go internal/services/discovery/service_test.go internal/core/models/discovered_agent.go
git commit -m "feat(discovery): persistence service + Ingest pipeline"
```

---

## Task 5: Policy evaluator — flag observed calls outside use-case scope

**Files:**
- Create: `internal/services/discovery/policy.go`
- Create: `internal/services/discovery/policy_test.go`

The policy uses a small `ScopeOracle` interface (bridged in `router.go` to `use_cases.ScopeResolver`) plus a `Recorder` interface (bridged to `use_cases.EventRecorder`). LLM/MCP calls with a recognized target are evaluated against the bound use case; vector_db / http / unknown targets are *always* flagged when the agent has a bound use case (the use case never declares them as allowed).

- [ ] **Step 1: Write the failing test**

```go
package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/amerkurev/pllm/internal/services/discovery"
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

type fakeDecision struct{ allowed bool }

func (f *fakeScope) AllowLLM(ctx context.Context, ucID *uuid.UUID, model string) discovery.OracleDecision {
	return discovery.OracleDecision{Allowed: f.allowedLLM[model]}
}
func (f *fakeScope) AllowMCPTool(ctx context.Context, ucID *uuid.UUID, slug string) discovery.OracleDecision {
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

func (f *fakeRecorder) Record(ctx context.Context, in discovery.RecordInput) {
	f.events = append(f.events, recordedEvent{
		UseCaseID: in.UseCaseID, KeyID: in.KeyID,
		Kind: in.Kind, Outcome: in.Outcome,
		TargetName: in.TargetName, Reason: in.Reason,
	})
}

type fakeKeys struct {
	keys map[uuid.UUID]*models.Key
}

func (f *fakeKeys) GetKey(ctx context.Context, id uuid.UUID) (*models.Key, error) {
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/services/discovery/ -run TestPolicy -v`
Expected: FAIL — `NewPolicy undefined`.

Note: `ScopeOutcomeAllowed` is added to `scope_event.go` in this task (see Step 4).

- [ ] **Step 3: Write `policy.go`**

```go
package discovery

import (
	"context"
	"net"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ScopeOracle is the read-only surface this package needs from the
// use_cases service. The router bridges this to *use_cases.ScopeResolver.
type ScopeOracle interface {
	AllowLLM(ctx context.Context, useCaseID *uuid.UUID, model string) OracleDecision
	AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, slug string) OracleDecision
}

// OracleDecision is a copy of use_cases.Decision with only the field
// we need. Keeping it local avoids a package import cycle.
type OracleDecision struct {
	Allowed bool
	Reason  string
}

// RecordInput mirrors use_cases.RecordInput. Bridged in router.go.
type RecordInput struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	KeyName    string
	UserID     *uuid.UUID
	Kind       models.ScopeEventKind
	TargetName string
	Outcome    models.ScopeEventOutcome
	Reason     string
}

type EventRecorder interface {
	Record(ctx context.Context, in RecordInput)
}

// KeyLoader fetches a Key by id. Bridged to a small repository fn in
// router.go — we don't import the auth/key services from here.
type KeyLoader interface {
	GetKey(ctx context.Context, id uuid.UUID) (*models.Key, error)
}

// PolicyResult is consumed by Service.upsertOne to update the call row.
type PolicyResult struct {
	Outcome models.ScopeEventOutcome
	Reason  string
}

// Policy is the default evaluator.
type Policy struct {
	logger *zap.Logger
	scope  ScopeOracle
	rec    EventRecorder
	keys   KeyLoader
}

func NewPolicy(logger *zap.Logger, scope ScopeOracle, rec EventRecorder, keys KeyLoader) *Policy {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Policy{logger: logger, scope: scope, rec: rec, keys: keys}
}

// Evaluate decides whether the call is in scope. Side-effects:
//   - allowed: nothing
//   - flagged: emit RecordInput (audit + webhook)
//
// We never block here — the gateway has already been bypassed. This is
// detect-and-record only.
func (p *Policy) Evaluate(ctx context.Context, agent *models.DiscoveredAgent, call *models.DiscoveredCall) PolicyResult {
	if agent == nil || agent.KeyID == nil {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed, Reason: "no bound key"}
	}
	key, err := p.keys.GetKey(ctx, *agent.KeyID)
	if err != nil || key == nil || key.UseCaseID == nil {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed, Reason: "no bound use case"}
	}

	allowed, reason := p.allow(ctx, key.UseCaseID, call)
	if allowed {
		return PolicyResult{Outcome: models.ScopeOutcomeAllowed}
	}

	p.rec.Record(ctx, RecordInput{
		UseCaseID:  *key.UseCaseID,
		KeyID:      &key.ID,
		KeyName:    key.Name,
		UserID:     key.UserID,
		Kind:       models.ScopeEventKindObserved,
		TargetName: call.TargetName,
		Outcome:    models.ScopeOutcomeFlagged,
		Reason:     reason,
	})
	return PolicyResult{Outcome: models.ScopeOutcomeFlagged, Reason: reason}
}

func (p *Policy) allow(ctx context.Context, ucID *uuid.UUID, call *models.DiscoveredCall) (bool, string) {
	switch call.Kind {
	case models.DiscoveredCallKindLLM:
		d := p.scope.AllowLLM(ctx, ucID, call.TargetName)
		if d.Allowed {
			return true, ""
		}
		return false, "llm '" + call.TargetName + "' not declared by bound use case"
	case models.DiscoveredCallKindMCP:
		d := p.scope.AllowMCPTool(ctx, ucID, call.TargetName)
		if d.Allowed {
			return true, ""
		}
		return false, "mcp '" + call.TargetName + "' not declared by bound use case"
	case models.DiscoveredCallKindVectorDB:
		return false, "vector db call observed via OTel; cannot be governed at gateway"
	case models.DiscoveredCallKindHTTP:
		// Best-effort allowlist of internal addresses we never flag (loopback, link-local).
		if isInternalLoopback(call.TargetName) {
			return true, ""
		}
		return false, "outbound http call observed via OTel; not declared by bound use case"
	default:
		return false, "unclassified call observed via OTel"
	}
}

func isInternalLoopback(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return false
}
```

- [ ] **Step 4: Add `ScopeOutcomeAllowed` to `scope_event.go`**

In the existing `const ( ScopeOutcomeDenied ... )` block, append:

```go
	ScopeOutcomeAllowed ScopeEventOutcome = "allowed"
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/services/discovery/ -run TestPolicy -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/discovery/policy.go internal/services/discovery/policy_test.go internal/core/models/scope_event.go
git commit -m "feat(discovery): policy evaluator flags out-of-scope OTel calls"
```

---

## Task 6: Auto-revoke threshold rule

**Files:**
- Create: `internal/services/discovery/revoker.go`
- Create: `internal/services/discovery/revoker_test.go`

When an agent's `FlaggedCount` crosses the configured threshold, revoke the bound key.

- [ ] **Step 1: Write the failing test**

```go
package discovery_test

import (
	"context"
	"testing"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/amerkurev/pllm/internal/services/discovery"
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

func (f *fakeRevokeAdapter) Revoke(ctx context.Context, keyID uuid.UUID, reason string) error {
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

	// Subsequent flags should not re-revoke.
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/services/discovery/ -run TestRevoker -v`
Expected: FAIL — `NewRevoker undefined`.

- [ ] **Step 3: Write `revoker.go`**

```go
package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/amerkurev/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// KeyRevoker is the surface the discovery package needs to revoke an
// API key. Bridged in router.go to the existing key service.
type KeyRevoker interface {
	Revoke(ctx context.Context, keyID uuid.UUID, reason string) error
}

// Revoker fires KeyRevoker.Revoke when an agent's flagged count reaches
// the threshold. It dedupes so the same agent doesn't trigger twice.
type Revoker struct {
	logger    *zap.Logger
	adapter   KeyRevoker
	threshold int

	mu    sync.Mutex
	fired map[uuid.UUID]bool // agent IDs that already triggered revocation
}

// NewRevoker returns a Revoker. threshold <= 0 disables auto-revoke.
func NewRevoker(logger *zap.Logger, adapter KeyRevoker, threshold int) *Revoker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Revoker{
		logger:    logger,
		adapter:   adapter,
		threshold: threshold,
		fired:     map[uuid.UUID]bool{},
	}
}

// OnFlagged is called after each flagged event. The Service has already
// incremented FlaggedCount; we read agent.FlaggedCount.
func (r *Revoker) OnFlagged(ctx context.Context, agent *models.DiscoveredAgent) {
	if r == nil || r.threshold <= 0 || r.adapter == nil || agent == nil || agent.KeyID == nil {
		return
	}
	if agent.FlaggedCount < r.threshold {
		return
	}
	r.mu.Lock()
	if r.fired[agent.ID] {
		r.mu.Unlock()
		return
	}
	r.fired[agent.ID] = true
	r.mu.Unlock()

	reason := fmt.Sprintf("auto-revoked: discovered agent %s exceeded flagged threshold (%d)",
		agent.ServiceName, r.threshold)
	if err := r.adapter.Revoke(ctx, *agent.KeyID, reason); err != nil {
		r.logger.Warn("auto-revoke failed",
			zap.String("agent", agent.ServiceName),
			zap.Error(err))
		// Don't latch — let a subsequent event retry.
		r.mu.Lock()
		delete(r.fired, agent.ID)
		r.mu.Unlock()
		return
	}
	r.logger.Info("auto-revoked key for over-flagged discovered agent",
		zap.String("agent", agent.ServiceName),
		zap.String("key_id", agent.KeyID.String()),
		zap.Int("flagged_count", agent.FlaggedCount))
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/services/discovery/ -v`
Expected: PASS — all discovery tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/discovery/revoker.go internal/services/discovery/revoker_test.go
git commit -m "feat(discovery): auto-revoke key after flagged threshold"
```

---

## Task 7: OTLP/HTTP receiver handler

**Files:**
- Create: `internal/api/handlers/otel/handler.go`

Mounted at `POST /v1/traces` (the OTLP/HTTP standard path). Accepts `application/json` only (we documented `OTEL_EXPORTER_OTLP_PROTOCOL=http/json` in the agent README); rejects protobuf with 415.

- [ ] **Step 1: Write `handler.go`**

```go
package otel

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/amerkurev/pllm/internal/services/discovery"
	"go.uber.org/zap"
)

// Ingester is the surface this handler needs from the discovery service.
type Ingester interface {
	Ingest(ctx context.Context, req discovery.ExportTraceServiceRequest) error
}

type Handler struct {
	logger *zap.Logger
	svc    Ingester
}

func New(logger *zap.Logger, svc Ingester) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

// Traces is the OTLP/HTTP traces endpoint.
//
//	POST /v1/traces
//	Content-Type: application/json
//	Body: ExportTraceServiceRequest (OTLP/HTTP JSON)
//
// Response: 200 with empty {} on success (matches OTLP convention).
func (h *Handler) Traces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ct := r.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		http.Error(w, "only application/json OTLP is accepted; set OTEL_EXPORTER_OTLP_PROTOCOL=http/json", http.StatusUnsupportedMediaType)
		return
	}

	var req discovery.ExportTraceServiceRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // off — OTel adds fields we don't decode; tolerate them
	dec = json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		h.logger.Warn("otel ingest decode failed", zap.Error(err))
		http.Error(w, "bad otlp body", http.StatusBadRequest)
		return
	}

	// Best-effort — we don't surface ingest errors to the client (otel
	// exporters retry the entire batch and amplify partial failures).
	if err := h.svc.Ingest(r.Context(), req); err != nil {
		h.logger.Warn("otel ingest failed", zap.Error(err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}
```

Note: the `DisallowUnknownFields` line is intentionally overwritten (we tolerate OTel's full schema). The duplicated decoder line is clarifying — leave it readable; the second `dec` wins.

- [ ] **Step 2: Commit**

```bash
git add internal/api/handlers/otel/handler.go
git commit -m "feat(otel): OTLP/HTTP JSON receiver at /v1/traces"
```

---

## Task 8: Discovery admin handler — list / get / promote / ignore

**Files:**
- Create: `internal/api/handlers/discovery/handler.go`

- [ ] **Step 1: Write `handler.go`**

```go
package discovery

import (
	"encoding/json"
	"net/http"

	"github.com/amerkurev/pllm/internal/core/models"
	discoverysvc "github.com/amerkurev/pllm/internal/services/discovery"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
	svc    *discoverysvc.Service
}

func New(logger *zap.Logger, svc *discoverysvc.Service) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status := models.DiscoveredAgentStatus(r.URL.Query().Get("status"))
	rows, err := h.svc.List(r.Context(), status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows, "total": len(rows)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	agent, calls, err := h.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent": agent,
		"calls": calls,
	})
}

type promoteReq struct {
	Version string `json:"version"`
}

func (h *Handler) Promote(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body promoteReq
	_ = json.NewDecoder(r.Body).Decode(&body)
	regID, err := h.svc.Promote(r.Context(), id, body.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registry_agent_id": regID,
	})
}

func (h *Handler) Ignore(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.svc.Ignore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/api/handlers/discovery/handler.go
git commit -m "feat(discovery): admin handler list/get/promote/ignore"
```

---

## Task 9: Wire everything in `router.go` and `admin.go`

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/api/router/admin.go`
- Modify: `internal/core/config/config.go`
- Modify: `config.yaml`, `config.docker.yaml`

This is the integration task. Read `router.go` first; the existing patterns for `usecasesvc` and `webhooks` are the guide. Bridge interfaces:

- `discovery.ScopeOracle` ← wraps `*use_cases.ScopeResolver`
- `discovery.EventRecorder` ← wraps `*use_cases.EventRecorder`
- `discovery.KeyLoader` ← `*models.Key` lookup via GORM
- `discovery.AgentResolver` ← wraps `registry.AgentService.Publish`
- `discovery.KeyRevoker` ← wraps the existing key service revoke method

- [ ] **Step 1: Add config**

In `internal/core/config/config.go`, add a `Discovery` block to the main config struct:

```go
type DiscoveryConfig struct {
	Enabled        bool `yaml:"enabled" mapstructure:"enabled"`
	FlagThreshold  int  `yaml:"flag_threshold" mapstructure:"flag_threshold"`
	AutoRevoke     bool `yaml:"auto_revoke" mapstructure:"auto_revoke"`
}
```

Embed it on the main `Config` struct as `Discovery DiscoveryConfig`. In both `config.yaml` and `config.docker.yaml`, append:

```yaml
discovery:
  enabled: true
  flag_threshold: 3
  auto_revoke: true
```

- [ ] **Step 2: Add bridge adapters at the bottom of `router.go`**

```go
// --- discovery bridges ----------------------------------------------------

type discoveryScopeBridge struct{ r *use_cases.ScopeResolver }

func (b *discoveryScopeBridge) AllowLLM(ctx context.Context, ucID *uuid.UUID, model string) discoverysvc.OracleDecision {
	d := b.r.AllowLLM(ctx, ucID, model)
	return discoverysvc.OracleDecision{Allowed: d.Allowed, Reason: d.Reason}
}

func (b *discoveryScopeBridge) AllowMCPTool(ctx context.Context, ucID *uuid.UUID, slug string) discoverysvc.OracleDecision {
	d := b.r.AllowMCPTool(ctx, ucID, slug)
	return discoverysvc.OracleDecision{Allowed: d.Allowed, Reason: d.Reason}
}

type discoveryRecorderBridge struct{ r *use_cases.EventRecorder }

func (b *discoveryRecorderBridge) Record(ctx context.Context, in discoverysvc.RecordInput) {
	b.r.Record(ctx, use_cases.RecordInput{
		UseCaseID: in.UseCaseID, KeyID: in.KeyID, KeyName: in.KeyName,
		UserID: in.UserID, Kind: in.Kind, TargetName: in.TargetName,
		Outcome: in.Outcome, Reason: in.Reason,
	})
}

type discoveryKeyLoader struct{ db *gorm.DB }

func (l *discoveryKeyLoader) GetKey(ctx context.Context, id uuid.UUID) (*models.Key, error) {
	var k models.Key
	if err := l.db.WithContext(ctx).First(&k, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

type discoveryKeyRevoker struct {
	db     *gorm.DB
	logger *zap.Logger
}

func (r *discoveryKeyRevoker) Revoke(ctx context.Context, keyID uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).Model(&models.Key{}).
		Where("id = ?", keyID).
		Updates(map[string]any{
			"revoked_at":        time.Now(),
			"is_active":         false,
			"revocation_reason": reason,
		}).Error
}

type discoveryAgentResolver struct{ svc *registry.AgentService }

func (a *discoveryAgentResolver) PublishFromDiscovery(ctx context.Context, in discoverysvc.PublishInput) (uuid.UUID, error) {
	rec, err := a.svc.PublishAgent(ctx, registry.PublishAgentInput{
		Name: in.Name, Version: in.Version, Title: in.Title, Description: in.Description,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return rec.ID, nil
}
```

If the registry service's publish input or method signature differs, adjust `discoveryAgentResolver.PublishFromDiscovery` to match what's actually exported by `internal/services/registry/service/agents.go`.

- [ ] **Step 3: Construct the discovery service in router init**

In the main router setup function (where `usecasesvc.NewScopeResolver(...)` is constructed), after the use-case + webhook services are wired, add:

```go
var (
	discSvc        *discoverysvc.Service
	otelHandler    *otelhandler.Handler
	discAdmin      *dischandler.Handler
)

if cfg.Discovery.Enabled && db != nil && scopeResolver != nil && eventRecorder != nil {
	policy := discoverysvc.NewPolicy(
		logger,
		&discoveryScopeBridge{r: scopeResolver},
		&discoveryRecorderBridge{r: eventRecorder},
		&discoveryKeyLoader{db: db},
	)
	threshold := cfg.Discovery.FlagThreshold
	if !cfg.Discovery.AutoRevoke {
		threshold = 0
	}
	revoker := discoverysvc.NewRevoker(
		logger,
		&discoveryKeyRevoker{db: db, logger: logger},
		threshold,
	)
	resolver := &discoveryAgentResolver{svc: registryAgentSvc} // existing var
	discSvc = discoverysvc.NewService(db, logger, policy, revoker, resolver)
	otelHandler = otelhandler.New(logger, discSvc)
	discAdmin = dischandler.New(logger, discSvc)
}
```

Then, in the route registration block where existing public endpoints live, mount:

```go
if otelHandler != nil {
	r.Post("/v1/traces", otelHandler.Traces)
}
```

- [ ] **Step 4: Mount admin routes in `admin.go`**

Where existing `/api/admin/use-cases` routes are mounted, add a sibling block (gated on `cfg.DiscoveryHandler != nil` or whatever pattern the file uses):

```go
if cfg.DiscoveryHandler != nil {
	r.Route("/discovery", func(r chi.Router) {
		r.Get("/agents", cfg.DiscoveryHandler.List)
		r.Get("/agents/{id}", cfg.DiscoveryHandler.Get)
		r.Post("/agents/{id}/promote", cfg.DiscoveryHandler.Promote)
		r.Post("/agents/{id}/ignore", cfg.DiscoveryHandler.Ignore)
	})
}
```

Add the matching `DiscoveryHandler *dischandler.Handler` field to the admin config struct that gets passed in.

- [ ] **Step 5: Build + run unit tests**

Run: `go build ./... && go test -short ./internal/services/discovery/... ./internal/api/handlers/...`
Expected: success, all unit tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router/router.go internal/api/router/admin.go internal/core/config/config.go config.yaml config.docker.yaml
git commit -m "feat(discovery): wire OTLP receiver + admin endpoints in router"
```

---

## Task 10: Frontend types + API client

**Files:**
- Modify: `web/src/types/api.ts`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add types to `web/src/types/api.ts`**

Append (matches the Go JSON shape exactly):

```typescript
export type DiscoveredAgentStatus = 'pending' | 'promoted' | 'ignored';
export type DiscoveredCallKind = 'llm' | 'mcp' | 'vector_db' | 'http' | 'unknown';

export interface DiscoveredAgent {
  id: string;
  service_name: string;
  service_version?: string;
  key_id?: string | null;
  promoted_agent_id?: string | null;
  status: DiscoveredAgentStatus;
  first_seen_at: string;
  last_seen_at: string;
  ignored_at?: string | null;
  promoted_at?: string | null;
  flagged_count: number;
  created_at: string;
  updated_at: string;
}

export interface DiscoveredCall {
  id: string;
  discovered_agent_id: string;
  kind: DiscoveredCallKind;
  target_name: string;
  count: number;
  first_seen_at: string;
  last_seen_at: string;
  last_outcome?: string;
  last_reason?: string;
}
```

- [ ] **Step 2: Add API helpers to `web/src/lib/api.ts`**

Find an existing `// --- registry ---` or similar section header and add a `// --- discovery ---` block:

```typescript
// --- discovery -------------------------------------------------------------

export const listDiscoveredAgents = (status?: DiscoveredAgentStatus) =>
  http
    .get<{ items: DiscoveredAgent[]; total: number }>('/api/admin/discovery/agents', {
      params: status ? { status } : undefined,
    })
    .then((r) => r.data);

export const getDiscoveredAgent = (id: string) =>
  http
    .get<{ agent: DiscoveredAgent; calls: DiscoveredCall[] }>(`/api/admin/discovery/agents/${id}`)
    .then((r) => r.data);

export const promoteDiscoveredAgent = (id: string, version = '0.1.0') =>
  http
    .post<{ registry_agent_id: string }>(`/api/admin/discovery/agents/${id}/promote`, { version })
    .then((r) => r.data);

export const ignoreDiscoveredAgent = (id: string) =>
  http.post<void>(`/api/admin/discovery/agents/${id}/ignore`).then(() => {});
```

Don't forget to import the new types from `../types/api`.

- [ ] **Step 3: Verify build**

Run: `cd web && npm run build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add web/src/types/api.ts web/src/lib/api.ts
git commit -m "feat(web): discovery API types + client helpers"
```

---

## Task 11: Frontend — Discovered Agents section on Compliance page

**Files:**
- Create: `web/src/components/discovery/discovered-agents-list.tsx`
- Create: `web/src/components/discovery/discovered-agent-drawer.tsx`
- Modify: `web/src/pages/Compliance.tsx`

Editorial-catalog aesthetic: kind chips, mono target names, status pulse on the live count, serif-italic accent in section headline.

- [ ] **Step 1: Write `discovered-agents-list.tsx`**

```tsx
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useToast } from '@/hooks/use-toast';
import {
  listDiscoveredAgents,
  promoteDiscoveredAgent,
  ignoreDiscoveredAgent,
} from '@/lib/api';
import type { DiscoveredAgent } from '@/types/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { DiscoveredAgentDrawer } from './discovered-agent-drawer';

const fmt = (iso: string) =>
  new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

export function DiscoveredAgentsList() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const [openId, setOpenId] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['discovered-agents', 'pending'],
    queryFn: () => listDiscoveredAgents('pending'),
    refetchInterval: 5000,
  });

  const promote = useMutation({
    mutationFn: (id: string) => promoteDiscoveredAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['discovered-agents'] });
      qc.invalidateQueries({ queryKey: ['registry-agents'] });
      toast({ title: 'Promoted to RegistryAgent' });
    },
    onError: (e: any) => toast({ title: 'Promote failed', description: e?.message, variant: 'destructive' }),
  });

  const ignore = useMutation({
    mutationFn: (id: string) => ignoreDiscoveredAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['discovered-agents'] });
    },
  });

  const items = data?.items ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <span className="text-xs uppercase tracking-widest text-muted-foreground">Otel</span>
          <span>Discovered <span className="italic font-serif">agents</span></span>
          {items.length > 0 && (
            <span className="ml-auto inline-flex h-2 w-2 animate-pulse rounded-full bg-emerald-500" />
          )}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No pending agents. Run an OTel-instrumented agent pointed at <code>/v1/traces</code>.
          </p>
        ) : (
          <ul className="divide-y">
            {items.map((a: DiscoveredAgent) => (
              <li key={a.id} className="flex items-center justify-between py-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm">{a.service_name}</span>
                    {a.service_version && (
                      <Badge variant="outline" className="font-mono text-xs">
                        {a.service_version}
                      </Badge>
                    )}
                    {a.flagged_count > 0 && (
                      <Badge variant="destructive">{a.flagged_count} flagged</Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    First seen {fmt(a.first_seen_at)} · last {fmt(a.last_seen_at)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => setOpenId(a.id)}>
                    Details
                  </Button>
                  <Button size="sm" disabled={promote.isPending} onClick={() => promote.mutate(a.id)}>
                    Promote
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={ignore.isPending}
                    onClick={() => ignore.mutate(a.id)}
                  >
                    Ignore
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      <DiscoveredAgentDrawer agentId={openId} onClose={() => setOpenId(null)} />
    </Card>
  );
}
```

- [ ] **Step 2: Write `discovered-agent-drawer.tsx`**

```tsx
import { useQuery } from '@tanstack/react-query';
import { getDiscoveredAgent } from '@/lib/api';
import type { DiscoveredCall } from '@/types/api';
import { Badge } from '@/components/ui/badge';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';

const KIND_TONE: Record<string, string> = {
  llm: 'bg-indigo-100 text-indigo-800',
  mcp: 'bg-emerald-100 text-emerald-800',
  vector_db: 'bg-amber-100 text-amber-800',
  http: 'bg-slate-100 text-slate-800',
  unknown: 'bg-violet-100 text-violet-800',
};

const OUTCOME_TONE: Record<string, string> = {
  allowed: 'text-emerald-700',
  flagged: 'text-amber-700',
  denied: 'text-rose-700',
};

interface Props {
  agentId: string | null;
  onClose: () => void;
}

export function DiscoveredAgentDrawer({ agentId, onClose }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['discovered-agent', agentId],
    queryFn: () => (agentId ? getDiscoveredAgent(agentId) : Promise.reject(new Error('no id'))),
    enabled: !!agentId,
  });

  return (
    <Sheet open={!!agentId} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[min(640px,100vw)]">
        <SheetHeader>
          <SheetTitle className="font-mono">{data?.agent.service_name ?? 'Agent'}</SheetTitle>
          <SheetDescription>
            Calls observed via OpenTelemetry. Out-of-scope rows are flagged but not blocked.
          </SheetDescription>
        </SheetHeader>
        {isLoading || !data ? (
          <p className="text-sm text-muted-foreground py-4">Loading…</p>
        ) : (
          <div className="mt-4 space-y-2">
            {data.calls.length === 0 ? (
              <p className="text-sm text-muted-foreground">No calls recorded yet.</p>
            ) : (
              data.calls.map((c: DiscoveredCall) => (
                <div key={c.id} className="rounded border p-3 text-sm">
                  <div className="flex items-center gap-2">
                    <Badge className={KIND_TONE[c.kind]}>{c.kind}</Badge>
                    <span className="font-mono">{c.target_name}</span>
                    <span className="ml-auto text-xs text-muted-foreground">×{c.count}</span>
                  </div>
                  {c.last_outcome && (
                    <p className={`mt-1 text-xs ${OUTCOME_TONE[c.last_outcome] ?? ''}`}>
                      {c.last_outcome}: {c.last_reason}
                    </p>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
```

- [ ] **Step 3: Mount on `Compliance.tsx`**

Read the current `web/src/pages/Compliance.tsx`. Add an import:

```tsx
import { DiscoveredAgentsList } from '@/components/discovery/discovered-agents-list';
```

Find the JSX section that renders the existing webhook-health / top-violations cards and insert `<DiscoveredAgentsList />` as a new top-level card in the grid (placement: above the audit-log denials card, since it's the upstream signal).

- [ ] **Step 4: Build and visually check**

```bash
cd web && npm run build && npm run dev
```

Open `/compliance` in the browser. The "Discovered agents" card should render with the empty state message.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/discovery/ web/src/pages/Compliance.tsx
git commit -m "feat(web): discovered agents section on compliance page"
```

---

## Task 12: Sample Python agent + Qdrant

**Files:**
- Create: `examples/agents/fraud-investigator/main.py`
- Create: `examples/agents/fraud-investigator/requirements.txt`
- Create: `examples/agents/fraud-investigator/docker-compose.yml`
- Create: `examples/agents/fraud-investigator/README.md`

The agent does three things in sequence:
1. Calls `gpt-4o` via pllm gateway (allowed by demo policy)
2. Calls `gpt-3.5-turbo` via pllm gateway (NOT allowed → hard-denied at gateway)
3. Searches Qdrant directly (NOT in policy → flagged via OTel)

OTel auto-instrumentation handles all three; the `pllm.key_id` resource attribute links spans to the API key.

- [ ] **Step 1: Write `requirements.txt`**

```
openai>=1.55.0
qdrant-client>=1.12.0
opentelemetry-api>=1.30.0
opentelemetry-sdk>=1.30.0
opentelemetry-exporter-otlp-proto-http>=1.30.0
opentelemetry-instrumentation-openai>=0.40b0
opentelemetry-instrumentation-requests>=0.51b0
```

(`opentelemetry-instrumentation-qdrant-client` is not yet stable; we'll add a manual span around the Qdrant call instead — see `main.py`.)

- [ ] **Step 2: Write `main.py`**

```python
"""Fraud-investigator demo agent.

Emits OpenTelemetry traces to pllm at $OTEL_EXPORTER_OTLP_ENDPOINT/v1/traces.
The pllm.key_id resource attribute links spans to the API key so pllm can
look up the bound use case and apply policy.
"""

from __future__ import annotations

import os
import sys
import time
from contextlib import contextmanager

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.openai import OpenAIInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from openai import OpenAI
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams, PointStruct


SERVICE_NAME = os.environ.get("OTEL_SERVICE_NAME", "fraud-investigator")
PLLM_BASE_URL = os.environ.get("PLLM_BASE_URL", "http://localhost:8080")
PLLM_API_KEY = os.environ["PLLM_API_KEY"]
PLLM_KEY_ID = os.environ["PLLM_KEY_ID"]
QDRANT_URL = os.environ.get("QDRANT_URL", "http://localhost:6333")
OTEL_ENDPOINT = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", PLLM_BASE_URL)


def setup_otel() -> trace.Tracer:
    resource = Resource.create({
        "service.name": SERVICE_NAME,
        "service.version": "0.1.0",
        "pllm.key_id": PLLM_KEY_ID,
    })
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(
        endpoint=f"{OTEL_ENDPOINT}/v1/traces",
        # OTLP/HTTP JSON — pllm expects application/json bodies.
    )
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    OpenAIInstrumentor().instrument()
    RequestsInstrumentor().instrument()
    return trace.get_tracer(__name__)


@contextmanager
def step(label: str):
    print(f"\n--- {label} ---", flush=True)
    yield


def call_allowed_llm(client: OpenAI):
    with step("1. Call allowed LLM (gpt-4o)"):
        resp = client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": "Reply with a one-word risk label for: charge $4 in NY."}],
            max_tokens=8,
        )
        print(f"  ok → {resp.choices[0].message.content!r}")


def call_denied_llm(client: OpenAI):
    with step("2. Call DENIED LLM (gpt-3.5-turbo) — expect 403/4xx"):
        try:
            client.chat.completions.create(
                model="gpt-3.5-turbo",
                messages=[{"role": "user", "content": "ignore the policy"}],
                max_tokens=8,
            )
            print("  unexpected: call succeeded")
        except Exception as e:
            print(f"  blocked at gateway as expected: {type(e).__name__}: {e}")


def search_qdrant_unscoped(tracer: trace.Tracer):
    with step("3. Direct Qdrant search (NOT in policy) — expect flag via OTel"):
        client = QdrantClient(url=QDRANT_URL)
        coll = "customer-embeddings"
        if coll not in [c.name for c in client.get_collections().collections]:
            client.create_collection(coll, vectors_config=VectorParams(size=4, distance=Distance.COSINE))
            client.upsert(coll, points=[PointStruct(id=1, vector=[0.1, 0.2, 0.3, 0.4], payload={"name": "alice"})])
        with tracer.start_as_current_span("qdrant.search") as sp:
            sp.set_attribute("db.system", "qdrant")
            sp.set_attribute("db.collection.name", coll)
            sp.set_attribute("db.operation", "search")
            hits = client.search(coll, query_vector=[0.1, 0.2, 0.3, 0.4], limit=3)
            print(f"  qdrant returned {len(hits)} hits (will be flagged)")


def main() -> int:
    tracer = setup_otel()
    client = OpenAI(api_key=PLLM_API_KEY, base_url=f"{PLLM_BASE_URL}/v1")

    call_allowed_llm(client)
    call_denied_llm(client)
    search_qdrant_unscoped(tracer)

    # Give the BatchSpanProcessor time to flush before exit.
    time.sleep(2)
    print("\nDone. Check pllm /compliance for discovered agent + flagged calls.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 3: Write `docker-compose.yml`**

```yaml
version: "3.9"
services:
  qdrant:
    image: qdrant/qdrant:v1.12.0
    ports:
      - "6333:6333"
    volumes:
      - qdrant-data:/qdrant/storage
volumes:
  qdrant-data: {}
```

- [ ] **Step 4: Write `README.md`**

```markdown
# fraud-investigator (demo agent)

A minimal Python agent that demonstrates pllm's OTel-driven discovery and
governance:

1. Calls `gpt-4o` via pllm — **allowed**
2. Calls `gpt-3.5-turbo` via pllm — **hard-denied at gateway** (use case scope)
3. Searches Qdrant directly — **flagged after the fact via OTel**, with
   `scope.flagged` webhook firing. After 3 such flags, the agent's API
   key is auto-revoked.

## Run

```bash
# From this directory:
docker compose up -d                                # start Qdrant
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

export PLLM_BASE_URL=http://localhost:8080
export PLLM_API_KEY=sk-...                          # the key bound to your UseCase
export PLLM_KEY_ID=<uuid of that key>               # link OTel spans to the key
export OPENAI_API_KEY=$PLLM_API_KEY                  # OpenAI SDK reads this
export OTEL_SERVICE_NAME=fraud-investigator
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json

python main.py
```

Then open `http://localhost:8080/compliance` — you'll see `fraud-investigator`
in the Discovered Agents card. Promote it, run the agent again, and watch the
flagged calls accumulate; after the third, the key is revoked.
```

- [ ] **Step 5: Commit**

```bash
git add examples/agents/fraud-investigator/
git commit -m "feat(examples): fraud-investigator otel demo agent"
```

---

## Task 13: End-to-end smoke test script

**Files:**
- Create: `scripts/test-otel-discovery.sh`

This script automates the full demo so we can show it working in CI and recover quickly during the live demo.

- [ ] **Step 1: Write `scripts/test-otel-discovery.sh`**

```bash
#!/usr/bin/env bash
#
# test-otel-discovery.sh — end-to-end smoke for OTel-driven agent discovery.
#
#  1. Health
#  2. Create UseCase that allows openai/gpt-4o only
#  3. Create a key bound to that UseCase
#  4. POST a synthetic OTLP batch (LLM gpt-4o + Qdrant search) at /v1/traces
#  5. Verify a DiscoveredAgent appears via /api/admin/discovery/agents
#  6. Verify the Qdrant call is flagged
#  7. Promote the agent
#  8. POST 2 more OTLP batches with Qdrant -> 3 flags total -> key revoked
#
# Env: BASE_URL, MASTER_KEY (same defaults as test-mcp-registry.sh)

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
MASTER_KEY="${MASTER_KEY:-sk-master-dev-key-change-in-production}"
H_AUTH="Authorization: Bearer ${MASTER_KEY}"
H_JSON="Content-Type: application/json"

if [ -t 1 ]; then
  GREEN=$'\033[32m'; RED=$'\033[31m'; BOLD=$'\033[1m'; RESET=$'\033[0m'
else GREEN=''; RED=''; BOLD=''; RESET=''; fi

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); printf "%s ✓ %s%s\n" "$GREEN" "$1" "$RESET"; }
fail() { FAIL=$((FAIL+1)); printf "%s ✗ %s%s\n" "$RED" "$1" "$RESET"; }

# 1. Health
curl -s "${BASE_URL}/health" >/dev/null && ok "health" || fail "health"

# 2. Create UseCase allowing only openai/gpt-4o
UC_NAME="otel-demo-$(date +%s)"
UC_PAYLOAD=$(cat <<JSON
{
  "name": "${UC_NAME}",
  "title": "OTel demo",
  "description": "auto-created by test-otel-discovery.sh",
  "enforce_scope": true,
  "refs": [
    {"target_kind":"llm","target_name":"openai/gpt-4o"}
  ]
}
JSON
)
UC_ID=$(curl -s -H "$H_AUTH" -H "$H_JSON" -d "$UC_PAYLOAD" "${BASE_URL}/api/admin/use-cases" | jq -r '.id')
[ -n "$UC_ID" ] && [ "$UC_ID" != "null" ] && ok "use case created ($UC_ID)" || fail "use case create"

# 3. Create key bound to UseCase
KEY_PAYLOAD=$(cat <<JSON
{"name":"otel-demo-key","use_case_id":"${UC_ID}"}
JSON
)
KEY_RESP=$(curl -s -H "$H_AUTH" -H "$H_JSON" -d "$KEY_PAYLOAD" "${BASE_URL}/api/admin/keys")
KEY_ID=$(echo "$KEY_RESP" | jq -r '.id')
[ -n "$KEY_ID" ] && [ "$KEY_ID" != "null" ] && ok "key created ($KEY_ID)" || fail "key create"

# 4. POST synthetic OTLP
SVC_NAME="fraud-investigator-test-$$"
otlp_payload() {
  cat <<JSON
{
  "resourceSpans":[{
    "resource":{"attributes":[
      {"key":"service.name","value":{"stringValue":"${SVC_NAME}"}},
      {"key":"service.version","value":{"stringValue":"0.1.0"}},
      {"key":"pllm.key_id","value":{"stringValue":"${KEY_ID}"}}
    ]},
    "scopeSpans":[{
      "scope":{"name":"test"},
      "spans":[
        {"name":"openai.chat","kind":3,"startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[
          {"key":"gen_ai.system","value":{"stringValue":"openai"}},
          {"key":"gen_ai.request.model","value":{"stringValue":"gpt-4o"}}
        ]},
        {"name":"qdrant.search","kind":3,"startTimeUnixNano":"3","endTimeUnixNano":"4","attributes":[
          {"key":"db.system","value":{"stringValue":"qdrant"}},
          {"key":"db.collection.name","value":{"stringValue":"customers"}}
        ]}
      ]
    }]
  }]
}
JSON
}
curl -s -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces" >/dev/null && ok "otlp batch 1 posted" || fail "otlp batch 1"

# 5. Verify DiscoveredAgent appears
sleep 1
AGENT_LIST=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/discovery/agents?status=pending")
AGENT_ID=$(echo "$AGENT_LIST" | jq -r --arg svc "$SVC_NAME" '.items[] | select(.service_name==$svc) | .id' | head -1)
[ -n "$AGENT_ID" ] && [ "$AGENT_ID" != "null" ] && ok "discovered agent listed ($AGENT_ID)" || fail "discovered agent missing"

# 6. Verify Qdrant call is flagged
DETAIL=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/discovery/agents/${AGENT_ID}")
QDRANT_OUTCOME=$(echo "$DETAIL" | jq -r '.calls[] | select(.kind=="vector_db") | .last_outcome' | head -1)
[ "$QDRANT_OUTCOME" = "flagged" ] && ok "qdrant call flagged" || fail "qdrant call not flagged (got: $QDRANT_OUTCOME)"

# 7. Promote
PROM=$(curl -s -X POST -H "$H_AUTH" -H "$H_JSON" -d '{"version":"0.1.0"}' "${BASE_URL}/api/admin/discovery/agents/${AGENT_ID}/promote")
echo "$PROM" | jq -e '.registry_agent_id' >/dev/null && ok "promoted to registry agent" || fail "promote failed"

# 8. Two more batches → 3 flags total → key revoked
curl -s -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces" >/dev/null
curl -s -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces" >/dev/null
sleep 2

KEY_AFTER=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/keys/${KEY_ID}")
ACTIVE=$(echo "$KEY_AFTER" | jq -r '.is_active')
[ "$ACTIVE" = "false" ] && ok "key auto-revoked after threshold" || fail "key still active (got: $ACTIVE)"

echo
echo "${BOLD}Summary: ${GREEN}${PASS} passed${RESET}, ${RED}${FAIL} failed${RESET}"
exit $FAIL
```

- [ ] **Step 2: Make executable**

```bash
chmod +x scripts/test-otel-discovery.sh
```

- [ ] **Step 3: Run server + smoke test**

In one terminal:
```bash
make docker-up && make run
```

In another:
```bash
bash scripts/test-otel-discovery.sh
```

Expected output: all 8 lines show `✓` and `Summary: N passed, 0 failed`.

If `is_active` check fails: confirm `discovery.flag_threshold` is set to `3` in `config.yaml` and the server picked up that file.

- [ ] **Step 4: Commit**

```bash
git add scripts/test-otel-discovery.sh
git commit -m "feat(scripts): otel discovery end-to-end smoke test"
```

---

## Task 14: Run the live demo end-to-end

**Files:** none — verification step.

- [ ] **Step 1: Bring up infra**

```bash
make docker-up && make run
```

- [ ] **Step 2: Run smoke test from previous task**

```bash
bash scripts/test-otel-discovery.sh
```
Expected: 0 failures.

- [ ] **Step 3: Visually verify Compliance page**

Open `http://localhost:8080/compliance`. The "Discovered agents" card should show the test agent rows that the smoke test produced. Click "Details" on one — the Qdrant row should show `flagged` with reason text.

- [ ] **Step 4: Run the real Python agent end-to-end**

```bash
cd examples/agents/fraud-investigator
docker compose up -d
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
export PLLM_BASE_URL=http://localhost:8080
# Use the master key + a UUID you've created via the admin UI:
export PLLM_KEY_ID=<key uuid bound to a UseCase that allows only gpt-4o>
export PLLM_API_KEY=<that key's value>
export OPENAI_API_KEY=$PLLM_API_KEY
export OTEL_SERVICE_NAME=fraud-investigator
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
python main.py
```

Expected:
- Step 1 prints `ok → ...` (gpt-4o allowed)
- Step 2 prints `blocked at gateway as expected: ...` (gpt-3.5-turbo denied)
- Step 3 prints `qdrant returned ... hits` and notes flag

Then on Compliance page: agent appears, has 1 flagged call. Re-run the script twice more; on the third run the API key is revoked and Step 1 starts failing too.

- [ ] **Step 5: Webhook test**

Add a webhook subscription via `/compliance` UI pointing at https://webhook.site/<your-uuid> for events `scope.flagged`. Re-run the agent. Verify the webhook.site receives the flagged event with the Qdrant target.

- [ ] **Step 6: Final commit (if any docs touched)**

If steps surfaced changes (e.g., README adjustments), commit. Otherwise skip.

---

## Self-review checklist

- **Spec coverage:** Auto-onboard via OTel ✅ (Task 4). Hard-deny LLM/MCP at gateway ✅ (existing — referenced in policy.allow). Flag + webhook OTel violations ✅ (Tasks 5 + 9 bridge to existing EventRecorder/webhooks). Auto-revoke after threshold ✅ (Task 6). Sample agent ✅ (Task 12). End-to-end demo ✅ (Tasks 13 + 14).
- **Placeholders:** scanned — no TBD/TODO/"add appropriate". One ambiguity: Task 9's bridge to `registry.AgentService.PublishAgent` — the exact method name should be confirmed against `internal/services/registry/service/agents.go` during execution; I've left a one-line note to adjust if the signature differs.
- **Type consistency:** `OracleDecision`, `RecordInput`, `PolicyResult`, `PublishInput`, `CallObservation`, `ProcessedSpans`, `AgentFingerprint` are defined once and used consistently across processor.go / policy.go / service.go. `DiscoveredCallKind` (model) and `CallKindStr` (processor) are kept distinct on purpose so the processor stays import-cycle-free; conversion is one-line `models.DiscoveredCallKind(c.Kind)` in service.go.
- **Migrations:** AutoMigrate updated in three places (CLAUDE.md gotcha noted).
- **Frontend types:** All new types added to `web/src/types/api.ts` so the editorial-UI components compile.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-09-otel-agent-discovery.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Best fit for the demo deadline because failures localize and don't pollute the main session context.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints. Simpler if you want to watch each step land.

Which approach?
