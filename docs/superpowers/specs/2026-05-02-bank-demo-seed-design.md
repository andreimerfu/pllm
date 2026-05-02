# Bank Demo Seed — Design

**Date:** 2026-05-02
**Status:** Proposed
**Owner:** governance / demo readiness
**Audience:** mixed risk-officer + platform-engineer demo, run locally and on real K8s

---

## Problem

The Governance section (Use Cases + Compliance) has no realistic seed
data. Operators see an empty matrix, a flat posture chart, and one
fixture use case with `200% enforced`. The bank-MRM story we keep
referencing in CLAUDE.md ("declare → enforce → audit") cannot be told
without a populated portfolio, a populated dashboard, and a working
runtime path through the gateway.

We need a one-command seed that produces a believable bank scenario,
runs the same way on a developer laptop and on the K8s cluster the
real demo will be given on, and never depends on credentials we do
not already have configured.

## Goals

- One coherent **bank scenario** spanning the matrix (T1 blocking, T1
  in-flight review, T2 blocking, T4 sandbox).
- **Real LLM call** through pllm — no mock provider.
- Catalog content for the Registry: servers, agents, prompts, skills.
- A populated Compliance dashboard — 7 days of scope events with both
  allowed and denied calls so the timeline, top-violators, and posture
  score all have meaning.
- **Idempotent**: re-running the seed updates instead of erroring;
  `--cleanup` removes exactly what was seeded.
- **K8s-deployable**: same code path runs from an operator laptop or
  as a one-shot K8s Job using the existing `pllm` image.
- **Demo arc** that closes the loop: catalog → risk view → drill-in →
  compliance → live denial → webhook fires.

## Non-goals

- Building a real bank knowledge base, transactions service, or
  customer database. Catalog entries are believable metadata only.
- Replacing the existing `scripts/test-mcp-registry.sh` smoke test.
- Authoring net-new MCP servers — we use what `wrapper-image-minikube`
  already supports plus catalog-only entries.
- Multi-tenant fixtures. One demo namespace, master-key only.

## Design

### Delivery

A new `pllm seed` Cobra subcommand in `cmd/cli/commands/seed.go`,
exposed as `pllm seed bank`. Lives next to the existing `user`,
`team`, `key`, `budget`, `config` subcommands and follows their
shape.

The CLI talks to pllm via the public REST API using the master key —
no direct DB access. This keeps the seed environment-agnostic
(localhost, in-cluster Service, remote tunnel) and means the seed
itself doubles as live API documentation.

#### Three entrypoints

1. **`make demo-seed` / `make demo-seed-clean`** — local dev. Runs
   the binary against `localhost:8080`.
2. **`pllm seed bank` directly** — operator runs it from anywhere
   that can reach the gateway and has the master key.
3. **`deploy/k8s/demo-seed-job.yaml`** — one-shot K8s Job referencing
   the existing pllm image, with envFrom pulling `PLLM_MASTER_KEY`
   from the install secret and `PLLM_BASE_URL` defaulting to the
   in-cluster Service. `kubectl apply -f` is the demo prep step.

The legacy `scripts/demo-seed.sh` becomes a thin wrapper: prefer
`pllm seed bank` if on `$PATH`, fall back to inline curl calls so
existing muscle memory still works.

### Bank cast

| # | Slug | Tier | Lifecycle | Mode | Data classes | Components |
|---|---|---|---|---|---|---|
| 1 | `loan-underwriting-assistant` | T1 | Approved | Blocking | PII, customer-financial, confidential | LLM (route) · `policy-docs` (catalog) · `credit-scoring` (catalog) · `loan-officer-copilot` agent · `decision-rationale` prompt |
| 2 | `aml-investigation-copilot` | T1 | Under review | Flag | PII, customer-financial, confidential | LLM (route) · `transactions-readonly` (catalog) · `tavily-search` (catalog) · `kyc-summarizer` agent |
| 3 | `branch-staff-knowledge-search` | T2 | Approved | Blocking | internal | LLM (route) · `policy-docs` (catalog) · `filesystem` (deployed if `--with-deployments`) |
| 4 | `marketing-copy-drafting` | T4 | Draft | Flag | public | LLM (route) · `server-everything` (deployed if `--with-deployments`) |

