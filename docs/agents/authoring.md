# Authoring & Publishing Agents

This page walks you through writing a new agent from scratch and publishing it
to pllm's registry.

## Step 1 — implement `/invoke`

pllm doesn't care what language or framework you use. The only requirement is
an HTTP server that answers `POST /invoke`.

Minimal Python (Flask):

```python
import os, requests
from flask import Flask, request, jsonify

app = Flask(__name__)

PLLM_BASE_URL = os.environ["PLLM_BASE_URL"]     # injected by pllm
PLLM_API_KEY = os.environ["PLLM_API_KEY"]       # injected by pllm (use-case scoped)

@app.post("/invoke")
def invoke():
    body = request.get_json(force=True)
    user_input = body["input"]
    task_id = body["task_id"]

    # Call the LLM gateway — scope is enforced automatically
    resp = requests.post(
        f"{PLLM_BASE_URL}/v1/chat/completions",
        headers={"Authorization": f"Bearer {PLLM_API_KEY}"},
        json={
            "model": "my-gpt-4",                # must match a ref on your agent
            "messages": [{"role": "user", "content": str(user_input)}],
        },
    )

    if resp.status_code == 403:
        return jsonify({"output": None, "error": "scope denied"}), 200

    return jsonify({"output": resp.json()["choices"][0]["message"]["content"]}), 200

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
```

Notes:

- `PLLM_BASE_URL` is the **internal cluster URL** pllm injects — typically
  `http://pllm.pllm.svc.cluster.local:8080`. Never hardcode the external URL.
- `PLLM_API_KEY` is the use-case-scoped key pllm mints at deploy time. It's
  bound to the use case the operator picks.
- The model name (`my-gpt-4`) must be in the agent's `refs` AND in the use
  case's `refs`, or the call gets denied.

## Step 2 — containerize it

```dockerfile
# Dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY app.py .
EXPOSE 8080
CMD ["python", "app.py"]
```

Build:

```bash
# For minikube dev
docker build -t agent-research:0.1.0 .
minikube image load agent-research:0.1.0

# For a real registry
docker build -t ghcr.io/example/agent-research:0.1.0 .
docker push ghcr.io/example/agent-research:0.1.0
```

## Step 3 — publish to the registry

```bash
curl -X POST http://localhost:8080/api/admin/registry/agents \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pllm.io/agent-research",
    "version": "0.1.0",
    "summary": "Research agent — searches and summarizes",
    "package": {
      "type": "oci",
      "identifier": "ghcr.io/example/agent-research:0.1.0"
    },
    "refs": [
      {
        "target_kind": "llm",
        "target_name": "my-gpt-4",
        "local_name": "default_model"
      }
    ]
  }'
```

You get back the created artifact with `is_latest: true`.

## Step 4 — create/update a use case that approves it

```bash
curl -X PUT http://localhost:8080/api/admin/use-cases/<id> \
  -H "Authorization: Bearer $MASTER" \
  -d '{
    "refs": [
      {
        "target_kind": "agent",
        "target_name": "pllm.io/agent-research",
        "target_version": "0.1.0"
      },
      {
        "target_kind": "llm",
        "target_name": "my-gpt-4"
      }
    ]
  }'
```

The use case must declare **both** the agent and any resources the agent
calls — the Reconciler walks transitively, but explicit declarations are
easier to audit.

## Step 5 — deploy

Via UI (**Agents** page → **Deploy agent**) or API:

```bash
curl -X POST http://localhost:8080/api/admin/agents \
  -H "Authorization: Bearer $MASTER" \
  -d '{
    "registry_name": "pllm.io/agent-research",
    "registry_version": "0.1.0",
    "use_case_id": "<use-case-uuid>",
    "replicas": 1
  }'
```

pllm:

1. Mints a fresh API key with `use_case_id=<...>`
2. Renders a Deployment, Service, and Secret
3. Applies them to the configured `agent_namespace`
4. Watches for pod readiness
5. Registers the agent under `/v1/a2a/<slug>/`

Returns the `AgentDeployment` row. Status progresses `pending` → `deploying` →
`running`.

## Step 6 — invoke

```bash
curl -X POST http://localhost:8080/v1/a2a/agent-research/tasks/send \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tasks/send",
    "params": { "input": { "query": "What is pllm?" } },
    "id": 1
  }'
```

Or from the UI's **Agents** page → card **⋯** menu → **Invoke**.

## Iteration

Rebuild the image → push → `make agent-<name>-image-minikube` (if minikube)
→ publish a new version to the registry → update the deployment to the new
version (undeploy + deploy, or — roadmap — in-place version bump).

## Testing scope locally

Add a ref to your agent for a model the use case **doesn't** approve:

```json
"refs": [
  { "target_kind": "llm", "target_name": "my-gpt-4" },
  { "target_kind": "llm", "target_name": "test-model-restricted" }
]
```

Have the agent's code try both models. The second call will return 403. Open
**Compliance** in the UI — within ~5 seconds you'll see a fresh
`scope.denied` row.

That's the whole point of the product: **you didn't write any scope logic in
your agent**, yet the platform mediated the call and produced an audit event.
