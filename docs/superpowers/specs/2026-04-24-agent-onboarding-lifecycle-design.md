# Agent Onboarding Lifecycle — Design

**Status:** Draft · 2026-04-24
**Scope:** Developer-facing workflow for building, publishing, and deploying framework-native agents (LangGraph, Mastra, ADK, CrewAI, etc.) onto the pllm control plane, with UseCase-bound scope enforcement preserved end-to-end.

---

## 1. Context

pllm already ships three of the four primitives needed to run agentic workloads in a regulated enterprise:

- **Registry** — `RegistryAgent / RegistryServer / RegistrySkill / RegistryPrompt` with versioning, OSV scoring, npm/PyPI/OCI importers, `RegistryRef` as the graph edge table.
- **LLM + MCP gateway** — OpenAI-compatible `/v1/*` surface, MCP streamable-HTTP at `/v1/mcp`, per-tool ACL on keys.
- **MRM layer** — `UseCase` as an approvable top-level entity, `ScopeResolver`, `Reconciler`, `EventRecorder`, `Webhook` dispatch, Compliance UI.

What is missing is a **developer path for agents**. Today `RegistryAgent` is a manifest only — there is no scaffolder, no deploy adapter, no runtime invocation surface, and no SDK that wires a framework-native agent (LangGraph, etc.) into pllm's gateway with a UseCase-bound key.

The ecosystem is converging on the shape of this flow — see `agentregistry-dev/agentregistry` (agent-as-a-container, registry catalog, K8s deploy) and `agentgateway/agentgateway` (LLM + MCP + A2A proxy). Neither project wires through a UseCase/scope-enforcement layer. That wiring is this design's differentiator.

## 2. Goals

1. A developer can go from `pllmctl agent init` to a running, UseCase-bound, observable agent in under 30 minutes.
2. The same agent image can deploy into dev / staging / prod with different UseCases and different scope boundaries — **UseCase binding happens at deploy time, not publish time**.
3. Scope enforcement is visible at **dev time** via `pllmctl dev --bind-use-case`, not only post-deploy.
4. Every LLM / MCP / A2A call made by or to a deployed agent is scope-checked, audited (`ScopeEvent`), and alertable via webhook — zero code changes in the agent itself.
5. The onboarding surface is shaped like agentregistry's (container + manifest + registry + K8s deploy) so the ecosystem tooling interoperates.
6. **Reuse over new infrastructure.** Approximately 60 % of the work reuses existing pieces (`K8sAdapter`, `MCPBridge`, `RegistryAgent`, `RegistryRef`, `ScopeResolver`, `Reconciler`, `EventRecorder`, `Webhook` dispatcher). The remaining 40 % is net-new and well-bounded.

## 3. Non-goals

- **No new agent framework.** We ship an SDK that configures existing frameworks; we do not replace LangGraph / Mastra / ADK.
- **No serverless / ephemeral agent runtime.** Agents run as long-lived HTTP servers in pods. Per-invocation ephemeral runtimes are deferred.
- **No cross-cluster federation.** `AgentAdapter` deploys to the same Kubernetes cluster pllm already targets.
- **No guardrail application to agent outputs** in this scope (tracked separately under the existing guardrails gap).
- **No non-Kubernetes deploy target.** Same constraint as the MCP deployment path today.

## 4. The 8-stage developer lifecycle

Walked with a concrete scenario: `sales-research` — a LangGraph agent that reads customer documents from SharePoint, summarized for a relationship manager. The bank has an approved `customer-research-approved` UseCase.

### Stage 1 — Scaffold

```
pllmctl agent init sales-research \
  --framework langgraph \
  --deps server:sharepoint-mcp,llm:gpt-4o-mini
```

Produces:

```
sales-research/
├── agent.yaml              # manifest — shape matches RegistryAgent + refs
├── Dockerfile              # FROM pllm/agent-base-python:1
├── app/
│   ├── agent.py            # LangGraph graph pre-wired with pllm_sdk
│   └── requirements.txt
├── .env.example            # PLLM_BASE_URL, PLLM_API_KEY, PLLM_MCP_URL, PLLM_USE_CASE
└── README.md
```

Templates live in `deploy/agent-templates/{langgraph,mastra,adk,crewai,raw-python,raw-ts}/`. Each template is a minimal runnable example.

### Stage 2 — Develop locally

Scaffolded `app/agent.py` opens with:

```python
from pllm_sdk import configure
cfg = configure()                       # reads PLLM_* env vars

llm = cfg.chat_model("gpt-4o-mini")     # → /v1/chat/completions
mcp = cfg.mcp_client()                  # → /v1/mcp, auto-discovers tools
```