This shape was chosen because:

- All four lifecycle stages get representation (draft, under review,
  approved × 2). The `Risk × lifecycle` matrix lights cells across
  rows and columns.
- All three risk-relevant tiers are present (T1 hot, T2 warm, T4
  cool).
- Both blocking and flag mode are represented at T1 — the flag-mode
  T1 use case (`aml-investigation-copilot`) is the "reviewers are
  watching" story we want for the live denial demo.
- Data-class spread is realistic: `confidential` and `customer-financial`
  on the high-tier flows, `internal` on the operational one,
  `public` on the sandbox. The Classification Mix panel becomes
  interesting.

### Component catalog

Seeded as `RegistryServer` / `RegistryAgent` / `RegistrySkill` /
`RegistryPrompt`, all under the `bank.demo/*` slug namespace except
the public entries (`tavily-search`, `filesystem`, `server-everything`)
which use their conventional names.

#### Servers (catalog-only)

| Slug | Purpose | Latest version |
|---|---|---|
| `bank.demo/policy-docs` | "Internal policy + procedure RAG" | 1.0.0 |
| `bank.demo/credit-scoring` | "Credit-score lookup" | 1.0.0 |
| `bank.demo/transactions-readonly` | "Read-only transaction history" | 1.0.0 |
| `bank.demo/customer-360` | "Aggregated customer profile" | 1.0.0 |
| `tavily-search` | "Web search" | 1.0.0 |

Each gets:
- A believable `description`
- A forced-set `EnrichmentScore` (90/85/80/75/95) so the score ribbon
  on `/registry` renders without waiting for OSV runs
- `status = "active"`

#### Servers (live, optional)

When `--with-deployments` is passed:

- `@modelcontextprotocol/server-filesystem` published to registry
  under `mcp.io/filesystem` and deployed via the existing wrapper
  (read-only mount, sandboxed path).
- `server-everything` published under `mcp.io/server-everything` and
  deployed via the wrapper.

These reuse `MCPBridge` so they appear under
`reg-mcp-io-filesystem__*` and `reg-mcp-io-server-everything__*` in
`/v1/mcp`, which lets the demo invoke real MCP tools.

#### Agents

- `bank.demo/loan-officer-copilot` v0.1.0 — refs `policy-docs`,
  `credit-scoring`, `decision-rationale` prompt.
- `bank.demo/kyc-summarizer` v0.1.0 — refs `transactions-readonly`,
  `kyc-summary-template` prompt, `pii-redactor` skill.

#### Prompts

- `bank.demo/decision-rationale` — "Walk through underwriting decision"
- `bank.demo/kyc-summary-template` — "Summarize KYC findings"

#### Skills

- `bank.demo/pii-redactor` — "Redact PII from free text"

### LLM dependency

Every use case declares its LLM dependency by **route name**, not
model instance — that's how the existing scope check works for
chat. Default route is `free-route` (overridable with
`--llm-route`). The route must already exist on the gateway; the
seed verifies this with a `GET /api/admin/routes` filter (the same
endpoint the Routes UI uses) and aborts cleanly with a guidance
message if it doesn't:

```
error: route "free-route" not found.
  pllm seed bank requires an LLM route. Either:
    - create one in the UI at /ui/routes
    - or pass --llm-route <existing-route-name>
```

This means the demo's "real LLM call" goes through whatever provider
the operator has configured behind that route. Zero new credentials.

### API keys

Three demo keys, printed at the end of a successful seed:

| Key name | Bound use case | Notes |
|---|---|---|
| `demo-loan-officer` | `loan-underwriting-assistant` | Happy path. Used to demonstrate allowed call to `--llm-route`, denied call to undeclared model. |
| `demo-aml-analyst` | `aml-investigation-copilot` | Flag-mode demo. Out-of-scope calls log but proceed. |
| `demo-marketing` | `marketing-copy-drafting` | Sandbox key for the T4 walkthrough. |

