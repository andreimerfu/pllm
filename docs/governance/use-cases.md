# Use Cases

A **UseCase** is the top-level approval unit. It declares which resources
(agents, servers, skills, prompts, LLMs) are permitted for a given business
activity, and it carries the data classifications, risk tier, and approval
record that live with that activity.

## The model

```go
type UseCase struct {
    ID          UUID
    Name        string    // human-friendly identifier, unique
    Description string
    Status      string    // draft | pending_approval | approved | archived
    RiskTier    string    // low | medium | high | critical

    // Data classifications (user-tagged)
    DataClassifications []string  // e.g. ["PII", "confidential", "public"]

    // Effective classifications (Reconciler-computed, materialized)
    EffectiveClassifications []string  // includes transitive

    // Enforcement
    EnforceScope bool       // false = flag mode, true = block mode

    // Compliance
    ComplianceFlag string   // set by Reconciler if violations detected
    ApprovalRecord JSON     // opaque — ticket ID, approver, timestamps

    // Ownership
    TeamID    UUID
    OwnerID   UUID

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Components (agents, servers, etc.) are not stored on the UseCase row — they
live in `RegistryRef` with `OwnerKind="use_case"`. One table, one edge shape.

## CRUD endpoints

```
GET    /api/admin/use-cases                    List
POST   /api/admin/use-cases                    Create
GET    /api/admin/use-cases/{id}               Read
PUT    /api/admin/use-cases/{id}               Update
DELETE /api/admin/use-cases/{id}               Delete

GET    /api/admin/use-cases/events             Scope event audit log
POST   /api/admin/use-cases/{id}/recompute     Force reconcile
```

## Creating a UseCase

```bash
curl -X POST http://localhost:8080/api/admin/use-cases \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "customer-research-approved",
    "description": "Research agent used by Customer Success team",
    "status": "approved",
    "risk_tier": "high",
    "data_classifications": ["PII", "confidential"],
    "enforce_scope": true,
    "approval_record": {
      "ticket": "GRC-2345",
      "approver": "risk-team@example.com"
    },
    "refs": [
      {
        "target_kind": "agent",
        "target_name": "pllm.io/agent-research",
        "target_version": "0.1.0"
      },
      {
        "target_kind": "agent",
        "target_name": "pllm.io/agent-echo"
      },
      {
        "target_kind": "llm",
        "target_name": "my-gpt-4"
      },
      {
        "target_kind": "server",
        "target_name": "official/github-mcp"
      }
    ]
  }'
```

The `refs` array creates the `RegistryRef` edges atomically. If you update the
use case later (PUT), the refs are diffed and updated — additions are new rows,
removals are deletes.

## The Reconciler

On every Create and Update, the Reconciler walks the ref graph (BFS, depth
cap 6) and computes:

1. **`EffectiveClassifications`** — union of `DataClassifications` on the
   UseCase itself plus classifications on every transitively referenced
   artifact. If an agent declares a dependency on a server marked `PII` and
   the use case doesn't list `PII`, the effective set still includes it.
2. **`ComplianceFlag`** — set if the graph has violations (cycle, missing
   artifact, classification mismatch).

```mermaid
graph TB
    UC[UseCase<br/>data_classifications: ['public']] --> A[agent-research]
    A --> AC[agent-echo]
    A --> S[server-github<br/>classification: 'PII']
    S -.classification propagates.-> UC2[EffectiveClassifications:<br/>'public', 'PII']
```

Materialization is important: runtime checks read the effective set from a
single column, not via recursion.

## Statuses

| Status | Meaning |
|---|---|
| `draft` | WIP, not enforceable |
| `pending_approval` | Sent to approver, awaiting sign-off |
| `approved` | Live — runtime enforces this use case |
| `archived` | Decommissioned, kept for audit |

Only `approved` use cases affect runtime scope. Draft + pending use cases are
invisible to the runtime layer.

## Binding a key

The UseCase does nothing by itself until a key binds to it:

```bash
curl -X POST http://localhost:8080/api/admin/keys \
  -d '{
    "name": "customer-research-prod",
    "team_id": "...",
    "use_case_id": "uuid-of-customer-research-approved",
    "max_budget": 500.0,
    "budget_duration": "monthly"
  }'
```

From this point forward, every call made with this key is scoped. Unbinding
is just `PUT { "use_case_id": null }`.

## Cache

`ScopeResolver` caches the `(use_case → allowed set)` lookup for 30 seconds.
On any UseCase write (Create / Update / Delete), the cache is invalidated for
that UseCase — so changes propagate in under a second, not 30.

## Manually recompute

If you suspect the materialized fields are out of sync (e.g., after a
registry change that shifted a classification):

```bash
curl -X POST http://localhost:8080/api/admin/use-cases/{id}/recompute \
  -H "Authorization: Bearer $MASTER"
```

Re-runs the Reconciler and updates `EffectiveClassifications` and
`ComplianceFlag`. Useful after bulk-modifying artifact metadata.

## Data classifications

Values are freeform strings — whatever labels your org uses. Common defaults:

- `public` — no restrictions
- `internal` — employees only
- `confidential` — business-sensitive
- `PII` — personally identifiable
- `PHI` — protected health info
- `financial` — payment / regulated finance data

The platform doesn't interpret these — it just propagates them and exposes
them in audit queries.

## Webhooks on UseCase lifecycle

Every write fires a webhook (to any subscribed endpoint):

| Event | Fires on |
|---|---|
| `use_case.created` | POST |
| `use_case.updated` | PUT (diff is in the payload) |
| `use_case.deleted` | DELETE |
| `use_case.compliance_flag` | Reconciler sets `ComplianceFlag` |

See [Webhooks](/governance/webhooks) for payload shape and delivery
guarantees.
