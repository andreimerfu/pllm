# A2A Interoperability

**A2A** is an emerging open standard for agent-to-agent communication —
JSON-RPC over HTTP with a machine-readable capability card. pllm exposes every
deployed agent over A2A so they can call each other (or be called by external
orchestrators) without bespoke glue.

## The endpoints

Every deployed agent gets two endpoints under `/v1/a2a/<slug>/`:

| Endpoint | Purpose |
|---|---|
| `GET /.well-known/agent.json` | Agent capability card |
| `POST /tasks/send` | JSON-RPC `tasks/send` — the invoke path |

Where `<slug>` is the deployment's slug (derived from the registry name).

## The agent card

```json
{
  "name": "agent-research",
  "description": "Research agent — searches and summarizes",
  "url": "https://pllm.example.com/v1/a2a/agent-research",
  "version": "0.1.0",
  "capabilities": {
    "streaming": false,
    "pushNotifications": false
  },
  "authentication": {
    "schemes": ["bearer"]
  },
  "skills": [
    {
      "id": "invoke",
      "name": "invoke",
      "description": "Free-form task invocation"
    }
  ]
}
```

Other agents (or orchestrators) `GET` the card to discover what this agent
offers. The card is rendered from the `RegistryAgent` metadata and the
deployment's URL.

## Calling an agent

```bash
curl -X POST http://localhost:8080/v1/a2a/agent-research/tasks/send \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tasks/send",
    "params": {
      "id": "task-abc",
      "input": { "query": "What is pllm?" },
      "metadata": {}
    }
  }'
```

pllm:

1. Authenticates the caller's key
2. Checks use-case scope (`AllowAgent(useCase, agentName)`)
3. Proxies to the pod's `/invoke` with `{"input": ..., "task_id": "task-abc"}`
4. Wraps the response in the A2A envelope:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "task-abc",
    "status": "completed",
    "output": { ...agent's output... }
  }
}
```

## Agent → agent calls

The most interesting case: an agent calls another agent. The agent's pod has
`PLLM_API_KEY` mounted — it uses that to call A2A just like a human would.

```python
import os, requests

PLLM_BASE_URL = os.environ["PLLM_BASE_URL"]
PLLM_API_KEY  = os.environ["PLLM_API_KEY"]

resp = requests.post(
    f"{PLLM_BASE_URL}/v1/a2a/agent-echo/tasks/send",
    headers={"Authorization": f"Bearer {PLLM_API_KEY}"},
    json={
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tasks/send",
        "params": {"id": "sub-task-1", "input": {"text": "hello"}},
    },
)
```

The platform automatically checks:

1. Does the caller's use case permit `agent-echo`? (ref with
   `target_kind="agent", target_name="pllm.io/agent-echo"`)
2. Does the agent-echo deployment exist and is it healthy?

If either is false → 403 → `ScopeEvent{kind: "agent", outcome: "denied"}`.

## Scope on A2A

`ScopeResolver.AllowAgent(useCaseID, agentName)` is the check. It accepts:

- Full registry name (`pllm.io/agent-echo`)
- Last path segment (`agent-echo`) — matches the slug
- `LocalName` alias on the ref

A use case that allows "agent A to call agent B" needs **both** agent refs in
its graph — one is not enough. This is by design: declaring B in the use case
makes the approval visible; relying on transitive refs on A would hide it.

## What A2A doesn't cover (yet)

- **Streaming** — agents return one response at a time. Streaming / SSE is in
  the A2A spec but not wired in pllm yet.
- **Push notifications** — A2A allows agents to push task updates to a
  webhook; pllm doesn't implement this.
- **Long-running tasks** — the invoke is synchronous; if your agent takes
  minutes, wrap it in a task queue yourself and return a task ID.

These are on the roadmap as A2A matures.

## Testing A2A outside pllm

External clients (other platforms, CI jobs, LangChain orchestrators) can call
pllm's A2A endpoints just like internal agents, provided they have an API key:

```bash
KEY=<a-key-bound-to-the-right-use-case>

curl http://pllm.example.com/v1/a2a/agent-research/.well-known/agent.json
curl -X POST http://pllm.example.com/v1/a2a/agent-research/tasks/send \
  -H "Authorization: Bearer $KEY" \
  -d '...'
```

This is how you integrate pllm-deployed agents into an external orchestrator
without giving the orchestrator access to the cluster.
