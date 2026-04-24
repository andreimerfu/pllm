# MCP Gateway Overview

The Model Context Protocol (MCP) is an open spec for letting AI agents call
tools exposed by external servers. pllm's MCP gateway **aggregates any number
of MCP backends behind a single streamable HTTP endpoint** — `/v1/mcp`.

## What it does

1. **Runs backends**. Starts stdio packages (via a wrapper), connects to SSE
   endpoints, or forwards to native HTTP MCP servers.
2. **Aggregates tools** across all backends into one surface, namespaced as
   `<slug>__<tool>` to avoid collisions.
3. **Enforces ACLs**. Every key carries `AllowedMCPTools` / `BlockedMCPTools`
   glob patterns; requests are filtered before reaching the backend.
4. **Scopes to use cases**. When a key is bound to a use case, tool calls are
   additionally checked against `ScopeResolver.AllowMCPTool(useCase, server,
   tool)`.
5. **Auto-registers deployed workloads**. When pllm deploys a registry server
   to Kubernetes, the `MCPBridge` auto-registers the workload as a backend —
   its tools appear under `reg-<workload>__*` without manual wiring.

## Endpoints

| Endpoint | Use |
|---|---|
| `POST /v1/mcp` | Streamable HTTP gateway. Clients (Claude Desktop, Cursor, agent SDKs) send JSON-RPC; the gateway fans out. |
| `POST /v1/mcp/registry` | **Registry exposed as an MCP server.** Lets agents discover registry artifacts via MCP tool calls (`list_servers`, `get_agent`, etc.). See [Registry as MCP](/mcp/registry-as-mcp). |
| `GET /api/admin/mcp/servers` | List configured backends. |
| `POST /api/admin/mcp/servers` | Register a new backend (or it's done automatically on deploy). |
| `POST /api/admin/mcp/tools/invoke` | Invoke a tool by name for testing. |

## Architecture

```mermaid
graph TB
    subgraph Clients
        Claude[Claude Desktop]
        Cursor[Cursor]
        Agent[Agent pod]
    end

    subgraph pllm["pllm /v1/mcp"]
        HTTPIn[Streamable HTTP handler]
        ACL[ACL filter<br/>AllowedMCPTools / BlockedMCPTools]
        Scope[ScopeResolver<br/>AllowMCPTool]
        Manager[Manager]
        ToolCache[Per-backend tool cache]
    end

    subgraph Backends
        B1[Stdio pkg<br/>npx / uvx]
        B2[SSE server]
        B3[HTTP server]
        B4[Deployed workload<br/>auto-registered]
    end

    Claude --> HTTPIn
    Cursor --> HTTPIn
    Agent --> HTTPIn

    HTTPIn --> ACL
    ACL --> Scope
    Scope --> Manager
    Manager --> ToolCache
    Manager -.routes to.-> B1
    Manager -.routes to.-> B2
    Manager -.routes to.-> B3
    Manager -.routes to.-> B4
```

## Data model

Two tables back the gateway:

| Table | What it holds |
|---|---|
| `MCPServer` | Configured backend: transport type, command/URL, env, slug, session state |
| `MCPServerTool` | Cached tool manifest per backend (name, description, input schema) |

These are **distinct from the registry catalog** (`RegistryServer` /
`RegistryAgent` / etc.). A backend is something that is currently running and
callable; a registry artifact is a versioned spec. The two are connected by
deployment — deploying a `RegistryServer` creates a matching `MCPServer` entry.

## A typical MCP request

```mermaid
sequenceDiagram
    participant C as Client
    participant G as /v1/mcp
    participant K as Key + ACL
    participant S as ScopeResolver
    participant M as Manager
    participant B as Backend

    C->>G: tools/list (JSON-RPC)
    G->>K: Filter tools by AllowedMCPTools
    K->>S: Check use-case scope
    S->>M: GetAvailableTools()
    M-->>S: [github__search_repos, fs__read, ...]
    S-->>G: (intersected with key + use case)
    G-->>C: Tool list

    C->>G: tools/call github__search_repos
    G->>K: Key allowed? (glob match)
    K->>S: AllowMCPTool(useCase, "github", "search_repos")
    S-->>G: ALLOW
    G->>M: Route to "github" backend
    M->>B: search_repos(...)
    B-->>M: Result
    M-->>G: Result
    G-->>C: JSON-RPC response
```

## Configuration

```yaml
mcp:
  enabled: true                   # Toggle the feature
  health_check_interval: 30s      # Backend health probe cadence
  start_timeout: 30s              # Max time to boot a new backend
```

Most MCP state is **data-driven** — backends are stored in the database and
managed via the admin API or UI. The config only toggles the feature and sets
a few timing defaults.

## What's next

- [Backends & Transports](/mcp/backends) — stdio vs SSE vs HTTP, the wrapper image
- [Per-Tool ACL](/mcp/acl) — glob patterns on keys
- [Registry as MCP](/mcp/registry-as-mcp) — agents discovering registry via MCP
