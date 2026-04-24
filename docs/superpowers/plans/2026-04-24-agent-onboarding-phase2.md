# Agent Onboarding — Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose deployed agents for external invocation through pllm with scope-checked, audited traffic. External callers see A2A; the agent pod sees a simple `POST /invoke` JSON envelope.

**Architecture:** External surface speaks A2A JSON-RPC 2.0 (`POST /v1/a2a/{slug}` + `GET /v1/a2a/{slug}/.well-known/agent.json`) plus a plain alias (`POST /v1/agents/{slug}/invoke`). Internal surface is a pllm → agent-pod HTTP proxy: pllm resolves the slug to the `AgentDeployment.Endpoint` already populated in Phase 1, translates the incoming envelope to `{"input": ..., "task_id": ...}`, POSTs to `{endpoint}/invoke`, and translates the `{"output": ...}` response back. Caller's key is scope-checked against the bound UseCase's declared agent refs; every invocation emits a `ScopeEvent` (allow or deny).

**Tech Stack:** Go 1.25+, chi/v5 router, `net/http` client, existing `ScopeResolver` + `EventRecorder` extended with agent-shaped methods. No new DB tables. No new external dependencies.

---

## Scope

**In scope (Phase 2):**
- `ScopeResolver.AllowAgent` + `agents` cache set (reuses existing `RegistryRef` edges where `OwnerKind=use_case, TargetKind=agent`)
- `ScopeEventKindAgent` enum
- `internal/services/a2a/` package: manager (slug → endpoint lookup), proxy (HTTP call to pod), protocol types
- `internal/api/handlers/a2a/` handler package with three endpoints
- Narrow-interface bridges in `router.go` (`a2aScopeAdapter`, `RecordAgentDenial` on the existing event adapter)
- Plain alias `POST /v1/agents/{slug}/invoke` for dev UX
- A2A agent card derived from `RegistryAgent` + `AgentDeployment` + skill refs
- UI: invoke tester on the Agents page (small modal — POST to plain alias, show output)
- Smoke-test additions to `scripts/test-agent-onboarding.sh`

**Out of scope (later phases):**
- `tasks/sendSubscribe` (SSE streaming) — Phase 2.5
- `tasks/get` / `tasks/cancel` / multi-turn task state store — Phase 2.5
- Push notifications — not planned
- OpenAI-assistants-compatible surface — when Phase 3 ships the SDK
- Session pinning for stateful agents — "known gaps" item, separate
- Agent → agent (A2A client) calls initiated by an agent talking to another — Phase 4 or deferred

**Contract locked in this phase (operator-facing):**

The agent pod MUST expose `POST /invoke` on its container port, accepting:
```json
{"input": <any>, "task_id": "<string>"}
```
and returning:
```json
{"output": <any>, "error": "<string, optional>"}
```
Phase 3's SDK will auto-provide this endpoint. Phase 2 hand-written demo agents implement it directly (10-15 lines of Flask/FastAPI).

---

## File Structure

| File | Create / Modify | Responsibility |
|---|---|---|
| `internal/core/models/scope_event.go` | modify | Add `ScopeEventKindAgent` enum value |
| `internal/services/use_cases/scope.go` | modify | Add `AllowAgent` method + `agents` cache set + fetch logic |
| `internal/services/use_cases/scope_test.go` | modify (or create) | Cover `AllowAgent` allow/deny paths |
| `internal/services/a2a/protocol.go` | create | JSON-RPC 2.0 envelope, Task, Message, AgentCard types |
| `internal/services/a2a/manager.go` | create | `slug → AgentDeployment` lookup; shared across handlers |
| `internal/services/a2a/proxy.go` | create | HTTP client to pod's `POST /invoke`, translates envelopes |
| `internal/services/a2a/proxy_test.go` | create | Tests using `httptest.Server` to fake the pod |
| `internal/api/handlers/a2a/gateway_handler.go` | create | `POST /v1/a2a/{slug}` JSON-RPC, `POST /v1/agents/{slug}/invoke` alias |
| `internal/api/handlers/a2a/card_handler.go` | create | `GET /v1/a2a/{slug}/.well-known/agent.json` |
| `internal/api/handlers/a2a/gateway_handler_test.go` | create | HTTP-level tests with `httptest.Server` pod + test DB |
| `internal/api/handlers/a2a/interfaces.go` | create | `ScopeChecker` + `ScopeEventRecorder` narrow interfaces (consumer-side) |
| `internal/api/router/router.go` | modify | `a2aScopeAdapter` + `RecordAgentDenial` on `scopeEventAdapter` + route mounting |
| `web/src/components/agents/invoke-dialog.tsx` | create | Small "Invoke" dialog — textarea → response viewer |
| `web/src/pages/Agents.tsx` | modify | Add Invoke dropdown action + wire dialog |
| `web/src/lib/api.ts` | modify | `invokeAgent(slug, input)` method |
| `scripts/test-agent-onboarding.sh` | modify | Extend to exercise `/v1/agents/{slug}/invoke` against a tiny reference pod |

**Design principles this structure enforces:**
- `internal/services/a2a/` owns protocol + routing + outbound HTTP. The handler package owns HTTP ingress + scope wrap only. Same separation as `internal/services/mcp/gateway/` vs `internal/api/handlers/mcp/`.
- Narrow interfaces live in `interfaces.go` — the handler package never imports `use_cases`. Follows the CLAUDE.md "Key pattern" exactly.
- `protocol.go` is *data only* (struct types + JSON tags + builders). No side effects. Easy to unit-test, easy to reuse later for streaming.
- No new DB tables. Routing uses `AgentDeployment.Endpoint` already populated in Phase 1 by `AgentK8sAdapter.Deploy`.

---

## Task 1: Add ScopeEventKindAgent + scope fetch support

**Files:**
- Modify: `internal/core/models/scope_event.go`
- Modify: `internal/services/use_cases/scope.go`

- [ ] **Step 1: Add the enum**

Modify `internal/core/models/scope_event.go`. Find the block:

```go
const (
	ScopeEventKindMCPTool ScopeEventKind = "mcp_tool"
	ScopeEventKindLLM     ScopeEventKind = "llm"
)
```

Add the new value:

```go
const (
	ScopeEventKindMCPTool ScopeEventKind = "mcp_tool"
	ScopeEventKindLLM     ScopeEventKind = "llm"
	ScopeEventKindAgent   ScopeEventKind = "agent"
)
```

- [ ] **Step 2: Extend `cachedScope` with an agents set**