Keys are reset to known plaintext values on each run (a `--rotate`
flag would be a follow-up if security concerns surface — for a demo
seed in a throwaway environment this is acceptable).

### Scope event history

Approximately 30 events spread across the prior 7 days. Distribution:

- `loan-underwriting-assistant`: ~12 allowed (varied targets within
  scope), ~5 denied yesterday targeting `gpt-4-turbo` (undeclared)
  with reason `model gpt-4-turbo is not declared by the bound use case`.
- `aml-investigation-copilot`: ~6 allowed, ~3 flagged on
  `tavily-search` outside scope (flag mode, so logged, not blocked).
- `branch-staff-knowledge-search`: ~4 allowed.

This populates:
- The 7-day timeline (not flat)
- The "Use cases with most denials" panel (loan-underwriting on top)
- The "Most-denied targets" panel (`gpt-4-turbo` on top)
- The posture score (lands ~70-80, in the "watch" band, which is the
  demo-friendly state — neither all-green nor crisis)

#### Required new admin endpoint

`POST /api/admin/use-cases/events:bulk` (master-key only). Accepts:

```json
{
  "events": [
    {
      "use_case_id": "uuid",
      "kind": "llm_model" | "mcp_tool",
      "outcome": "denied" | "flagged",
      "target_name": "gpt-4-turbo",
      "key_name": "demo-loan-officer",
      "reason": "model gpt-4-turbo is not declared by the bound use case",
      "remote_addr": "10.42.0.7",
      "created_at": "2026-04-26T12:34:56Z"
    }
  ]
}
```

Returns `{"inserted": <count>}`. Documented as a test/demo aid in the
handler godoc and the swagger spec. The endpoint reuses the existing
`EventRecorder` service rather than going around it. We add it
because the alternative — having the seed tool open a Postgres
connection — would couple the CLI to DB internals and break the
"REST-only" contract that makes K8s deployment one-line.

### Webhook subscription

One disabled webhook seeded:

- name: `demo-siem-stub`
- url: `https://webhook.site/REPLACE-ME`
- events: all `scope.*` and `use_case.*` event types
- enabled: false

Operator replaces the URL during demo prep, flips the toggle, and
demonstrates an out-of-scope call producing a `scope.denied` POST
visible in the webhook.site UI.

### Ordering & idempotency

Seed runs in dependency order:

1. Verify gateway health + master key works
2. Verify `--llm-route` exists; abort cleanly otherwise
3. Skills, prompts → standalone artifacts
4. Servers (catalog) → standalone
5. Servers (live, if requested) → publish + deploy
6. Agents → reference 3-5
7. Use cases → reference 3-6
8. Keys → bound to use cases from 7
9. Scope events history → references use cases from 7
10. Webhook → standalone

For each entity the seed:
- GETs by slug (or list-and-filter where no by-slug endpoint exists);
  if missing, POSTs; if present, PATCHes the mutable fields.
  Versioned registry artifacts check `is_latest` before publishing a
  new version — re-runs at the same logical version are no-ops.
- Caches the gateway-assigned UUID per slug in an in-memory map for
  the duration of one seed run, so dependents (use cases referencing
  components, refs referencing both endpoints) resolve without
  re-querying.

### Cleanup (`--cleanup`)

- Deletes by namespace prefix:
  - Use cases where slug matches `loan-underwriting-assistant` /
    `aml-investigation-copilot` /
    `branch-staff-knowledge-search` / `marketing-copy-drafting`
  - Keys with names starting `demo-`
  - Webhooks named `demo-siem-stub`
  - Registry artifacts under `bank.demo/*`
  - Catalog public entries (`tavily-search`, `mcp.io/filesystem`,
    `mcp.io/server-everything`) only if their description matches
    the seed's marker comment, so we don't blow away an operator's
    own entries with the same name
  - Scope events with `key_name` matching `demo-*`
- Idempotent — missing entities are silently skipped.

### Flags

