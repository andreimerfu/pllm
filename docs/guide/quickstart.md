# Quick Start

Get pllm running with the full stack in about two minutes.

## Docker Compose (recommended)

The repo's `docker-compose.yml` brings up pllm, Postgres, Redis, Dex, Grafana,
Prometheus, and Jaeger.

```bash
git clone https://github.com/andreimerfu/pllm.git
cd pllm

cp .env.example .env
# Edit .env — set at least OPENAI_API_KEY and PLLM_MASTER_KEY

docker-compose up -d
```

Accessible at:

| Service | URL | Login |
|---|---|---|
| API | `http://localhost:8080/v1` | Bearer $PLLM_MASTER_KEY |
| Admin UI | `http://localhost:8080/ui` | `admin / changeme123!` |
| Docs | `http://localhost:8080/docs` | — |
| Grafana | `http://localhost:3001` | `admin / admin` |
| Prometheus | `http://localhost:9091` | — |
| Jaeger | `http://localhost:16686` | — |

## First request

OpenAI-compatible:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

You should get a standard OpenAI response back. Streaming works with
`"stream": true`.

## What's next

Three common paths:

### Path A — just use the LLM gateway

You're happy with pllm as an OpenAI-compatible proxy. Configure providers and
models, set up teams and keys, move on.

- [Providers](/providers)
- [Routing](/guide/routing)
- [API Keys](/security/keys)

### Path B — add MCP servers

You want to give agents tool access through a single gateway.

```bash
# Register a GitHub MCP server
curl -X POST http://localhost:8080/api/admin/mcp/servers \
  -H "Authorization: Bearer $PLLM_MASTER_KEY" \
  -d '{
    "name": "github",
    "slug": "github",
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "..."}
  }'

# Point Claude Desktop / Cursor at http://localhost:8080/v1/mcp
```

- [MCP Gateway](/mcp/overview)
- [Backends & Transports](/mcp/backends)

### Path C — publish agents with governance

The full picture: curated registry, use-case approvals, one-click deploys,
scope enforcement, audit trails.

- [Concepts](/guide/concepts) — read first
- [Agent onboarding walkthrough](/agents/onboarding) — 5-minute demo

## Build from source

For Go development:

```bash
make dev      # hot reload via `air`
make build    # full binary (embeds UI + docs)
make test
```

Prereqs: Go 1.23+, Node 20+ (for the UI), Docker (for testcontainer-based
tests).

## Lite mode

Run without Postgres / Redis — LLM gateway only, in-memory rate limit and
cache:

```bash
# Don't set DATABASE_URL or REDIS_URL
OPENAI_API_KEY=sk-... go run cmd/server/main.go
```

Useful for demos and side-by-side benchmarking. Auth, registry, deployments,
and governance are disabled. See [Concepts — modes of operation](/guide/concepts#modes-of-operation).
