# Audit & Scope Events

Every allow/deny decision that the scope layer makes is written to the
`ScopeEvent` table. This is your audit trail — the evidence pack auditors,
incident responders, and compliance teams actually need.

## The event shape

```go
type ScopeEvent struct {
    ID          UUID
    UseCaseID   UUID
    KeyID       UUID

    Kind        string    // "llm" | "mcp_tool" | "agent"
    TargetKind  string    // "server" | "llm" | "agent"
    TargetName  string    // the resource requested

    // Only for kind=mcp_tool:
    ToolName    string

    Outcome     string    // "denied" | "flagged" (never "allowed")
    Reason      string    // human-readable explanation

    Metadata    JSON      // request ID, model hash, arbitrary context

    At          time.Time
}
```

Only **denied** and **flagged** outcomes are persisted. Allows are the
majority; storing every one would be expensive and not what auditors care
about. If you need allow-logging, subscribe to the HTTP access log (`/metrics`
or log aggregation) — the scope decision is recorded there too as a log field.

## Querying

```
GET /api/admin/use-cases/events
```

| Query param | Purpose |
|---|---|
| `use_case_id` | Filter to one use case |
| `kind` | `llm` / `mcp_tool` / `agent` |
| `outcome` | `denied` / `flagged` |
| `target_name` | Which resource was denied |
| `since` | ISO timestamp; events at or after this time |
| `limit` | Max rows (default 100, max 1000) |
| `offset` | Pagination |

Response:

```json
{
  "events": [
    {
      "id": "...",
      "use_case_id": "...",
      "key_id": "...",
      "kind": "llm",
      "target_name": "test-model-restricted",
      "outcome": "denied",
      "reason": "model not declared under use case",
      "at": "2026-04-24T10:12:34Z"
    }
  ],
  "total": 1337,
  "limit": 100,
  "offset": 0
}
```

## The Compliance page

`/ui` → **Compliance** is a dashboard for these events:

- **Posture hero** — 24-hour denial counts, runtime denial "score" over time
- **Scope events timeline** — 7-day bar chart; denial spikes stand out
- **Preset queries** — "denials last hour", "flagged-but-allowed last day",
  "denials by use case", etc.
- **Live feed** — bottom-right panel, auto-refreshes every 10 s
- **Webhooks section** — list of subscriptions, health, test button

It also exposes **Export events (CSV)**, which produces the format most
auditors actually ask for. Filename pattern:

```
pllm-scope-events-<use-case>-<from>-<to>.csv
```

## Retention

Events are kept indefinitely by default. Postgres will handle millions of rows
without blinking. Two common strategies if you don't want eternal retention:

1. **Partition by month** (`pg_partman`) and drop old partitions
2. **Mirror to long-term storage** via the webhook — send each event to S3 /
   BigQuery / your SIEM and prune pllm's copy after 90 days

pllm doesn't prune events automatically. A scheduled `DELETE FROM scope_events
WHERE at < now() - interval '180 days'` works fine.

## Webhooks fan-out

Every `ScopeEvent` that's persisted also fires a webhook:

- `outcome: denied` → `scope.denied`
- `outcome: flagged` → `scope.flagged`

The webhook payload is the event itself plus the bound use case's summary:

```json
{
  "event": "scope.denied",
  "delivered_at": "2026-04-24T10:12:34.123Z",
  "data": {
    "scope_event": { ... as above ... },
    "use_case": {
      "id": "...",
      "name": "customer-research-approved",
      "risk_tier": "high"
    }
  }
}
```

See [Webhooks](/governance/webhooks) for delivery guarantees and signature
verification.

## What events are NOT

They're not full request/response logs. They don't contain the prompt, the
completion, or the tool arguments. That's by design — auditors need proof
that "this use case denied this call on this date," not transcripts.

If you need full transcript logging, enable request/response tracing in the
guardrails layer or hook into the LLM gateway's middleware pipeline
independently.

## Typical audit questions

**"Show me every denied LLM call in March for HIGH-risk use cases."**

```bash
curl "http://localhost:8080/api/admin/use-cases/events?outcome=denied&kind=llm&since=2026-03-01"
```

Then filter client-side by joining to `use_cases.risk_tier = 'high'`. (A
richer `GET /events?risk_tier=high` is on the roadmap.)

**"Which models did agent-research try to call but couldn't?"**

```bash
curl "http://localhost:8080/api/admin/use-cases/events?kind=llm&outcome=denied" \
  | jq '.events | group_by(.target_name) | map({model: .[0].target_name, count: length})'
```

**"Did the flagged-mode rollout catch anything unexpected?"**

```bash
curl "http://localhost:8080/api/admin/use-cases/events?use_case_id=NEW_UC&outcome=flagged"
```