Modify `internal/services/use_cases/scope.go`. Find `type cachedScope struct` and add the field:

```go
type cachedScope struct {
	llms       map[string]struct{}
	serverSlug map[string]struct{}
	agents     map[string]struct{}
	enforce    bool
	expiresAt  time.Time
}
```

In `fetch()` find the `out := cachedScope{...}` literal and add `agents: map[string]struct{}{},`:

```go
	out := cachedScope{
		llms:       map[string]struct{}{},
		serverSlug: map[string]struct{}{},
		agents:     map[string]struct{}{},
		enforce:    uc.EnforceScope,
		expiresAt:  time.Now().Add(r.ttl),
	}
```

In the `for _, ref := range refs` loop add the agent case alongside `RegistryKindLLM` and `RegistryKindServer`:

```go
		case models.RegistryKindAgent:
			// Same matching rules as servers: accept full registry name,
			// last path segment, and any declared local_name alias.
			out.agents[ref.TargetName] = struct{}{}
			if i := strings.LastIndex(ref.TargetName, "/"); i >= 0 {
				out.agents[ref.TargetName[i+1:]] = struct{}{}
			}
			if ref.LocalName != "" {
				out.agents[ref.LocalName] = struct{}{}
			}
```

- [ ] **Step 3: Write the failing test for `AllowAgent`**

If `internal/services/use_cases/scope_test.go` exists, append. Otherwise create it:

```go
package use_cases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestAllowAgentMatchesDeclaredRef(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ucID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel:     models.BaseModel{ID: ucID},
		Name:          "uc", Slug: "uc",
		EnforceScope:  true,
	}).Error)
	require.NoError(t, db.Create(&models.RegistryRef{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		OwnerKind:  models.RegistryKindUseCase,
		OwnerID:    ucID,
		TargetKind: models.RegistryKindAgent,
		TargetName: "pllm.io/sales-research",
	}).Error)

	r := NewScopeResolver(db)

	// Full name allowed.
	d := r.AllowAgent(context.Background(), &ucID, "pllm.io/sales-research")
	require.True(t, d.Allowed)
	require.True(t, d.EnforceMode)

	// Last-segment allowed.
	d = r.AllowAgent(context.Background(), &ucID, "sales-research")
	require.True(t, d.Allowed)

	// Unknown agent denied.
	d = r.AllowAgent(context.Background(), &ucID, "some-other-agent")
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "is not declared")
}

func TestAllowAgentUnscopedKeyAllows(t *testing.T) {
	r := NewScopeResolver(nil)
	d := r.AllowAgent(context.Background(), nil, "anything")
	require.True(t, d.Allowed)
}
```

Run: `go test ./internal/services/use_cases/ -run TestAllowAgent -v`
Expected: FAIL — `AllowAgent` undefined.

- [ ] **Step 4: Implement `AllowAgent`**

Append to `internal/services/use_cases/scope.go` (after `AllowMCPTool`):

```go
// AllowAgent checks whether the key's use case declares `agentName` as
// an agent ref. Accepts the full registry name, the last path segment,
// or any declared local_name alias (mirrors AllowMCPTool's matching).
// Unscoped keys (no UseCaseID) or missing use case rows always allow.
func (r *ScopeResolver) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) Decision {
	if useCaseID == nil {
		return Decision{Allowed: true}
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		return Decision{Allowed: true, Reason: "use case lookup failed: " + err.Error()}
	}
	if _, ok := s.agents[agentName]; ok {
		return Decision{Allowed: true, EnforceMode: s.enforce}
	}
	return Decision{
		Allowed:     false,
		EnforceMode: s.enforce,
		Reason:      "agent " + agentName + " is not declared by the bound use case",
	}
}
```

Run: `go test ./internal/services/use_cases/ -run TestAllowAgent -v`
Expected: PASS (both cases).

- [ ] **Step 5: Commit**

```bash
git add internal/core/models/scope_event.go \
        internal/services/use_cases/scope.go \
        internal/services/use_cases/scope_test.go
git commit -m "$(cat <<'EOF'
feat(use_cases): scope check + event kind for agent invocations

AllowAgent mirrors AllowMCPTool: same TTL cache, same matching rules
(full name / last path segment / local_name alias). ScopeEventKindAgent
joins ScopeEventKindMCPTool and ScopeEventKindLLM so the denial audit
log carries the agent-invoke surface too.
EOF
)"
```

---

## Task 2: A2A protocol types + agent card builder

**Files:**
- Create: `internal/services/a2a/protocol.go`
- Create: `internal/services/a2a/protocol_test.go`

- [ ] **Step 1: Write the types**

Create `internal/services/a2a/protocol.go`:

```go
// Package a2a implements enough of the Agent-to-Agent JSON-RPC protocol
// for Phase 2: tasks/send (non-streaming) + an agent card.
//
// Spec reference: github.com/google/A2A (tracking v0.x — pre-1.0).
// We deliberately don't chase every optional field; only what the
// bank demo and the plain-JSON alias need.
package a2a

import "encoding/json"

// --- JSON-RPC 2.0 envelope ------------------------------------------------

// JSONRPCRequest is the top-level request envelope.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"` // always "2.0"
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"` // string, number, or null
}

// JSONRPCResponse is the top-level response envelope.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// JSONRPCError codes we use.
const (
	ErrCodeParse           = -32700
	ErrCodeInvalidRequest  = -32600
	ErrCodeMethodNotFound  = -32601
	ErrCodeInvalidParams   = -32602
	ErrCodeInternal        = -32603
	// App-level codes (outside the reserved JSON-RPC range).
	ErrCodeScopeDenied     = -32001
	ErrCodeAgentNotFound   = -32002
	ErrCodeAgentUnhealthy  = -32003
)

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// --- A2A task / message types --------------------------------------------

// TaskSendParams is the payload of "tasks/send".
type TaskSendParams struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
}

// Message is an A2A user/agent message. Parts can be mixed (text, data,
// file) but for Phase 2 we only pass-through; we don't interpret.
type Message struct {
	Role  string          `json:"role"` // "user" | "agent"
	Parts []MessagePart   `json:"parts"`
}

