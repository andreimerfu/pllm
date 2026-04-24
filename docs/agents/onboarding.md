# Onboarding Walkthrough

End-to-end demo: publish two agents, approve a use case, deploy with one
click, invoke live, and watch scope enforcement produce an audit trail.

**Estimated time:** 5–10 minutes (after the one-time setup).

This is the same script as [`DEMO.md`](https://github.com/andreimerfu/pllm/blob/main/DEMO.md)
in the repo, reformatted for the docs.

## One-time setup

### Prerequisites

- A running minikube cluster (or any K8s that you can `kubectl` into)
- Postgres + Redis accessible to the cluster
- `skaffold`, `docker`, `jq`, `curl` on PATH

### Steps

```bash
# 1. Deploy pllm into the cluster (leave this terminal open)
skaffold dev

# 2. In a second terminal — build + load the reference agent images
make agent-echo-image-minikube
make agent-research-image-minikube

# 3. Seed demo fixtures (UseCase + two published RegistryAgents)
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production \
  ./scripts/demo-seed.sh

# 4. Open the UI
open http://localhost:8080/ui
```

## 1. Publish — the Registry

**Click:** sidebar → **Registry** → tab **Agents**

You'll see two cards: `pllm.io/agent-echo v0.1.0` and
`pllm.io/agent-research v0.1.0`. Each shows:

- Kind badge (AGENT)
- Version pill
- OCI package identifier
- OSV enrichment score (if the scanner has run)

Click either card to open its detail drawer. The **Components** tab lists the
servers, skills, and prompts this agent declares as dependencies.

> This is your curated internal catalog. A developer publishes here the same
> way they'd publish a library.

## 2. Approve — the UseCase

**Click:** sidebar → **Use Cases**

Click **customer-research-approved** to open the drawer.

- **Overview** — description, data classifications (PII, confidential), mode
  (`block`)
- **Graph** — React Flow dependency tree. The use case node points at
  `agent-echo`, `agent-research`, and `test-model-allowed`. Note that
  `test-model-restricted` is **not** in the graph.
- **Components** — same data in tabular form

> Approval lives on the use case, not on each deployment. When risk signs off,
> they're signing off on exactly this graph.

## 3. Deploy — Agents page

**Click:** sidebar → **Agents** → **Deploy agent** (top-right)

In the dialog:

- **Agent** → `pllm.io/agent-echo v0.1.0`
- **Use case** → `customer-research-approved`
- **Replicas** → 1

Click **Deploy**. The Echo card appears with status pill
`pending` → `deploying` → `running` (auto-refresh every 5 s).

Repeat for `pllm.io/agent-research v0.1.0`.

> One dialog, two clicks. pllm minted a use-case-scoped key, rendered the K8s
> manifests, mounted the key as a Secret, and registered the workload.

## 4. Invoke — watch the pod run

On the Echo card: **⋯** → **Invoke** → type `hello from the demo` → **Invoke**.
The output pane shows the echo response.

On the Research card: same. The Research agent makes two LLM probes:

- `test-model-allowed` — **succeeds**
- `test-model-restricted` — pllm intercepts and returns 403

The JSON response surfaces both results. The second call **never reached the
LLM provider** — pllm blocked it mid-flight.

## 5. Scope enforcement — Compliance page

**Click:** sidebar → **Compliance**

Within seconds of the Research invocation:

- **Posture hero** — "denied in last 24h" counter increments
- **Scope events timeline** — rose-colored bar for the denial
- **Live feed** — `scope.denied` row with target `test-model-restricted` and
  the reason

Scroll down to **Webhooks**. If a SIEM endpoint is subscribed, that same event
fired there as an HMAC-signed POST.

Click **Export events** (top-right) to download the CSV.

## What you just saw

- A developer's agent published to a curated registry
- Platform team approved it by binding a use case
- One-click deploy — no K8s manifest, no secret management
- Runtime scope enforcement — agent code untouched
- Audit trail — persisted, queryable, webhook-delivered

**What the platform did for you:**

- Minted a use-case-bound API key and mounted it as a K8s Secret
- Applied a Kubernetes Deployment + Service with sane defaults
- Intercepted every LLM call and checked it against the use case's declared
  refs
- Persisted a `scope.denied` audit event and fired a webhook

## Cleanup / reset

```bash
# Tear down agent deployments, RegistryAgents, and the UseCase
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production \
  ./scripts/demo-seed.sh --cleanup

# Re-seed
./scripts/demo-seed.sh

# Or deploy in one shot, skipping the UI clicks
./scripts/demo-seed.sh --deploy
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `demo-seed.sh` exits — "pllm reachable" fails | Pod not ready | `kubectl -n pllm get pods`; wait for `skaffold dev` to finish |
| Agent images not found | Images not loaded into minikube | Re-run `make agent-echo-image-minikube` |
| Agent card stays `pending` | `ImagePullBackOff` | `kubectl -n pllm-agents describe pod <name>` |
| Invoke returns 404 | Pod not `running` yet | **⋯** → **Refresh status** |
| No `scope.denied` row | Wrong model name in Research agent env | Check `PLLM_RESTRICTED_MODEL` env matches `test-model-restricted` |