```
pllm seed bank [flags]
  --base-url           gateway URL (default: $PLLM_BASE_URL or http://localhost:8080)
  --master-key         admin key (default: $PLLM_MASTER_KEY)
  --llm-route          route name use cases declare (default: free-route)
  --with-deployments   also deploy 2 live MCP servers (requires K8s + wrapper image)
  --with-events        seed 7-day scope event history (default: true)
  --with-webhook       seed disabled webhook subscription (default: true)
  --webhook-url        URL to use if --with-webhook (default: https://webhook.site/REPLACE-ME)
  --cleanup            remove all `bank.demo/*` and `demo-*` fixtures and exit
  --json               machine-readable output (default: false)
```

### File layout

```
cmd/cli/commands/seed.go           # cobra command wiring + flag parsing
cmd/cli/commands/seed/             # seed package
  bank.go                          # the bank scenario fixtures + run loop
  client.go                        # thin REST client (reused master-key auth)
  cleanup.go                       # cleanup paths
  events.go                        # scope event history generator
  fixtures.go                      # the bank cast as Go literals
  print.go                         # human/json output helpers

internal/api/handlers/use_cases/   # existing
  scope_events.go                  # ADD: BulkSeed handler
internal/api/router/router.go      # ADD: route the bulk endpoint
internal/services/use_cases/       # existing
  event_recorder.go                # ADD: BulkInsert(events) method

deploy/k8s/demo-seed-job.yaml      # NEW: one-shot Job manifest
Makefile                           # ADD: demo-seed, demo-seed-clean targets
scripts/demo-seed.sh               # MODIFY: thin wrapper around `pllm seed bank`
```

## Demo arc enabled

1. **Catalog walk** — `/registry` shows the bank components with
   scores
2. **Risk view** — `/use-cases` matrix lights up T1/T2/T4, the
   tooltips and the new explainer popover land for first-time
   viewers, classification mix shows real distribution
3. **Drill-in** — click `loan-underwriting-assistant`, show the
   graph, classifications, blocking mode chip
4. **Compliance** — `/compliance` posture is in the watch band
   (~70-80 — not crisis, not bored), 7-day timeline has shape, "Use
   cases with most denials" features `loan-underwriting-assistant`
5. **Live fire** — operator hits `/v1/chat/completions` with
   `demo-loan-officer` key + `--llm-route` → success. Then with the
   same key + `gpt-4-turbo` → 403 with scope reason. New event
   appears live in the timeline
6. **Webhook flip** — enable the seeded webhook (after replacing the
   URL with their webhook.site listener), repeat the denial, show
   the POST landing in webhook.site

## Risks & open questions

- **Route assumption.** The seed assumes a usable LLM route exists.
  Mitigation: explicit pre-flight check + guidance message
  ("`pllm seed bank` requires a route. Configure one at
  `/ui/routes` first, or pass `--llm-route <name>`.").

- **Scope-event endpoint shape.** Bulk-insert is unusual in this
  codebase — most admin write paths are one-at-a-time. We accept this
  because the demo needs ~30 events with backdated timestamps, and
  doing 30 sequential POSTs over a remote tunnel is slow enough to
  feel broken.

- **Live deployments are optional.** The seed is fully meaningful
  without `--with-deployments` (the catalog-only servers still
  appear in graphs and refs). Operators on a fresh cluster without
  the wrapper image still get the full UI story; only the live MCP
  tool-call demo needs `--with-deployments` + wrapper.

- **Catalog vs deployed visual cue.** The UI doesn't currently
  distinguish a registered-only server from a deployed one in a
  glanceable way. Out of scope here, but worth a follow-up — for
  now the operator just knows which is which from the seed output.

- **Cleanup safety on shared environments.** The seed targets the
  `bank.demo/*` namespace + `demo-*` key-name prefix. We will not
  ship a `--force` that touches anything else. If an operator
  pre-creates entities with these names, cleanup is destructive —
  this is acceptable for a demo seed; documented in the command
  help.

## Out of scope (for follow-up)

- Multi-tenant or per-team fixtures
- Localization of seeded prompts/descriptions
- A "scenario picker" supporting non-bank verticals (insurance,
  healthcare). The same seed framework would extend cleanly when
  a second scenario is needed.
- Automatic webhook replay during seed (would require a real
  endpoint to POST to)
