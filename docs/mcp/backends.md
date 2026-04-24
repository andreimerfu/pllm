# Backends & Transports

An MCP backend is a running process (or remote service) that implements the
MCP protocol. pllm supports three transports — and adds one more by wrapping
stdio packages into HTTP.

## Transport types

### stdio

The original MCP transport. A process reads JSON-RPC from stdin and writes to
stdout. Most published MCP servers — `@modelcontextprotocol/server-filesystem`,
`@modelcontextprotocol/server-github`, everything on npm and PyPI — are stdio.

**Problem:** you can't expose stdin/stdout through an HTTP gateway.

**Solution:** the `pllm/mcp-wrapper` image. It runs the stdio process in a
container, exposes a local HTTP port that speaks MCP, and the gateway talks to
it over HTTP. You configure the wrapper once in
`deployment.kubernetes.wrapper_image`; deployments pull it automatically when
the artifact's runtime is `npx` or `uvx`.

Build the wrapper for local minikube work:

```bash
make wrapper-image-minikube
```

### SSE (Server-Sent Events)

Long-lived MCP servers that run as standalone services expose SSE endpoints.
The gateway keeps one connection open per backend and demuxes responses.

```json
{
  "transport": "sse",
  "url": "https://mcp.example.com/sse"
}
```

### HTTP (streamable)

Native MCP-over-HTTP servers. Every request is a POST with a JSON-RPC envelope;
streaming responses use chunked transfer encoding.

```json
{
  "transport": "http",
  "url": "https://mcp.example.com/v1/mcp"
}
```

## Registering a backend

### Via admin API

```bash
curl -X POST http://localhost:8080/api/admin/mcp/servers \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "slug": "github",
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": {
      "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_..."
    }
  }'
```

### Via the admin UI

**Settings → MCP Servers → Add Backend**. Pick transport, fill in the details,
hit save. Health check will probe within `mcp.health_check_interval`.

### Automatically on deploy

Deploying a `RegistryServer` through the Deployments page (or admin API) creates
a matching `MCPServer` row via the `MCPBridge`. The backend appears under the
slug `reg-<deployment-name>` — tools become `reg-<deployment-name>__<tool>`.

## Session management

MCP is stateful by design. The gateway tracks a **session per client** over the
`/v1/mcp` endpoint:

- Client opens a stream (persistent connection or long-poll)
- Gateway holds per-backend cursors (tool subscriptions, log subscriptions, etc.)
- Tool calls route through the same session to the right backend

**Scaling note:** the gateway currently uses in-memory session state. When
running pllm with multiple replicas, the load balancer must pin each client
session to the same pod (sticky sessions by `Mcp-Session-Id` header). Session
pinning across replicas is on the roadmap — see
[Known Gaps](/#known-gaps-next-steps).

## Health & readiness

Each backend has a health state updated by:

1. Startup probe — the backend must return a valid `tools/list` within
   `mcp.start_timeout` or it's marked unhealthy.
2. Periodic health check — every `mcp.health_check_interval`, the gateway calls
   `tools/list` or `ping` and resets the health tracker.
3. Request failures — transport errors or timeouts trip the backend's circuit
   breaker.

The admin UI shows a live status pill per backend:

| Status | Meaning |
|---|---|
| `running` | Healthy, serving tools |
| `starting` | Booting, pre-first-probe |
| `unhealthy` | Last probe failed; gateway will retry |
| `stopped` | Manually disabled |

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Backend stuck in `starting` | Wrapper image can't pull, or npx package doesn't exist | Check admin UI → backend → logs; verify `deployment.kubernetes.wrapper_image` is loaded into the cluster |
| Tools not showing up in `/v1/mcp` | Key's `AllowedMCPTools` doesn't match | Check key permissions; try with master key first |
| Scope denial with correct tool name | ScopeResolver sees a different slug than what your use case declares | Check logs — `ScopeResolver.AllowMCPTool` accepts the full registry name, the last path segment, OR the `LocalName` alias. Prefer the slug the deployment registered under. |