Developer sets `.env.local` with a personal dev key and runs the agent normally. Framework-native tests work unchanged — pllm is a transparent gateway.

### Stage 3 — Test against a real UseCase

```
pllmctl dev --bind-use-case customer-research-approved
```

Provisions a short-lived dev key with `Key.UseCaseID = <customer-research-approved.id>` and streams `ScopeEvent`s over SSE to the developer's terminal:

```
✓ llm:gpt-4o-mini            allowed
✓ sharepoint__read_file      allowed
✗ sharepoint__delete_file    scope.denied — target not in use case
```

Developers see scope violations while iterating, not after the compliance review.

### Stage 4 — Package

```
pllmctl agent build .         # → pllm/agents/sales-research:0.1.0
pllmctl agent push
```

Standard container build. `pllm/agent-base-python:1` (new, at `deploy/agent-base/python/`) includes:

- `pllm_sdk` pre-installed
- A thin HTTP server that exposes `/a2a` + `/.well-known/agent.json` automatically, so developer code does not write the server boilerplate
- Health endpoints at `/healthz` + `/readyz`

Parallel `pllm/agent-base-node:1` for TS frameworks (Mastra, AI SDK).

### Stage 5 — Publish to registry

```
pllmctl agent publish --version 0.1.0
```

Creates a `RegistryAgent` row, resolves declared `deps` into `RegistryRef` rows, triggers enrichment (OSV + license + basic static analysis). The artifact appears in the Registry UI with its score ribbon, install command, version picker — identical treatment to a published MCP server.

**After this stage the agent is discoverable but not authorized to run anywhere.** Authorization is attached via a UseCase (stage 6), not granted by publication.

### Stage 6 — Attach to a UseCase (platform team)

Platform team opens `customer-research-approved` in the Compliance UI, attaches `sales-research@0.1.0`:

1. Creates `RegistryRef{OwnerKind: use_case, OwnerID: <uc.id>, TargetKind: agent, TargetName: sales-research, TargetVersion: 0.1.0}`.
2. `Reconciler` walks the dependency graph (agent → sharepoint-mcp → `{PII, customer-confidential}` data classes) and updates `UseCase.EffectiveClassifications`.
3. If the newly introduced data classes exceed what the UseCase is approved for, the UI flags the delta and blocks save until a compliance review lands.
4. On save, webhook fires `use_case.updated` to SIEM / GRC subscribers.

No new tables. Reuses the existing `RegistryRef` and `Reconciler` exactly.

### Stage 7 — Deploy

```
pllmctl agent deploy sales-research@0.1.0 \
  --use-case customer-research-approved \
  --env prod --replicas 2
```

`AgentAdapter` (new, parallel to `K8sAdapter` for MCP):

1. Provisions a scoped `Key` with `UseCaseID = <customer-research-approved.id>`; stores as a K8s `Secret`.
2. Applies `Deployment + Service + optional NetworkPolicy` in the `pllm-agents` namespace.
3. Injects env vars on the pod: `PLLM_BASE_URL`, `PLLM_API_KEY` (from Secret), `PLLM_MCP_URL`, `PLLM_USE_CASE`.
4. `AgentBridge` (new, parallel to `MCPBridge`) registers the running pod as an **A2A backend** under its slug.
5. Exposes `/v1/a2a/sales-research` + `/v1/agents/sales-research/invoke` through the gateway.

### Stage 8 — Invoke and observe

External caller (Slack bot, another agent, or a human via the UI):

```
POST /v1/a2a/sales-research
Authorization: Bearer sk-caller-...
{ "task": "Summarize Acme Corp's Q3 engagement history" }
```

Runtime path:

1. pllm checks the caller's key scope — A2A invocation is itself a scoped operation (new capability in `ScopeResolver`: `AllowAgentInvoke`).
2. pllm proxies to the agent pod, forwarding task payload + W3C trace context headers.
3. The agent calls `/v1/chat/completions` — its own deploy-time key's `UseCaseID` gates the call.
4. The agent calls `/v1/mcp` for SharePoint — gate passes for `read_file`, denies `delete_file`.
5. Every call emits `ScopeEvent` rows. Denials fire the `scope.denied` webhook.
6. Deployments UI shows live traffic + denials; Compliance UI shows the audit trail unchanged.

## 5. Architecture additions

