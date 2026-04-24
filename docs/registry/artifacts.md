# Artifacts & Versioning

All four artifact kinds share the same shape. This page covers the common
fields, kind-specific extensions, and lifecycle semantics.

## Common fields

```go
type RegistryArtifact struct {
    Name        string    // e.g. "pllm.io/agent-echo"
    Version     string    // SemVer preferred: "0.1.0", "1.0.0-rc.1"
    IsLatest    bool      // Exactly one row per name has this set

    Summary     string    // One-line description
    Metadata    JSON      // Kind-specific blob (see below)
    Visibility  string    // "public" | "private"
    TeamID      UUID      // Owning team (for RBAC on writes)

    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

## `RegistryServer`

```json
{
  "kind": "server",
  "name": "official/github-mcp",
  "version": "2024.11.5",
  "summary": "GitHub MCP server — repos, issues, PRs",
  "metadata": {
    "description": "Full GitHub API surface via MCP...",
    "homepage": "https://github.com/modelcontextprotocol/servers/tree/main/src/github",
    "tools_preview": ["search_repos", "get_pull_request", ...]
  },
  "package": {
    "type": "npm",
    "identifier": "@modelcontextprotocol/server-github",
    "runtime": "npx"
  },
  "transport": "stdio",
  "third_party": true,
  "enrichment_score": 82
}
```

**Runtime values:** `npx`, `uvx`, `docker`, `oci`, `http`.

- `npx` / `uvx` — stdio package; pllm wraps it with the wrapper image on deploy
- `docker` / `oci` — pre-packaged image; pllm deploys directly
- `http` — remote service; no deployment needed, just a URL

**`third_party: true`** marks servers that haven't been authored internally.
Used by the (roadmap) Third-Party Lens page to filter dependencies by origin.

## `RegistryAgent`

Agents add `refs` — the edges that declare dependencies.

```json
{
  "kind": "agent",
  "name": "pllm.io/agent-research",
  "version": "0.1.0",
  "summary": "Research agent — hits a search MCP + summarizes with an LLM",
  "metadata": {
    "description": "...",
    "contract": "POST /invoke {\"input\": ..., \"task_id\": ...}"
  },
  "package": {
    "type": "oci",
    "identifier": "registry.example.com/pllm/agent-research:0.1.0"
  },
  "refs": [
    {
      "target_kind": "server",
      "target_name": "official/github-mcp",
      "target_version": "",
      "local_name": "github",
      "required": true
    },
    {
      "target_kind": "llm",
      "target_name": "my-gpt-4",
      "local_name": "default_model"
    }
  ]
}
```

`AgentWithRefs` is the service helper that buckets these by target kind so the
UI can render **Servers / Skills / Prompts / LLMs** tabs.

## `RegistrySkill`

```json
{
  "kind": "skill",
  "name": "pllm.io/skill-summarize",
  "version": "1.0.0",
  "metadata": {
    "description": "Summarize a document",
    "input_schema": { "type": "object", "properties": { "text": { "type": "string" } } },
    "template": "Summarize the following in 3 bullet points:\n{{text}}"
  }
}
```

## `RegistryPrompt`

Lighter than skills — just a named, versioned prompt string.

```json
{
  "kind": "prompt",
  "name": "pllm.io/prompt-triage",
  "version": "0.3.0",
  "metadata": {
    "text": "Classify this ticket as BUG, FEATURE, or QUESTION: {{ticket}}"
  }
}
```

## Publishing

```bash
curl -X POST http://localhost:8080/api/admin/registry/servers \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "official/github-mcp",
    "version": "2024.11.5",
    "summary": "GitHub MCP server",
    "transport": "stdio",
    "package": {
      "type": "npm",
      "identifier": "@modelcontextprotocol/server-github",
      "runtime": "npx"
    },
    "metadata": {
      "description": "..."
    }
  }'
```

Returns the artifact with `is_latest: true` and triggers enrichment.

## Delete / deprecate

There's no "deprecate" flag yet; deletion is hard delete by version:

```bash
curl -X DELETE \
  "http://localhost:8080/api/admin/registry/servers/official/github-mcp/versions/2024.11.5" \
  -H "Authorization: Bearer $MASTER"
```

Before deleting, check cross-refs:

```bash
curl "http://localhost:8080/v1/registry/refs?target_kind=server&target_name=official/github-mcp"
```

If any agent or use case still references it, you'll want to update those first
(or accept that their calls will start failing).

## `is_latest` bookkeeping

Writing a new version atomically:

1. Insert the new row with `is_latest = true`
2. Update the previous latest row, setting `is_latest = false`

Both happen in one transaction. Readers that query without a version always
get `is_latest`; readers that specify a version get exactly that version or a
404.

## Visibility

| Visibility | Who can read | Who can write |
|---|---|---|
| `public` | Any authenticated user | Team admins or pllm admin |
| `private` | Members of `TeamID` | Team admins of `TeamID` |

Visibility is **per-version** — you can keep `v0.0.x` private and promote
`v1.0.0` to public once it's been reviewed.
