# API Keys & Permissions

Every request to pllm carries a key. Keys are the unit of authorization —
they scope the request to a user, team, budget, model allowlist, MCP tool
allowlist, registry permission set, and optionally a use case.

## The key model

```go
type Key struct {
    ID        UUID
    Key       string     // the value shown once at creation — never again
    KeyHash   string     // stored; lookups are O(1) by prefix + hash
    KeyPrefix string     // first 8 chars, shown in UI for disambiguation
    Name      string

    UserID    UUID       // key is user-owned
    TeamID    UUID       // or team-owned
    Type      string     // "api" | "virtual" | "master" | "system"

    // Scope to a use case — enables runtime enforcement
    UseCaseID UUID       // nullable

    // Model access
    AllowedModels []string   // empty = all
    BlockedModels []string

    // MCP tools
    AllowedMCPTools []string  // glob patterns on "<slug>__<tool>"
    BlockedMCPTools []string

    // Registry
    RegistryPermissions []string  // action:kind/glob patterns

    // Budget
    MaxBudget        float64
    BudgetDuration   string        // hourly | daily | weekly | monthly | ...
    TPM              int
    RPM              int
    MaxParallelCalls int

    // Flags
    ExpiresAt *time.Time
    Disabled  bool
    CreatedAt time.Time
}
```

## Key types

| Type | Purpose | Who can use |
|---|---|---|
| `master` | Bootstrap admin, always bypasses all ACLs | Platform owners only |
| `api` | Long-lived, human-managed | Developers, team keys |
| `virtual` | Short-lived, scoped (e.g., one agent deployment) | Agents, pipelines |
| `system` | Internal, pllm-minted | MCPBridge, deployment service |

Master is a single key set in `auth.master_key` (config) — there's only one.
Everything else is stored in the `Key` table.

## Creating keys

Admin API:

```bash
curl -X POST http://localhost:8080/api/admin/keys \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "team-alpha-prod",
    "team_id": "uuid-...",
    "type": "api",
    "allowed_models": ["my-gpt-4", "openrouter-claude"],
    "max_budget": 500.0,
    "budget_duration": "monthly",
    "tpm": 100000,
    "rpm": 500
  }'
```

Returns:

```json
{
  "id": "uuid-...",
  "key": "sk-pllm-abc123...xyz",        // shown once!
  "key_prefix": "sk-pllm-",
  ...
}
```

**Save the `key` immediately — it's not stored plaintext and cannot be
retrieved later.** If lost, delete and create a new one.

## User self-service

Users can manage their own personal keys (no team / use-case scoping):

```
GET    /v1/user/keys
POST   /v1/user/keys
DELETE /v1/user/keys/{key_id}
```

Team admins can additionally create keys for their team. Platform admins
(master key) can create keys for any user / team / use case.

## Registry permissions

`RegistryPermissions` is a list of `action:kind/glob` patterns.

| Pattern | Grants |
|---|---|
| `read:*` | Read any registry kind (catalog browsing) |
| `write:server` | Write any server version |
| `write:agent/pllm.io/*` | Write agents under the `pllm.io/` namespace |
| `delete:prompt/draft-*` | Delete draft prompts only |
| `publish:*` | Any publish (create or update) |

Action verbs: `read`, `write`, `publish`, `delete`, `import`, `deploy`.

Kinds: `server`, `agent`, `skill`, `prompt`, `llm`, `use_case`.

Evaluated with **deny-by-default**: a write without a matching `write:*` or
`*:*` pattern is refused.

## Example key patterns

### Full-access platform admin

```json
{
  "name": "platform-admin",
  "type": "api",
  "registry_permissions": ["*:*"]
}
```

### Developer who can publish only their namespace

```json
{
  "name": "alice-dev",
  "registry_permissions": [
    "read:*",
    "write:agent/alice/*",
    "publish:server/alice/*"
  ]
}
```

### Use-case-scoped agent runtime key (typical)

```json
{
  "name": "agent-research-prod-key",
  "type": "virtual",
  "use_case_id": "uuid-...",
  "allowed_models": [],        // inherited via scope check
  "allowed_mcp_tools": [],     // inherited via scope check
  "max_budget": 50.0,
  "budget_duration": "daily"
}
```

When `use_case_id` is set, the allowlists are **additional** filters on top
of the use case's declared refs. Leaving them empty means "use what the use
case allows, no more."

## Key rotation

Keys don't auto-rotate. Two rotation patterns:

1. **Replace-then-delete.** Create the new key, update the consumer, delete
   the old key. No downtime.
2. **Dual-trust window.** Create the new key, keep both active for a
   transition window, rotate, delete the old one.

For production, set `expires_at` on every key and alert in the 7 days before
expiry.

## Key revocation

```bash
curl -X DELETE http://localhost:8080/api/admin/keys/{id} \
  -H "Authorization: Bearer $MASTER"
```

Revocation is immediate. In-flight requests with the key complete; new ones
fail with 401.

## Security notes

- Keys are hashed with SHA-256 before storage
- The first 8 chars (`KeyPrefix`) are stored plaintext so the UI can show a
  recognizable disambiguator ("sk-pllm-abc…")
- Auth lookups are O(1) by prefix
- Brute-force protection: rate limiting by source IP applies to auth failures
- Master key should live only in a secret store (Vault, K8s Secret). Never
  commit to git.
