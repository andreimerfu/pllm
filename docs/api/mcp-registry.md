# MCP & Registry Endpoints

These are the non-LLM public endpoints: the MCP gateway, the registry, and
the agent A2A surface.

## MCP Gateway

### `POST /v1/mcp`

Streamable-HTTP MCP endpoint. Accepts JSON-RPC 2.0 envelopes, streams
responses via chunked transfer. Session state is keyed on the `Mcp-Session-Id`
header.

```bash
# List tools across all backends
curl -X POST http://localhost:8080/v1/mcp \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'

# Invoke a tool
curl -X POST http://localhost:8080/v1/mcp \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "github__search_repos",
      "arguments": {"q": "pllm"}
    }
  }'
```

Tool names are `<slug>__<tool>`. Use `tools/list` to discover the names
available to your key.

### `POST /v1/mcp/registry`

pllm's registry exposed as an MCP server. See [Registry as MCP](/mcp/registry-as-mcp).

Tools: `list_servers`, `get_server`, `list_agents`, `get_agent`, `list_skills`,
`get_skill`, `list_prompts`, `get_prompt`, `list_refs`.

## Registry reads

All of these are authenticated but **read-only for any authed user**. Writes
require admin — see [Admin Endpoints](/api/admin).

### Servers / Agents / Skills / Prompts

Identical shape for all four kinds. Examples show `servers`; substitute as
needed.

```
GET /v1/registry/servers
GET /v1/registry/servers/{name}
GET /v1/registry/servers/{name}/versions
GET /v1/registry/servers/{name}/versions/{version}

GET /v1/registry/agents/...
GET /v1/registry/skills/...
GET /v1/registry/prompts/...
```

#### List

```bash
curl "http://localhost:8080/v1/registry/servers?search=github&limit=20" \
  -H "Authorization: Bearer $KEY"
```

Query params:

| Param | Purpose |
|---|---|
| `search` | Substring match on name, summary, description |
| `namespace` | Filter by namespace prefix (e.g. `official/`) |
| `third_party` | `true` / `false` — only third-party or only internal |
| `visibility` | `public` / `private` |
| `limit`, `offset` | Pagination (default 50) |

#### Get by name (latest)

```bash
curl http://localhost:8080/v1/registry/servers/official/github-mcp \
  -H "Authorization: Bearer $KEY"
```

#### Version history

```bash
curl http://localhost:8080/v1/registry/servers/official/github-mcp/versions
```

Returns all versions in descending order.

#### Pinned version

```bash
curl http://localhost:8080/v1/registry/servers/official/github-mcp/versions/2024.11.5
```

### Refs

```bash
curl "http://localhost:8080/v1/registry/refs?target_kind=server&target_name=github-mcp" \
  -H "Authorization: Bearer $KEY"
```

Query params:

| Param | Required | Purpose |
|---|---|---|
| `target_kind` | yes | `server` / `agent` / `skill` / `prompt` / `llm` |
| `target_name` | yes | Exact name |
| `target_version` | no | Pin (else matches across versions) |
| `owner_kind` | no | Filter by owner: `agent` or `use_case` |

Returns both `agent` and `use_case` owners that declare an edge to this
target. The **deprecation query** — who breaks if you remove this?

## Agent A2A

Every deployed agent is reachable via A2A.

### `GET /v1/a2a/{slug}/.well-known/agent.json`

Agent capability card — describes the agent's name, version, skills, and
auth scheme. Public (no auth) when `require_auth: false`; authed otherwise.

### `POST /v1/a2a/{slug}/tasks/send`

JSON-RPC 2.0:

```bash
curl -X POST http://localhost:8080/v1/a2a/agent-research/tasks/send \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tasks/send",
    "params": {
      "id": "task-abc",
      "input": {"query": "..."}
    }
  }'
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "task-abc",
    "status": "completed",
    "output": {...}
  }
}
```

The scope layer checks `AllowAgent(useCase, agentName)` before proxying.

## User profile

Endpoints users call on behalf of themselves (JWT auth, not API key):

```
GET    /v1/user/profile
PUT    /v1/user/profile
PUT    /v1/user/password

GET    /v1/user/keys
POST   /v1/user/keys
DELETE /v1/user/keys/{key_id}

GET    /v1/user/usage
GET    /v1/user/usage/daily
GET    /v1/user/usage/monthly
GET    /v1/user/budget
GET    /v1/user/teams
```

These power the **Settings** area of the admin UI. They respect per-user RBAC
— admins can fetch any user's data; users can only read their own.
