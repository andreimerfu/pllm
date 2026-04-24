# Research reference agent

Second reference agent for the pllm demo. Unlike the echo agent (which just
echoes input), this one **calls pllm's own `/v1/chat/completions`** to prove
the scope-enforcement story end-to-end.

## What it does

On every `POST /invoke`, it probes two LLM model names:

- `test-model-allowed` — the demo UseCase declares this as a ref, so the
  scope layer passes the call through.
- `test-model-restricted` — intentionally NOT declared. The scope layer
  returns 403 and fires a `scope.denied` event that shows up in the
  Compliance audit log.

The response summarizes both probe results. The demo presenter hits Invoke
in the UI, then points to **Compliance → Scope denials** and shows a
freshly-landed audit row for the restricted model.

Note: the actual LLM providers don't need to be configured — the demo only
relies on whether pllm's scope layer returns 403 or anything else. The
routing layer will reject both calls with its own error once scope passes,
and that's fine.

## Contract

- `POST /invoke` with `{ "input": <any>, "task_id": "<string>" }` —
  probes the two models and returns `{ output: <summary>, task_id: ... }`.
- `GET /healthz` — returns 200.

## Env vars (injected by pllm at deploy time)

| Var | Used for |
| --- | --- |
| `PLLM_BASE_URL` | Base URL for pllm's OpenAI-compatible endpoints |
| `PLLM_API_KEY` | Scoped key bound to the agent's UseCase |
| `PLLM_USE_CASE` | Slug of the bound UseCase (shown in the report) |
| `PLLM_AGENT_NAME` | Registry agent's resource_name (shown in the report) |

## Build + load into minikube

```sh
make agent-research-image-minikube
```

## Sanity-check locally (without a cluster)

```sh
make agent-research-image
docker run --rm -p 8000:8000 \
  -e PLLM_BASE_URL=http://host.docker.internal:8080 \
  -e PLLM_API_KEY=sk-master-test \
  pllm/agent-research:dev
curl -sS -X POST http://localhost:8000/invoke \
  -H 'Content-Type: application/json' \
  -d '{"input":"summarize","task_id":"t-1"}'
```