type MessagePart struct {
	Type string `json:"type"` // "text" | "data" | "file"
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

// Task is the canonical response shape.
type Task struct {
	ID       string      `json:"id"`
	Status   TaskStatus  `json:"status"`
	Messages []Message   `json:"messages,omitempty"`
}

type TaskStatus struct {
	State   string `json:"state"` // "submitted" | "working" | "completed" | "failed"
	Message string `json:"message,omitempty"`
}

// --- Agent card (well-known) ---------------------------------------------

// AgentCard is served at /v1/a2a/{slug}/.well-known/agent.json.
type AgentCard struct {
	Name         string             `json:"name"`
	Description  string             `json:"description,omitempty"`
	URL          string             `json:"url"` // where to POST A2A calls
	Version      string             `json:"version"`
	Capabilities AgentCapabilities  `json:"capabilities"`
	Skills       []AgentCardSkill   `json:"skills,omitempty"`
}

// AgentCapabilities describes what this agent supports. Phase 2 is
// request-response only; streaming + push lands in Phase 2.5.
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

// AgentCardSkill is the trimmed-down view of a registry skill we surface
// as "what this agent knows how to do". Only name + description are
// mandatory per the A2A spec; we fill both from the RegistryRef target.
type AgentCardSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// --- Pod contract (internal) ---------------------------------------------
//
// The agent pod exposes POST {endpoint}/invoke with the following shape:
//
//   request:  {"input": <any>, "task_id": "<string>"}
//   response: {"output": <any>, "error": "<optional string>"}
//
// These types are used by the proxy to build request bodies and parse
// responses. Not serialized on the external surface.

type InvokeRequest struct {
	Input  any    `json:"input"`
	TaskID string `json:"task_id"`
}

type InvokeResponse struct {
	Output any    `json:"output"`
	Error  string `json:"error,omitempty"`
}
```

- [ ] **Step 2: Write round-trip tests**

Create `internal/services/a2a/protocol_test.go`:

```go
package a2a

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONRPCRequestRoundTrip(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","method":"tasks/send","params":{"id":"t-1","message":{"role":"user","parts":[{"type":"text","text":"hello"}]}},"id":1}`)
	var req JSONRPCRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Equal(t, "2.0", req.JSONRPC)
	require.Equal(t, "tasks/send", req.Method)

	var p TaskSendParams
	require.NoError(t, json.Unmarshal(req.Params, &p))
	require.Equal(t, "t-1", p.ID)
	require.Equal(t, "user", p.Message.Role)
	require.Equal(t, "hello", p.Message.Parts[0].Text)
}

func TestAgentCardShape(t *testing.T) {
	card := AgentCard{
		Name:    "sales-research",
		URL:     "https://pllm.example.com/v1/a2a/sales-research-0-1-0",
		Version: "0.1.0",
		Capabilities: AgentCapabilities{Streaming: false, PushNotifications: false},
		Skills: []AgentCardSkill{{Name: "summarize"}},
	}
	b, err := json.Marshal(card)
	require.NoError(t, err)
	require.Contains(t, string(b), `"streaming":false`)
	require.Contains(t, string(b), `"skills":[{"name":"summarize"}]`)
}
```

Run: `go test ./internal/services/a2a/ -run 'TestJSONRPCRequestRoundTrip|TestAgentCardShape' -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/services/a2a/protocol.go internal/services/a2a/protocol_test.go
git commit -m "$(cat <<'EOF'
feat(a2a): protocol types — JSON-RPC envelope, task/message, agent card

Data-only file; no side effects. The pod contract (InvokeRequest /
InvokeResponse) is colocated so the proxy and the base image SDK land
on a shared schema.
EOF
)"
```

---

## Task 3: a2a.Manager — slug → agent deployment lookup

**Files:**
- Create: `internal/services/a2a/manager.go`
- Create: `internal/services/a2a/manager_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/services/a2a/manager_test.go`:

```go
package a2a

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestManagerResolveBySlugFindsAgentDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ucID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: ucID},
		Name:      "uc", Slug: "uc",
	}).Error)

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		RegistryAgentID: uuid.New(),
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       ucID,
		Platform:        models.DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		Status:          models.DeploymentStatusRunning,
		Endpoint:        "http://sales-research-0-1-0.pllm-agents.svc.cluster.local:8000",
	}).Error)

	m := NewManager(db)

	// Resolve by workload name.
	got, err := m.Resolve(context.Background(), "sales-research-0-1-0")
	require.NoError(t, err)
	require.Equal(t, "pllm.io/sales-research", got.ResourceName)

	// Unknown slug.
	_, err = m.Resolve(context.Background(), "nope")
	require.ErrorIs(t, err, ErrAgentNotFound)
}
```

Run: `go test ./internal/services/a2a/ -run TestManagerResolveBySlug -v`
Expected: FAIL — `NewManager` undefined.

- [ ] **Step 2: Implement**

Create `internal/services/a2a/manager.go`:

```go
package a2a

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ErrAgentNotFound is returned when no running AgentDeployment matches
// the requested slug.
var ErrAgentNotFound = errors.New("agent not found")

// Manager resolves an A2A slug to an AgentDeployment row. The slug is
// the AgentDeployment.WorkloadName (already unique + DNS-safe). Keeping
// this trivial for Phase 2; Phase 2.5 can add friendlier aliases or
// domain-based routing.
type Manager struct {
	db *gorm.DB
}

func NewManager(db *gorm.DB) *Manager {
	return &Manager{db: db}
}

// Resolve looks up the agent by slug. Returns ErrAgentNotFound if no row.
func (m *Manager) Resolve(ctx context.Context, slug string) (*models.AgentDeployment, error) {
	var row models.AgentDeployment
	err := m.db.WithContext(ctx).
		Where("workload_name = ?", slug).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("a2a manager: lookup: %w", err)
	}
	return &row, nil
}
```

Run: `go test ./internal/services/a2a/ -run TestManagerResolveBySlug -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/services/a2a/manager.go internal/services/a2a/manager_test.go
git commit -m "$(cat <<'EOF'
feat(a2a): manager resolves slug → AgentDeployment

Slug is AgentDeployment.WorkloadName. One-shot DB lookup, no cache —
the per-request cost is a single indexed SELECT. Cache can land later
if needed.
EOF
)"
```

---

## Task 4: a2a.Proxy — HTTP call to pod /invoke

**Files:**
- Create: `internal/services/a2a/proxy.go`
- Create: `internal/services/a2a/proxy_test.go`

- [ ] **Step 1: Write the failing test using an httptest fake pod**

Create `internal/services/a2a/proxy_test.go`:

```go
package a2a

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProxyInvokeHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/invoke", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		body, _ := io.ReadAll(r.Body)
		var in InvokeRequest
		require.NoError(t, json.Unmarshal(body, &in))
		require.Equal(t, "t-42", in.TaskID)
		_ = json.NewEncoder(w).Encode(InvokeResponse{Output: "ECHO: " + in.Input.(string)})
	}))
	defer srv.Close()

	p := NewProxy(2 * time.Second)
	res, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{
		Input:  "hello",
		TaskID: "t-42",
	})
	require.NoError(t, err)
	require.Equal(t, "ECHO: hello", res.Output)
	require.Empty(t, res.Error)
}

func TestProxyInvokeErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewProxy(2 * time.Second)
	_, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{Input: "x", TaskID: "t-1"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "500"))
}

func TestProxyInvokePropagatesContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(InvokeResponse{Output: "slow"})
	}))
	defer srv.Close()

	p := NewProxy(50 * time.Millisecond)
	_, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{Input: "x", TaskID: "t-1"})
	require.Error(t, err)
}
```

Run: `go test ./internal/services/a2a/ -run TestProxyInvoke -v`
Expected: FAIL — `NewProxy` undefined.

- [ ] **Step 2: Implement**

Create `internal/services/a2a/proxy.go`:

```go
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Proxy is the outbound HTTP client that calls an agent pod's /invoke
// endpoint. Stateless — one Proxy per pllm process is fine.
type Proxy struct {
	client  *http.Client
	timeout time.Duration
}

// NewProxy constructs a Proxy with the given per-call timeout.
// timeout == 0 falls back to 5 minutes.
func NewProxy(timeout time.Duration) *Proxy {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Proxy{
		client: &http.Client{
			// Overall per-call deadline is enforced via context below;
			// this is a safety net for the transport layer.
			Timeout: timeout + 15*time.Second,
		},
		timeout: timeout,
	}
}

// Invoke POSTs to {endpoint}/invoke with the given request and returns
// the parsed response.
func (p *Proxy) Invoke(ctx context.Context, endpoint string, req InvokeRequest) (*InvokeResponse, error) {
	url := strings.TrimRight(endpoint, "/") + "/invoke"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("a2a proxy: pod returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out InvokeResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("a2a proxy: decode response: %w", err)
	}
	return &out, nil
}
```

Run: `go test ./internal/services/a2a/ -run TestProxyInvoke -v`
Expected: all three PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/services/a2a/proxy.go internal/services/a2a/proxy_test.go
git commit -m "$(cat <<'EOF'
feat(a2a): proxy — HTTP client to pod /invoke

Stateless POST with configurable per-call timeout (default 5m). Context
cancellation propagates; transport-level safety net of timeout+15s on
the underlying http.Client.
EOF
)"
```

---

## Task 5: Gateway handler — `/v1/a2a/{slug}` + `/v1/agents/{slug}/invoke`

**Files:**
- Create: `internal/api/handlers/a2a/interfaces.go`
- Create: `internal/api/handlers/a2a/gateway_handler.go`
- Create: `internal/api/handlers/a2a/gateway_handler_test.go`

- [ ] **Step 1: Define the narrow consumer interfaces**

Create `internal/api/handlers/a2a/interfaces.go`:

```go
// Package a2a holds the HTTP handlers for the A2A gateway surface.
//
// Per repo convention (CLAUDE.md — "Key pattern: narrow interfaces at
// layer boundaries"), this file declares the dependencies this package
// expects — ScopeChecker and ScopeEventRecorder — so the package doesn't
// import `use_cases` directly. The concrete adapters live in the main
// router.go and bridge these interfaces to use_cases.ScopeResolver +
// use_cases.EventRecorder.
package a2a

import (
	"context"

	"github.com/google/uuid"
)

// ScopeDecision is the handler-facing mirror of use_cases.Decision.
type ScopeDecision struct {
	Allowed     bool
	EnforceMode bool
	Reason      string
}

// ScopeChecker decides whether a caller's key may invoke a named agent.
type ScopeChecker interface {
	AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) ScopeDecision
}

// ScopeEventRecorder persists a scope-enforcement event to the audit log.
type ScopeEventRecorder interface {
	RecordAgentDenial(
		ctx context.Context,
		useCaseID uuid.UUID,
		keyID *uuid.UUID, keyName, target, reason string, enforced bool,
		remoteAddr, requestID string,
	)
}
```

- [ ] **Step 2: Write the handler test first (failing)**

Create `internal/api/handlers/a2a/gateway_handler_test.go`:

```go
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

// fakePod boots an httptest server that behaves like a Phase-1 agent pod.
func fakePod(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/invoke", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var in a2asvc.InvokeRequest
		_ = json.Unmarshal(body, &in)
		_ = json.NewEncoder(w).Encode(a2asvc.InvokeResponse{Output: "ECHO:" + in.Input.(string)})
	}))
}

// stubScope allows everything — exercises the happy path only.
type stubScope struct{}

func (stubScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) ScopeDecision {
	return ScopeDecision{Allowed: true}
}

// stubRec drops everything — happy path doesn't call it.
type stubRec struct{ denials int }

func (r *stubRec) RecordAgentDenial(
	_ context.Context, _ uuid.UUID, _ *uuid.UUID,
	_, _, _ string, _ bool, _, _ string,
) {
	r.denials++
}

func TestInvokeAliasHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	pod := fakePod(t)
	defer pod.Close()

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: uuid.New(),
		Platform:  models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: pod.URL,
	}).Error)

	h := NewGatewayHandler(
		nil,
		a2asvc.NewManager(db),
		a2asvc.NewProxy(2*time.Second),
		stubScope{}, &stubRec{},
	)

	// Mount on a chi router so {slug} URL param resolves.
	r := chi.NewRouter()
	r.Post("/v1/agents/{slug}/invoke", h.InvokePlain)

	body, _ := json.Marshal(map[string]any{"input": "hello", "task_id": "t-1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/a-0-1-0/invoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out a2asvc.InvokeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "ECHO:hello", out.Output)
}

// denyScope always denies with enforce=true.
type denyScope struct{}

func (denyScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) ScopeDecision {
	return ScopeDecision{Allowed: false, EnforceMode: true, Reason: "not in use case"}
}

func TestInvokeAliasBlockedByScope(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ucID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: ucID},
		Name: "uc", Slug: "uc",
	}).Error)
	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: ucID, Platform: models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: "http://never-called.invalid",
	}).Error)

	rec := &stubRec{}
	h := NewGatewayHandler(nil, a2asvc.NewManager(db), a2asvc.NewProxy(time.Second), denyScope{}, rec)

	r := chi.NewRouter()
	r.Post("/v1/agents/{slug}/invoke", h.InvokePlain)

	body, _ := json.Marshal(map[string]any{"input": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/a-0-1-0/invoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, 1, rec.denials)
}

func TestA2AJSONRPCTasksSend(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	pod := fakePod(t)
	defer pod.Close()

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: uuid.New(),
		Platform:  models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: pod.URL,
	}).Error)

	h := NewGatewayHandler(nil, a2asvc.NewManager(db), a2asvc.NewProxy(time.Second), stubScope{}, &stubRec{})
	r := chi.NewRouter()
	r.Post("/v1/a2a/{slug}", h.JSONRPC)

	env := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"id": "t-xyz",
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{{"type": "text", "text": "hi"}},
			},
		},
		"id": 1,
	}
	body, _ := json.Marshal(env)
	req := httptest.NewRequest(http.MethodPost, "/v1/a2a/a-0-1-0", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp a2asvc.JSONRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.Error)
	// Result is a Task with state=completed and one agent message carrying the echo.
	b, _ := json.Marshal(resp.Result)
	var task a2asvc.Task
	require.NoError(t, json.Unmarshal(b, &task))
	require.Equal(t, "t-xyz", task.ID)
	require.Equal(t, "completed", task.Status.State)
	require.Len(t, task.Messages, 1)
	require.Equal(t, "agent", task.Messages[0].Role)
}
```

