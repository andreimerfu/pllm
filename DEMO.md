# pllm — 5-Minute Demo

**Stakeholder walkthrough of the Phase 1/2 agent-onboarding pipeline.**

Narrative spine: a developer writes an agent; the platform team publishes it to
a curated registry, binds it to an approved business use case, deploys it with
one click, and gets continuous scope enforcement + audit — without writing a
single K8s manifest or scope-check middleware.

---

## What you'll see

- A **curated agent registry** — not random GitHub repos. Every artifact is
  versioned, scored, and discoverable.
- An **approved use case** that declares exactly which agents and LLMs are
  permitted. Approval lives on the use case, not on each deployment.
- A **one-click deploy** that provisions a scoped API key, applies the K8s
  Deployment, and mounts the key as a Secret — automatically.
- **Scope enforcement in action**: the Research agent probes a model it was
  not declared against; the call is blocked mid-flight with a 403.
- A **live compliance dashboard**: the denial shows up as a `scope.denied`
  event seconds later, exportable as CSV, fireable to any SIEM/GRC via webhook.

---

## Setup (one-time, ~90 s)

### Prerequisites

- minikube running with a reachable cluster
- PostgreSQL + Redis accessible to the cluster (configured in
  `deploy/k8s/secret.yaml`)
- `skaffold`, `docker`, `jq`, `curl` on PATH

### Steps

```bash
# 1. Deploy pllm into minikube (live-reloading; leave this terminal open)
skaffold dev

# 2. In a second terminal — build + load the reference agent images
make agent-echo-image-minikube
make agent-research-image-minikube

# 3. Seed demo fixtures (UseCase + two published RegistryAgents; no deploy yet)
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production \
  ./scripts/demo-seed.sh

# 4. Open the UI
open http://localhost:8080/ui
```

> **Master key:** the value in `deploy/k8s/secret.yaml` under
> `PLLM_MASTER_KEY` (default for local dev:
> `sk-master-dev-key-change-in-production`). Pass it as `PLLM_MASTER_KEY`
> or update the secret before running `skaffold dev`.

---

## The demo (5 minutes)

### 0. State the goal (30 s)

> "Every team at this org now has an AI agent. Each team builds it however they
> want — LangGraph, custom Python, whatever. The problem: how does the platform
> team control what those agents are allowed to call, audit every LLM request
> they make, and give compliance a live view — without touching each team's
> code?
>
> That's what pllm does. Let me show you."

---

### 1. Publish — the Registry (45 s)

**Click:** sidebar → **Registry** → tab **Agents**

Scroll to the two cards: `pllm.io/agent-echo v0.1.0` and
`pllm.io/agent-research v0.1.0`. Each card shows:

- Kind badge (AGENT), version pill, OCI package identifier
- Enrichment score (OSV scanner result) if the scanner has run

Click either card to open its detail drawer. Point at the **Components** tab —
it lists the servers, skills, and prompts this agent declares as dependencies.

> **Why it matters:** this is a curated internal catalog. A developer publishes
> here the same way they publish a library. The platform team reviews it once;
> every downstream deployment inherits that approval.

---

### 2. Approve — the UseCase (45 s)

**Click:** sidebar → **Use Cases**

Click the row **customer-research-approved** to open the detail drawer.

- **Overview tab:** description, data classifications, enforcement mode (block).
- **Graph tab:** React Flow dependency graph. The use case node points to
  `pllm.io/agent-echo`, `pllm.io/agent-research`, and `test-model-allowed`.
  Point out that `test-model-restricted` is intentionally **absent**.
- **Components tab:** same data in table form — agents, LLMs declared.

> **Why it matters:** the approval lives on the use case, not on each
> deployment. When risk signs off on this use case, they are signing off on
> exactly this dependency set. Adding a new model requires a use-case update —
> which creates an audit trail.

---

### 3. Deploy — the Agents page (60 s)

**Click:** sidebar → **Agents** → **Deploy agent** (top-right button)

Dialog opens. In the **Agent** dropdown pick `pllm.io/agent-echo v0.1.0`.
In the **Use case** dropdown pick `customer-research-approved`. Leave
Replicas at 1. Click **Deploy**.

Watch the Echo card appear with status pill `pending` → `deploying` → `running`
(auto-refreshes every 5 s).

Repeat for `pllm.io/agent-research v0.1.0` + `customer-research-approved`.

> **Why it matters:** one dialog, two clicks. pllm provisioned a
> UseCase-scoped API key, created a Kubernetes Deployment + Service, mounted
> the key as a K8s Secret, and registered the workload. The team's code never
> touched any of that.

---

### 4. Invoke — watch the pod run (60 s)

**On the Echo card:** click the `⋯` menu → **Invoke**

In the input box type anything (e.g., `hello from the demo`). Click **Invoke**.