```
cmd/cli/commands/
├── agent.go                 # init, build, push, publish, deploy, invoke
└── dev.go                   # `pllmctl dev --bind-use-case` SSE client

sdk/
├── python/pllm_sdk/         # configure(), chat_model(), mcp_client(), a2a_server()
└── node/@pllm/sdk/          # same surface, TS

deploy/
├── agent-base/
│   ├── python/Dockerfile    # base image with SDK + A2A server + health probes
│   └── node/Dockerfile
└── agent-templates/{langgraph,mastra,adk,crewai,raw-python,raw-ts}/

internal/services/
├── deployment/
│   ├── agent_adapter.go     # new — parallel to k8s_adapter.go
│   └── agent_bridge.go      # new — parallel to mcp_bridge.go
├── mcp/
│   └── transport/a2a.go     # new — A2A protocol client/server
└── a2a/                     # new package
    ├── manager.go           # backend registry, parallel to mcp/gateway/manager.go
    ├── backend.go
    └── handler.go           # /v1/a2a/{slug} and /v1/agents/{slug}/invoke

internal/api/handlers/
├── a2a/                     # new
│   ├── gateway_handler.go
│   └── admin_handler.go
└── dev/                     # new — SSE stream for `pllmctl dev`
    └── scope_events.go

web/src/
├── pages/Agents.tsx         # list + detail for running agents
└── components/agents/       # invoke tester, scope-denial inline panel
```

### Data model impact

**At most one new table** (pending resolution of §8 Q1 — `AgentBackend` vs. extending `MCPServer` with a `Kind` column). Existing reuse:

- `RegistryAgent` — already has `Packages` (OCI refs) and `Remotes` (invocation endpoints).
- `RegistryRef` — edges from UseCase → agent (already supported).
- `Key.UseCaseID` — scope binding (already supported).
- `ScopeEvent` — audit log (already supported).

If a new `AgentBackend` table lands, it must be added to all three `AutoMigrate` lists per repo convention (`database/connection.go`, `database/migrate.go`, `testutil/database.go`).

### Narrow interfaces (per the CLAUDE.md pattern)

- `a2a.ScopeChecker` (3 methods) — bridged in `router.go` to `usecasesvc.ScopeResolver`, mirroring `mcphandlers.ScopeChecker`.
- `a2a.ScopeEventRecorder` — bridged to `usecasesvc.EventRecorder`.
- `deployment.AgentProvisioner` — the consumer interface `agent_adapter.go` exposes; service layer bridges to the `K8sAdapter`.

No handler package imports `use_cases` directly. EE-gate placement unchanged.

## 6. API surface additions

### LLM + MCP gateway — unchanged

### A2A gateway (new)

- `POST /v1/a2a/{slug}` — streamable-HTTP A2A invocation
- `GET /v1/a2a/{slug}/.well-known/agent.json` — agent card proxy
- `POST /v1/agents/{slug}/invoke` — OpenAI-assistants-compatible alias for familiarity

### Admin (auth + admin role)

- `POST /api/admin/agents/deploy` — body: `{name, version, use_case_id, env, replicas}`
- `DELETE /api/admin/agents/{slug}` — tear down deployment + scoped key
- `GET /api/admin/agents` — list running agent deployments (separate from MCP deployments listing)
- `GET /api/admin/agents/{slug}/logs` — pod logs proxy (already pattern-matched from MCP deployments)

### Dev endpoint

- `GET /api/dev/scope-events?key_id=<dev-key>` — SSE stream of `ScopeEvent`s for that key; used by `pllmctl dev`

## 7. Key design decisions

### D1 — UseCase binding at deploy time, not publish time

The same agent image deploys into different UseCases (dev, staging, prod, and even different business UseCases) with different scope boundaries. Binding lives on the `Deployment` row and on the scoped `Key`, not on the `RegistryAgent`.

**Rejected alternative:** bake UseCase into manifest at publish. Simpler but locks each published version to one environment — breaks the multi-env story that is the MRM layer's whole point.

### D2 — A2A as the default invocation protocol

A2A is the emerging open protocol (Google-originated, now in agentgateway and multiple frameworks). Default. Ship an OpenAI-assistants-compatible `/v1/agents/{name}/invoke` alias for developer familiarity — same backend handler, different JSON envelope.

**Rejected alternative:** roll our own simpler JSON protocol. No interop story; shuts the door on external agent frameworks calling pllm-hosted agents and vice versa.

### D3 — Agents run as long-lived HTTP servers in pods

Matches agentregistry's shape. Reuses `K8sAdapter` and `MCPBridge` patterns nearly directly.

**Rejected alternative:** ephemeral jobs per invocation. Cheaper at idle, but complicates state, streaming responses, and session pinning. Defer.

### D4 — SDK is thin (reads env vars, returns configured clients)