Run: `go test ./internal/api/handlers/a2a/ -v`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/api/handlers/a2a/gateway_handler.go`:

```go
package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

// GatewayHandler serves /v1/a2a/{slug} (JSON-RPC) and /v1/agents/{slug}/invoke
// (plain JSON alias).
type GatewayHandler struct {
	logger  *zap.Logger
	manager *a2asvc.Manager
	proxy   *a2asvc.Proxy
	scope   ScopeChecker
	events  ScopeEventRecorder
}

func NewGatewayHandler(
	logger *zap.Logger,
	manager *a2asvc.Manager,
	proxy *a2asvc.Proxy,
	scope ScopeChecker,
	events ScopeEventRecorder,
) *GatewayHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GatewayHandler{
		logger: logger, manager: manager, proxy: proxy,
		scope: scope, events: events,
	}
}

// plainInvokeRequest is the body of /v1/agents/{slug}/invoke.
type plainInvokeRequest struct {
	Input  any    `json:"input"`
	TaskID string `json:"task_id,omitempty"`
}

// InvokePlain handles POST /v1/agents/{slug}/invoke.
func (h *GatewayHandler) InvokePlain(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req plainInvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TaskID == "" {
		req.TaskID = uuid.NewString()
	}

	agent, resp, denied := h.authorizeAndResolve(r, slug)
	if denied {
		return // response already written
	}
	if resp != nil {
		writeJSONError(w, resp.status, resp.msg)
		return
	}

	out, err := h.proxy.Invoke(r.Context(), agent.Endpoint, a2asvc.InvokeRequest{
		Input: req.Input, TaskID: req.TaskID,
	})
	if err != nil {
		h.logger.Warn("a2a invoke: pod call failed",
			zap.String("slug", slug), zap.Error(err))
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// JSONRPC handles POST /v1/a2a/{slug}. Accepts JSON-RPC 2.0 requests;
// currently only method "tasks/send" is supported. Other methods return
// JSON-RPC MethodNotFound.
func (h *GatewayHandler) JSONRPC(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req a2asvc.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCErr(w, nil, a2asvc.ErrCodeParse, err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}
	if req.Method != "tasks/send" {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeMethodNotFound, "unsupported method: "+req.Method)
		return
	}

	var params a2asvc.TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeInvalidParams, err.Error())
		return
	}
	if params.ID == "" {
		params.ID = uuid.NewString()
	}

	agent, errResp, denied := h.authorizeAndResolve(r, slug)
	if denied {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeScopeDenied, "invocation denied by use case scope")
		return
	}
	if errResp != nil {
		code := a2asvc.ErrCodeInternal
		if errResp.status == http.StatusNotFound {
			code = a2asvc.ErrCodeAgentNotFound
		}
		writeJSONRPCErr(w, req.ID, code, errResp.msg)
		return
	}

	// Flatten incoming message parts into a single "input" for the pod.
	// A2A-aware pods can ask for structured input in Phase 2.5.
	flat := flattenMessageText(params.Message)

	out, err := h.proxy.Invoke(r.Context(), agent.Endpoint, a2asvc.InvokeRequest{
		Input: flat, TaskID: params.ID,
	})
	if err != nil {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeAgentUnhealthy, err.Error())
		return
	}

	task := a2asvc.Task{
		ID:     params.ID,
		Status: a2asvc.TaskStatus{State: "completed"},
		Messages: []a2asvc.Message{{
			Role: "agent",
			Parts: []a2asvc.MessagePart{{
				Type: "text",
				Text: fmt.Sprintf("%v", out.Output),
			}},
		}},
	}
	writeJSON(w, http.StatusOK, a2asvc.JSONRPCResponse{
		JSONRPC: "2.0", Result: task, ID: req.ID,
	})
}

// --- internals ------------------------------------------------------------

type httpErr struct {
	status int
	msg    string
}

// authorizeAndResolve does the scope check + agent lookup. Returns one of:
//   - (agent, nil, false)        — allowed, agent resolved
//   - (nil, httpErr, false)      — non-scope error (404, 502, etc.)
//   - (nil, nil, true)           — denied; response already written by plain path
//                                  (JSONRPC path writes its own error envelope)
// For plain path, writes 403 before returning denied=true.
// For JSONRPC path, leaves response unwritten so caller can wrap in JSON-RPC error.
func (h *GatewayHandler) authorizeAndResolve(r *http.Request, slug string) (*models.AgentDeployment, *httpErr, bool) {
	agent, err := h.manager.Resolve(r.Context(), slug)
	if err != nil {
		if errors.Is(err, a2asvc.ErrAgentNotFound) {
			return nil, &httpErr{http.StatusNotFound, "agent not found"}, false
		}
		return nil, &httpErr{http.StatusInternalServerError, err.Error()}, false
	}

	// Scope check: caller's key UseCaseID must declare this agent's
	// resource_name (full or last-segment).
	var callerUC *uuid.UUID
	var keyID *uuid.UUID
	var keyName string
	if k, ok := middleware.GetKey(r.Context()); ok && k != nil {
		callerUC = k.UseCaseID
		kid := k.ID
		keyID = &kid
		keyName = k.Name
	}

	dec := h.scope.AllowAgent(r.Context(), callerUC, agent.ResourceName)
	if !dec.Allowed && dec.EnforceMode {
		if h.events != nil && callerUC != nil {
			h.events.RecordAgentDenial(
				r.Context(), *callerUC, keyID, keyName,
				agent.ResourceName, dec.Reason, true,
				r.RemoteAddr, requestID(r),
			)
		}
		// For plain path we write a 403 here so the caller sees a clear
		// HTTP error. JSONRPC path writes its own; to distinguish we
		// leave response unwritten and signal via `denied=true`. We
		// handle this by checking which surface is calling.
		if isPlainPath(r) {
			writeJSONError(responseWriter(r), http.StatusForbidden, "scope: "+dec.Reason)
		}
		return nil, nil, true
	}
	return agent, nil, false
}