The output pane shows the echo response — the exact text sent back from the
pod. The response includes the task ID the platform generated.

**On the Research card:** same flow — `⋯` → **Invoke** → type something →
**Invoke**.

The Research agent makes two internal LLM probes:
- one against `test-model-allowed` (declared in the use case) — succeeds
- one against `test-model-restricted` (not declared) — the platform
  intercepts the call and returns a 403

The JSON response from Research will contain an error field or both results
depending on how Research surfaces them — the key point is that the second
call never reached the LLM provider.

> **Why it matters:** the agent pod runs with a scoped API key. The platform
> mediates every outbound LLM and MCP call against the use case's declared
> component list. The agent's code does not implement any of this — it just
> calls the standard API.

---

### 5. Scope enforcement — Compliance (60 s)

**Click:** sidebar → **Compliance**

The page auto-refreshes every 10 s. Within seconds of the Research invocation
you'll see:

- **Posture hero:** "Runtime denial score" ticking down, "denied in last 24h"
  counter incrementing.
- **Scope events timeline:** a rose-colored bar for the denial.
- **Live feed (bottom-right panel):** the fresh `scope.denied` row with
  `test-model-restricted` as the target, the reason, and the timestamp.

Point at the **Webhooks** section at the bottom of the page. If a SIEM/GRC
endpoint is subscribed, that same event fired there as an HMAC-signed POST
within milliseconds.

Click **Export events** (top-right) to download the CSV — the format auditors
actually ask for.

> **Why it matters:** this is continuous compliance evidence, not a quarterly
> scramble. Every blocked call is persisted, timestamped, and queryable. The
> webhook makes it a push notification to whatever GRC tool the risk team
> already uses.

---

## The closing punch

> **What you saw:** a developer's agent published to a curated registry,
> approved by the platform team through a use-case binding, deployed with one
> click, invoked with live scope enforcement, and denials audited in real time.
>
> **What the platform did for you:**
> - Provisioned a UseCase-bound API key and mounted it as a K8s Secret
> - Applied a Kubernetes Deployment + Service with sane defaults
> - Intercepted every LLM call and checked it against the use case's declared
>   refs — no middleware change in the agent's code
> - Persisted a `scope.denied` audit event and fired a webhook on every blocked
>   call
>
> **What you didn't have to do:** write a K8s manifest, manage an API key,
> add a scope-check middleware, wire audit logging, or build a compliance
> dashboard.

---

## Resetting between runs

```bash
# Tear down agent deployments, RegistryAgents, and the UseCase
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production \
  ./scripts/demo-seed.sh --cleanup

# Re-seed for the next run
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production \
  ./scripts/demo-seed.sh
```

To also redeploy in one shot (skip the UI click in step 3):

```bash
./scripts/demo-seed.sh --deploy
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `demo-seed.sh` exits — `pllm is reachable` fails | pllm pod not ready | Wait for `skaffold dev` to finish; check `kubectl -n pllm get pods` |
| Agent images not found by K8s | Images not loaded into minikube | Re-run `make agent-echo-image-minikube` and `make agent-research-image-minikube` |
| Agent card stays `pending` | Pod can't pull image (ImagePullBackOff) | `kubectl -n pllm-agents describe pod <name>` — verify image name matches |
| Invoke returns 404 | Pod not `running` yet | Wait; use `⋯ → Refresh status` to force a status poll |
| No `scope.denied` row on Compliance | Wrong model name in Research agent env | Verify `PLLM_RESTRICTED_MODEL` env var matches `test-model-restricted` in the cluster ConfigMap |
| Compliance shows stale data | Page auto-refreshes every 10 s | Wait up to 10 s; or hard-reload the browser tab |
| `demo-seed.sh --cleanup` leaves a UseCase | API delete returned non-200 | `GET /api/admin/use-cases` and manually delete by ID via curl |

---

## Appendix — what's under the hood

- **Agent pod contract:** `POST /invoke` JSON `{"input": <any>, "task_id": "<string>"}` →
  `{"output": <any>, "error": "<optional string>"}`. The reference echo agent
  at `deploy/agent-reference/echo/` is ~15 lines of Flask.
- **Scope enforcement:** `AllowAgent` + `AllowLLM` + `AllowMCPTool` checked on
  a 30 s TTL cache backed by the use case's `RegistryRef` edges. Cache
  invalidates on every UseCase write.
- **Audit:** `scope.agent | scope.llm | scope.mcp_tool` events in the
  `ScopeEvent` table; UI filter at `/compliance`.
- **A2A interop:** deployed agents are also reachable via
  `/v1/a2a/{slug}/tasks/send` (JSON-RPC) and expose an agent card at
  `/v1/a2a/{slug}/.well-known/agent.json`.
- **Phase 3 (not in this demo):** `pllmctl agent` CLI, `pllm_sdk`
  Python/Node, framework templates.
