# mcp-wrapper

Tiny bridge image used by the pllm Deployment feature for npm / PyPI-based
registry entries that only ship a stdio MCP server.

The K8sAdapter sets these env vars on each wrapper-based Deployment:

| Var | Meaning |
|---|---|
| `PLLM_WRAPPER_KIND`    | `npx` or `uvx` |
| `PLLM_WRAPPER_PACKAGE` | e.g. `@modelcontextprotocol/server-everything` |
| `PLLM_WRAPPER_VERSION` | e.g. `0.6.2` (optional) |
| `PLLM_WRAPPER_ARGS`    | extra CLI args, space-separated (optional) |

`bridge.mjs`:
- spawns the stdio MCP server as a child process,
- exposes a POST `/mcp` endpoint on port `8000` that speaks the
  MCP streamable-HTTP transport (one JSON-RPC message per request),
- multiplexes many HTTP requests over the single stdio channel
  by matching `jsonrpc.id`.

## Build

```bash
docker build -t pllm/mcp-wrapper:dev deploy/mcp-wrapper
```

## Load into minikube

```bash
minikube image load pllm/mcp-wrapper:dev
```

## Security notes

- Runs as non-root (UID 10001) to match the pod `SecurityContext`
  pllm applies.
- Expects a writable `/tmp` (the K8sAdapter mounts an `emptyDir`) and
  `HOME=/tmp` so `npm` / `uv` caches have somewhere to live when the
  root filesystem is read-only.