`pllm_sdk.configure()` reads 4 env vars and returns a config object that hands out pre-configured OpenAI client, MCP client, and A2A server. No pllm-specific concepts leak into the developer's graph / chain / runner. Swap the SDK out and the agent still runs — it just loses the gateway benefits.

### D5 — Dev-time scope visibility via SSE

`pllmctl dev --bind-use-case` streams `ScopeEvent`s in real time. Implementation: new `/api/dev/scope-events` endpoint filtered by key ID; existing `EventRecorder` already persists the data.

## 8. Open questions (to resolve during plan)

1. **AgentBackend storage** — reuse `MCPServer` with a `Kind` column (`mcp | agent`), or add a parallel `AgentBackend` table. Reuse is simpler; separate is cleaner at the domain level. Recommendation: **separate `AgentBackend` table**, because invocation semantics differ enough that a shared table grows conditionals.
2. **Agent card schema alignment** — track the A2A spec revision we target (pre-1.0 churn is real). Pin to a specific commit; bump with intent.
3. **SDK package hosting** — publish `pllm-sdk` to PyPI + npm, or vendor-only initially. Recommendation: PyPI + npm from day one, with `dev` channel until v1.
4. **Framework templates** — start with LangGraph + Mastra + raw Python; add ADK, CrewAI, AI SDK after first external user. Each template is small but has maintenance drag.
5. **Concurrency / session stickiness** — when invocations need to hit the same pod replica (conversational state), pllm needs session pinning. Already in the "known gaps" list for MCP; lift into scope if the bank demo needs it, else defer.

## 9. Phasing

The work decomposes into four independently shippable phases. Each phase is demo-able end-to-end.

**Phase 1 — Agent deploy, no SDK (~1 week)**
- `AgentAdapter` + `AgentBridge`
- Admin API: `POST /api/admin/agents/deploy`, `DELETE /api/admin/agents/{slug}`
- Agents page in the UI (read-only)
- Hand-written env-var wiring in the agent code; no SDK yet
- **Milestone:** deploy a hand-written LangGraph agent, see it run, see scope events.

**Phase 2 — A2A invocation (~1 week)**
- `internal/services/a2a/` manager + backend + handler
- `/v1/a2a/{slug}` + `/v1/agents/{slug}/invoke`
- Agent card proxy
- **Milestone:** invoke the Phase-1 agent from outside the cluster through pllm.

**Phase 3 — SDK + base images + templates (~1 week)**
- `pllm_sdk` Python + `@pllm/sdk` Node
- `agent-base-python` + `agent-base-node` Docker images
- LangGraph + Mastra + raw-python templates
- `pllmctl agent init/build/push/publish/deploy` commands
- **Milestone:** `pllmctl agent init && ... && pllmctl agent deploy` in under 30 minutes for a LangGraph agent.

**Phase 4 — Dev-time scope binding + polish (~3 days)**
- `/api/dev/scope-events` SSE endpoint
- `pllmctl dev --bind-use-case` command
- Agent invoke tester in the UI
- Inline scope-denial panel on the agent detail page
- **Milestone:** developer sees `scope.denied` in their terminal while editing code.

Total: ~3–4 weeks elapsed. Phase 1 is shippable on its own; every subsequent phase strengthens the developer story without blocking earlier value.

## 10. Testing strategy

- Integration tests under `testing.Short()` gate, following repo convention.
- New `AgentAdapter` exercised via testcontainers K8s (kind) — mirror the pattern the MCP adapter uses if present, otherwise add a minimal fake `kubernetes.Interface`.
- Extend `scripts/test-mcp-registry.sh` into `scripts/test-agent-lifecycle.sh` covering: scaffold → build (skipped in CI, bring-your-own image) → publish → attach-to-use-case → deploy → invoke → verify denial event.
- SDK packages ship with contract tests against a mock pllm (OpenAI-compatible + MCP endpoints).
- Do **not** add new AutoMigrate entries unless the `AgentBackend` open question lands on a new table; if it does, remember to update all three migrator lists.

## 11. Risks

- **A2A spec churn** — pre-1.0. Mitigate with a pinned version and a single `a2a.Version` constant.
- **Agent base image size** — Python + Node base images risk bloat. Target <200 MB each; alpine where feasible; document `FROM` override for teams that want their own base.
- **Scope-check blast radius** — A2A invocation scope checks go through the same `ScopeResolver` as chat + MCP. If that path is slow under load, every agent invocation inherits the latency. Mitigate with the 30 s TTL cache (already present) and explicit benchmarking in Phase 2.
- **EE gating** — if the MRM layer becomes a licensed feature, this design keeps scope enforcement on the A2A path optional-but-default. Same `cfg.UseCaseHandler != nil` guard as existing code.
