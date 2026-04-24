# API Reference

pllm exposes every capability over HTTP. The surface splits into three
families:

| Family | Prefix | Auth | Purpose |
|---|---|---|---|
| **Public** | `/v1/*` | API key (Bearer) | Normal app traffic: LLM calls, MCP calls, registry reads |
| **Admin** | `/api/admin/*` | Master key or admin JWT | Platform ops: CRUD on keys, teams, registry, deployments, use cases |
| **Infra** | `/health`, `/ready`, `/metrics` | None | Ops probes and Prometheus scrape |

A mirror of the public LLM routes exists under `/api/v1/*` — same handlers,
same auth, provided for tools that assume "API v1" in the path.

## Quick map

| What you want to do | Go here |
|---|---|
| Chat completions, streaming, Anthropic Messages | [OpenAI-Compatible](/api/openai), [Anthropic Messages](/api/anthropic) |
| List models | `GET /v1/models` |
| Invoke an MCP tool | `POST /v1/mcp` (JSON-RPC) |
| Browse the registry | `GET /v1/registry/{servers,agents,skills,prompts}` |
| Publish a registry artifact | [Admin Endpoints](/api/admin) |
| Deploy a server or agent | [Admin Endpoints](/api/admin) |
| Create / approve use cases | [Admin Endpoints](/api/admin) |
| Subscribe to webhooks | [Admin Endpoints](/api/admin) |
| Call a deployed agent | `POST /v1/a2a/{slug}/tasks/send` |

## Auth header

Every authenticated request:

```
Authorization: Bearer <key>
```

or equivalently:

```
X-API-Key: <key>
```

Where `<key>` is a master key, API key, or JWT bearer token (after login).

## Common response shapes

### Success

```json
{
  "data": ...,
  "meta": { "page": 1, "total": 100 }
}
```

Chat / embeddings / OpenAI-compatible endpoints return their native shapes
(no `data` wrapper) for SDK compatibility.

### Error

```json
{
  "error": {
    "message": "Model 'foo' not found",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

### Error types

| Type | HTTP |
|---|---|
| `invalid_request_error` | 400 |
| `authentication_error` | 401 |
| `permission_error` | 403 |
| `not_found` | 404 |
| `rate_limit_error` | 429 |
| `scope_denied` | 403 (from the scope layer) |
| `budget_exceeded` | 429 |
| `guardrail_error` | 400 or 502 |
| `server_error` | 500 |
| `service_unavailable` | 503 |

## Streaming

SSE-style streaming for LLM endpoints:

```
Content-Type: text/event-stream

data: {"id":"chatcmpl-...", ...}

data: {"id":"chatcmpl-...", ...}

data: [DONE]
```

The gateway flushes immediately — there's no buffering layer between the
provider and the client. See [OpenAI-Compatible](/api/openai#streaming) for
the exact chunk shape.

## What's next

- [OpenAI-Compatible](/api/openai) — `/v1/chat/completions`, embeddings,
  audio, images, realtime
- [Anthropic Messages](/api/anthropic) — `/v1/messages`
- [MCP & Registry](/api/mcp-registry) — `/v1/mcp`, `/v1/registry/*`, `/v1/a2a/*`
- [Admin Endpoints](/api/admin) — everything under `/api/admin/*`