// isPlainPath returns true if the request URL is the plain /invoke alias
// (so the shared authorize helper knows whether to pre-write the response).
func isPlainPath(r *http.Request) bool {
	// Cheap check — the shape of the plain endpoint is /v1/agents/{slug}/invoke.
	return len(r.URL.Path) > len("/v1/agents/") && r.URL.Path[len(r.URL.Path)-len("/invoke"):] == "/invoke"
}

// responseWriter pulls the http.ResponseWriter out of the request via a
// custom context key the handlers set. The plain-invoke and json-rpc
// entrypoints call authorizeAndResolve and both know their w; we stash
// it so the helper can respond on the plain path. See the wrapper pattern
// below in InvokePlain.
type rwKey struct{}

func responseWriter(r *http.Request) http.ResponseWriter {
	v := r.Context().Value(rwKey{})
	if w, ok := v.(http.ResponseWriter); ok {
		return w
	}
	return nil
}

func withWriter(w http.ResponseWriter, r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), rwKey{}, w))
}

// requestID pulls the chi-provided correlation id for audit.
func requestID(r *http.Request) string {
	return chiMiddleware.GetReqID(r.Context())
}

// flattenMessageText concatenates text parts; drops data/file parts for
// Phase 2. Phase 2.5 passes structured input through.
func flattenMessageText(m a2asvc.Message) string {
	out := ""
	for _, p := range m.Parts {
		if p.Type == "text" {
			if out != "" {
				out += "\n"
			}
			out += p.Text
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSONRPCErr(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	writeJSON(w, http.StatusOK, a2asvc.JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &a2asvc.JSONRPCError{Code: code, Message: msg},
		ID:      id,
	})
}
```

Now the `InvokePlain` and `JSONRPC` handlers need to stash `w` into context before calling `authorizeAndResolve`. Replace the single-line entry in `InvokePlain` from:
```go
agent, resp, denied := h.authorizeAndResolve(r, slug)
```
with:
```go
r = withWriter(w, r)
agent, resp, denied := h.authorizeAndResolve(r, slug)
```

Do the same substitution at the top of `JSONRPC`, just above the `if errResp != nil` block:
```go
r = withWriter(w, r)
agent, errResp, denied := h.authorizeAndResolve(r, slug)
```


Run: `go test ./internal/api/handlers/a2a/ -v`
Expected: all three handler tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/a2a/interfaces.go \
        internal/api/handlers/a2a/gateway_handler.go \
        internal/api/handlers/a2a/gateway_handler_test.go
git commit -m "$(cat <<'EOF'
feat(a2a): gateway handler — JSON-RPC tasks/send + plain invoke alias

Scope-check + audit wrapped around an HTTP proxy to the pod's /invoke.
JSON-RPC envelope flattens text message parts into the pod's plain
input; Phase 2.5 will pass structured parts through.
EOF
)"
```

---

## Task 6: Card handler — `GET /v1/a2a/{slug}/.well-known/agent.json`

**Files:**
- Create: `internal/api/handlers/a2a/card_handler.go`
- Create: `internal/api/handlers/a2a/card_handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/handlers/a2a/card_handler_test.go`:

```go
package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

func TestAgentCardDerivedFromRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel:   models.BaseModel{ID: agentID},
		Name:        "pllm.io/sales-research",
		Version:     "0.1.0",
		Title:       "Sales Research",
		Description: "Summarize customer docs",
		IsLatest:    true,
	}).Error)
	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		RegistryAgentID: agentID,
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       uuid.New(),
		Platform:        models.DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		Status:          models.DeploymentStatusRunning,
	}).Error)
	// Attach one skill ref.
	require.NoError(t, db.Create(&models.RegistryRef{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		OwnerKind:  models.RegistryKindAgent,
		OwnerID:    agentID,
		TargetKind: models.RegistryKindSkill,
		TargetName: "summarize",
	}).Error)

	h := NewCardHandler(nil, a2asvc.NewManager(db), db, "https://pllm.example.com")
	r := chi.NewRouter()
	r.Get("/v1/a2a/{slug}/.well-known/agent.json", h.Card)

	req := httptest.NewRequest(http.MethodGet, "/v1/a2a/sales-research-0-1-0/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var card a2asvc.AgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &card))
	require.Equal(t, "pllm.io/sales-research", card.Name)
	require.Equal(t, "0.1.0", card.Version)
	require.Equal(t, "https://pllm.example.com/v1/a2a/sales-research-0-1-0", card.URL)
	require.Len(t, card.Skills, 1)
	require.Equal(t, "summarize", card.Skills[0].Name)
	require.False(t, card.Capabilities.Streaming)
}
```

Run: `go test ./internal/api/handlers/a2a/ -run TestAgentCard -v`
Expected: FAIL — `NewCardHandler` undefined.

- [ ] **Step 2: Implement**

Create `internal/api/handlers/a2a/card_handler.go`:

```go
package a2a

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

// CardHandler serves /v1/a2a/{slug}/.well-known/agent.json.
type CardHandler struct {
	logger   *zap.Logger
	manager  *a2asvc.Manager
	db       *gorm.DB
	publicURL string // base URL clients see pllm at; e.g. "https://pllm.example.com"
}

func NewCardHandler(logger *zap.Logger, manager *a2asvc.Manager, db *gorm.DB, publicURL string) *CardHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CardHandler{logger: logger, manager: manager, db: db, publicURL: publicURL}
}

// Card returns the A2A agent card derived from the RegistryAgent (for
// display fields) and RegistryRef rows (for skills).
func (h *CardHandler) Card(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	agent, err := h.manager.Resolve(r.Context(), slug)
	if err != nil {
		if errors.Is(err, a2asvc.ErrAgentNotFound) {
			writeJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Pull registry row for display fields (description). Tolerate missing.
	var reg models.RegistryAgent
	if err := h.db.WithContext(r.Context()).
		First(&reg, "id = ?", agent.RegistryAgentID).Error; err != nil {
		h.logger.Warn("card: registry agent missing", zap.String("slug", slug))
	}

	// Skill refs — OwnerKind=agent, OwnerID=registry agent, TargetKind=skill.
	var refs []models.RegistryRef
	_ = h.db.WithContext(r.Context()).
		Where("owner_kind = ? AND owner_id = ? AND target_kind = ?",
			models.RegistryKindAgent, agent.RegistryAgentID, models.RegistryKindSkill).
		Find(&refs).Error

	skills := make([]a2asvc.AgentCardSkill, 0, len(refs))
	for _, ref := range refs {
		skills = append(skills, a2asvc.AgentCardSkill{
			Name: lastSegmentOrName(ref.TargetName),
		})
	}

	card := a2asvc.AgentCard{
		Name:        agent.ResourceName,
		Description: reg.Description,
		URL:         strings.TrimRight(h.publicURL, "/") + "/v1/a2a/" + slug,
		Version:     agent.ResourceVersion,
		Capabilities: a2asvc.AgentCapabilities{
			Streaming:         false,
			PushNotifications: false,
		},
		Skills: skills,
	}
	writeJSON(w, http.StatusOK, card)
}

func lastSegmentOrName(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
```

Run: `go test ./internal/api/handlers/a2a/ -run TestAgentCard -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/a2a/card_handler.go internal/api/handlers/a2a/card_handler_test.go
git commit -m "$(cat <<'EOF'
feat(a2a): agent card at /v1/a2a/{slug}/.well-known/agent.json

Card is derived at request time from RegistryAgent + AgentDeployment +
skill RegistryRefs — no caching needed; everything is a single-row /
short-list DB hit. Capabilities advertise request-response only for
Phase 2; streaming lands in Phase 2.5.
EOF
)"
```

---

## Task 7: Router wiring + scope adapter bridge

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/core/config/config.go`
- Modify: `config.yaml`, `config.docker.yaml`

- [ ] **Step 1: Add config for agent invocation timeout + public URL**

Modify `internal/core/config/config.go`. Below `DeploymentConfig` add:

```go
// AgentInvocationConfig tunes the outbound /invoke calls pllm makes to
// agent pods and the public URL embedded in agent cards.
type AgentInvocationConfig struct {
	InvokeTimeout time.Duration `mapstructure:"invoke_timeout"`
	PublicURL     string        `mapstructure:"public_url"`
}
```

And add the field to the top-level `Config`:

```go
	Deployment DeploymentConfig `mapstructure:"deployment"`
	Agents     AgentInvocationConfig `mapstructure:"agents"`
```

Update `config.yaml` and `config.docker.yaml` with defaults:

```yaml
agents:
  invoke_timeout: 5m
  public_url: ""
```

- [ ] **Step 2: Add `a2aScopeAdapter` in router.go**

Right after the existing `llmScopeAdapter` (around line 67) add:

```go
// a2aScopeAdapter bridges to the narrow ScopeChecker the a2a handler expects.
type a2aScopeAdapter struct {
	inner *usecasesvc.ScopeResolver
}

func (a a2aScopeAdapter) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) a2ahandlers.ScopeDecision {
	d := a.inner.AllowAgent(ctx, useCaseID, agentName)
	return a2ahandlers.ScopeDecision{
		Allowed:     d.Allowed,
		EnforceMode: d.EnforceMode,
		Reason:      d.Reason,
	}
}
```

Import `a2ahandlers "github.com/amerfu/pllm/internal/api/handlers/a2a"` at the top if not already present.

- [ ] **Step 3: Add `RecordAgentDenial` on `scopeEventAdapter`**

Right after `RecordLLMDenial` (around line 98) add:

```go
// RecordAgentDenial implements a2ahandlers.ScopeEventRecorder.
func (a scopeEventAdapter) RecordAgentDenial(
	ctx context.Context,
	useCaseID uuid.UUID,
	keyID *uuid.UUID, keyName, target, reason string, enforced bool,
	remoteAddr, requestID string,
) {
	a.write(ctx, coremodels.ScopeEventKindAgent, useCaseID, keyID, keyName, target, reason, enforced, remoteAddr, requestID)
}
```

- [ ] **Step 4: Mount the a2a routes in the `/v1` block**

Find the `r.Route("/v1", func(r chi.Router) { ... })` block (around line 563). After the MCP section (around line 614-625) add:

```go
				// A2A: external invocation of deployed agents.
				if db != nil {
					a2aManager := a2asvc.NewManager(db)
					a2aProxy := a2asvc.NewProxy(cfg.Agents.InvokeTimeout)
					var scopeChk a2ahandlers.ScopeChecker
					var evRec a2ahandlers.ScopeEventRecorder
					if useCaseScope != nil {
						scopeChk = a2aScopeAdapter{inner: useCaseScope}
					} else {
						scopeChk = allowAllScope{} // unscoped mode
					}
					if useCaseEvents != nil {
						evRec = scopeEventAdapter{inner: useCaseEvents}
					}
					gw := a2ahandlers.NewGatewayHandler(logger, a2aManager, a2aProxy, scopeChk, evRec)
					card := a2ahandlers.NewCardHandler(logger, a2aManager, db, cfg.Agents.PublicURL)

					r.Post("/agents/{slug}/invoke", gw.InvokePlain)
					r.Post("/a2a/{slug}", gw.JSONRPC)
					r.Get("/a2a/{slug}/.well-known/agent.json", card.Card)
				}
```

Then add the `allowAllScope` fallback near the other adapters at the top of the file:

```go
// allowAllScope is used when the UseCase layer is disabled (EE-gated
// off): the A2A handler still needs a ScopeChecker, but every call is
// accepted.
type allowAllScope struct{}

func (allowAllScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) a2ahandlers.ScopeDecision {
	return a2ahandlers.ScopeDecision{Allowed: true}
}
```

Import `a2asvc "github.com/amerfu/pllm/internal/services/a2a"` at the top.

- [ ] **Step 5: Build + smoke check**

Run: `go build ./... && go vet ./...`
Expected: no errors.

Run: `go test -short ./internal/api/router/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router/router.go \
        internal/core/config/config.go \
        config.yaml config.docker.yaml
git commit -m "$(cat <<'EOF'
feat(router): mount A2A gateway + card endpoints

/v1/a2a/{slug} (JSON-RPC), /v1/agents/{slug}/invoke (plain alias), and
/v1/a2a/{slug}/.well-known/agent.json. Scope + audit adapters added
alongside mcpScopeAdapter / llmScopeAdapter. allowAllScope fallback keeps
the surface usable when the EE-gated UseCase layer is disabled.

Config: agents.invoke_timeout (default 5m) and agents.public_url (for
the card's advertised URL).
EOF
)"
```

---

## Task 8: Frontend invoke dialog

**Files:**
- Modify: `web/src/lib/api.ts`
- Create: `web/src/components/agents/invoke-dialog.tsx`
- Modify: `web/src/pages/Agents.tsx`

- [ ] **Step 1: Add API method**

Append to `web/src/lib/api.ts` next to the other agent methods (around the block added in Phase 1):

```ts
export const invokeAgent = (slug: string, input: unknown) =>
  axiosInstance.post(`/v1/agents/${slug}/invoke`, { input }) as Promise<{ output: unknown; error?: string; task_id?: string }>;
```

- [ ] **Step 2: Create the dialog**

Create `web/src/components/agents/invoke-dialog.tsx`:

```tsx
import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { invokeAgent } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";

type Props = {
  slug: string | null;
  onClose: () => void;
};

export function InvokeDialog({ slug, onClose }: Props) {
  const { toast } = useToast();
  const [input, setInput] = useState("");
  const [output, setOutput] = useState<string | null>(null);

  const invokeMut = useMutation({
    mutationFn: () => invokeAgent(slug!, input),
    onSuccess: (res: any) => {
      setOutput(typeof res.output === "string" ? res.output : JSON.stringify(res.output, null, 2));
    },
    onError: (e: any) => {
      toast({
        title: "Invoke failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  return (
    <Dialog open={!!slug} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Invoke agent</DialogTitle>
          <DialogDescription className="font-mono text-xs">{slug}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <label className="text-xs font-mono uppercase text-muted-foreground">Input</label>
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              rows={4}
              placeholder="Enter text to send to the agent"
            />
          </div>
          {output !== null && (
            <div>
              <label className="text-xs font-mono uppercase text-muted-foreground">Output</label>
              <pre className="max-h-64 overflow-auto rounded-md border bg-muted/30 p-3 text-xs">
                {output}
              </pre>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Close</Button>
          <Button
            onClick={() => invokeMut.mutate()}
            disabled={!input.trim() || invokeMut.isPending}
          >
            {invokeMut.isPending ? "Invoking…" : "Invoke"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

Adapt import paths to whatever the repo uses (check `web/src/components/ui/dialog.tsx` and `web/src/components/ui/textarea.tsx` exist — they should, both are standard shadcn).

- [ ] **Step 3: Wire it into Agents.tsx**

In `web/src/pages/Agents.tsx`:

1. At the top, add:
```tsx
import { InvokeDialog } from "@/components/agents/invoke-dialog";
```

2. Add state next to the existing `confirmDelete`:
```tsx
const [invokeFor, setInvokeFor] = useState<string | null>(null);
```

3. In each row's dropdown menu (where "Refresh status" / "Undeploy" items are), add a new item just above "Undeploy":
```tsx
<DropdownMenuItem onClick={() => setInvokeFor(a.workload_name)}>
  Invoke
</DropdownMenuItem>
```

4. Render the dialog near the `<AlertDialog>` at the bottom:
```tsx
<InvokeDialog slug={invokeFor} onClose={() => setInvokeFor(null)} />
```

- [ ] **Step 4: Lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: no new errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/components/agents/invoke-dialog.tsx web/src/pages/Agents.tsx
git commit -m "$(cat <<'EOF'
feat(ui): invoke dialog on Agents page

Minimal input/output tester hitting POST /v1/agents/{slug}/invoke. Lets
operators sanity-check a deployment without leaving the UI.
EOF
)"
```

---

## Task 9: Smoke test extension + reference pod stub

**Files:**
- Modify: `scripts/test-agent-onboarding.sh`

- [ ] **Step 1: Extend the smoke test with an invoke step**

Modify `scripts/test-agent-onboarding.sh`. After the `step "Refresh status"` block (before `step "Undeploy"`), add:

```bash
step "Invoke the agent (plain alias)"
INVOKE_RESP=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/v1/agents/$(jq -r .workload_name <<<"${DEPLOY}")/invoke" \
  -d '{"input":"hello","task_id":"smoke-1"}')
echo "invoke response: ${INVOKE_RESP}"
# We don't assert on the body — the reference agent may echo or not,
# depending on what's running in the cluster. The non-zero exit of
# curl alone proves the 200 path reached the pod.

step "Fetch agent card"
curl -fsS "${AUTH[@]}" \
  "${PLLM_URL}/v1/a2a/$(jq -r .workload_name <<<"${DEPLOY}")/.well-known/agent.json" \
  | jq -e '.name and .url and (.capabilities.streaming == false)' >/dev/null
```

- [ ] **Step 2: Document the reference pod contract**

At the top of `scripts/test-agent-onboarding.sh`, extend the existing comment block with:

```bash
# The published agent's OCI image is expected to expose POST /invoke on
# port 8000 with the Phase 2 contract:
#   request  { "input": <any>, "task_id": "<string>" }
#   response { "output": <any>, "error": "<optional string>" }
#
# A 15-line reference pod at deploy/agent-reference/echo/app.py (Flask)
# satisfies this contract and is used in demos.
```

(The reference pod itself can be added outside this plan — out of scope.)

- [ ] **Step 3: Commit**

```bash
git add scripts/test-agent-onboarding.sh
git commit -m "$(cat <<'EOF'
chore(scripts): extend smoke test to cover Phase 2 invoke + card

Exercises POST /v1/agents/{slug}/invoke and GET /v1/a2a/{slug}/.well-known/agent.json
against a deployed agent. Card assertion checks required fields.
EOF
)"
```

---

## Completion checklist

After every task above has been committed, verify:

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test -short ./...` passes
- [ ] Integration tests with `testcontainers` + `httptest` fake pod pass (Tasks 1, 3, 4, 5, 6)
- [ ] `cd web && npm run build` passes
- [ ] `scripts/test-agent-onboarding.sh` runs green against a dev pllm + minikube + a tiny reference pod that exposes `POST /invoke`

**Milestone reached:** an external caller can invoke a Phase-1-deployed agent through pllm with scope enforcement + audit events, using either A2A JSON-RPC or a plain JSON envelope. Agent cards publish interop metadata. The internal pod contract is minimal enough that a 15-line Flask app satisfies it — Phase 3 will deliver the SDK that makes the contract implicit.
