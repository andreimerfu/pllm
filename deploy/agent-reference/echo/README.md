# Echo reference agent

Trivial Flask app that satisfies the pllm Phase 2 pod contract. Used by the
demo to prove the deploy → invoke loop end-to-end without any external
dependencies.

## Contract

- `POST /invoke` with `{ "input": <any>, "task_id": "<string>" }` — echoes the
  input back alongside the bound use case and agent name that pllm injects as
  env vars at deploy time.
- `GET /healthz` — returns 200.

## Build + load into minikube

```sh
make agent-echo-image-minikube
```

This builds `pllm/agent-echo:dev` and loads it into minikube's image store so
pllm's AgentK8sAdapter can pull it with `ImagePullPolicy: IfNotPresent`.

## Sanity-check locally (without a cluster)

```sh
make agent-echo-image
docker run --rm -p 8000:8000 pllm/agent-echo:dev
curl -sS -X POST http://localhost:8000/invoke \
  -H 'Content-Type: application/json' \
  -d '{"input":"hello","task_id":"t-1"}'
```

## How pllm actually runs this

pllm's AgentK8sAdapter applies a Deployment + Service referencing the image
above. Env vars injected into the pod:

| Var | Value |
| --- | --- |
| `PLLM_BASE_URL` | `http://pllm.pllm.svc.cluster.local:8080` |
| `PLLM_MCP_URL`  | `http://pllm.pllm.svc.cluster.local:8080/v1/mcp` |
| `PLLM_USE_CASE` | slug of the bound UseCase |
| `PLLM_AGENT_NAME` | registry agent's `resource_name` |
| `PLLM_API_KEY` | scoped key bound to the UseCase (mounted via `envFrom` Secret) |

Echo doesn't use any of these for its behaviour — it just echoes them so you
can visually confirm the platform wired everything correctly.
