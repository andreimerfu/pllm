# K8s Adapter

The `K8sAdapter` is the component that turns a `RegistryServer` or
`RegistryAgent` into Kubernetes objects. It uses `client-go` in-cluster.

## What gets rendered

For a typical MCP server deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pllm-mcp-github-2024-11-5
  namespace: pllm-mcp
  labels:
    app.kubernetes.io/managed-by: pllm
    pllm.io/registry-kind: server
    pllm.io/registry-name: official__github-mcp
    pllm.io/registry-version: 2024.11.5
    pllm.io/deployment-id: <uuid>
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: pllm-mcp-github-2024-11-5
  template:
    spec:
      containers:
        - name: wrapper
          image: pllm/mcp-wrapper:dev                    # from config
          env:
            - name: PLLM_WRAPPED_CMD
              value: "npx"
            - name: PLLM_WRAPPED_ARGS
              value: "-y @modelcontextprotocol/server-github"
            - name: GITHUB_PERSONAL_ACCESS_TOKEN
              valueFrom:
                secretKeyRef: { name: ..., key: GITHUB_TOKEN }
            - name: PLLM_API_KEY
              valueFrom:
                secretKeyRef: { name: ..., key: PLLM_API_KEY }     # if use_case bound
          ports:
            - containerPort: 8080
          livenessProbe:
            httpGet: { path: /health, port: 8080 }
          resources:
            requests: { cpu: 100m, memory: 256Mi }
            limits:   { cpu: 500m, memory: 512Mi }
---
apiVersion: v1
kind: Service
metadata:
  name: pllm-mcp-github-2024-11-5
  namespace: pllm-mcp
spec:
  selector:
    app.kubernetes.io/name: pllm-mcp-github-2024-11-5
  ports:
    - port: 8080
      targetPort: 8080
```

For an OCI-packaged artifact (`runtime: "oci"`), the container image is the
artifact's own image — no wrapper — and the container args are whatever the
image expects.

For an agent, the same shape + the pod exposes `POST /invoke`.

## Labels

Every object gets:

- `app.kubernetes.io/managed-by: pllm`
- `pllm.io/registry-kind: server | agent | skill | prompt`
- `pllm.io/registry-name: <slug-safe-name>`
- `pllm.io/registry-version: <version>`
- `pllm.io/deployment-id: <uuid>`

Labels are how pllm finds workloads during status refresh, undeploy, and
reconciliation. Don't remove them.

## RBAC

pllm needs a ServiceAccount with Deployment / Service / Secret / NetworkPolicy
permissions in the target namespaces. The manifests are in
[`deploy/k8s/rbac.yaml`](https://github.com/andreimerfu/pllm/blob/main/deploy/k8s/rbac.yaml).

Essentials:

```yaml
rules:
  - apiGroups: [""]
    resources: ["services", "secrets", "events", "pods", "pods/log"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["networkpolicies"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

Scope the RoleBinding to the deployment namespaces (`pllm-mcp`, `pllm-agents`)
rather than cluster-wide.

## Status refresh

The deployment status in the database lags Kubernetes by a few seconds. On
read, pllm optionally refreshes:

- **Manual**: `POST /api/admin/deployments/{id}/status` forces a K8s query and
  updates the row.
- **UI auto-refresh**: the Deployments page polls every 5 s.
- **Watch** (roadmap): a long-running informer that pushes changes into the
  database. For now, pull-based works fine.

## Events feed

Every operational action is written to the `DeploymentEvent` table:

```json
{
  "deployment_id": "...",
  "kind": "status_changed | oom_killed | oom_bumped | undeployed | failed",
  "outcome": "success | failure",
  "detail": "freeform string",
  "at": "..."
}
```

Queryable at `GET /api/admin/deployments/events`. Useful for the
"what happened to my deployment last night?" question without spelunking
through cluster events.

## In-cluster vs local dev

```yaml
deployment:
  kubernetes:
    in_cluster: true           # prod: use the pod's service account
```

For local dev with `skaffold`:

```yaml
deployment:
  kubernetes:
    in_cluster: false
    kubeconfig_path: /home/user/.kube/config
```

The minikube workflow is:

```bash
make wrapper-image-minikube   # build + load wrapper into minikube's daemon
skaffold dev                  # live rebuild + redeploy pllm itself
```

The `make wrapper-image-minikube` step is important — minikube's image cache
doesn't see locally-built images unless you load them explicitly.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `ImagePullBackOff` for an npx/uvx server | Rebuild wrapper: `make wrapper-image-minikube` |
| `ImagePullBackOff` for an agent | Rebuild agent image into minikube: `make agent-<name>-image-minikube` |
| Deployment stuck `pending` | Check pllm logs — RBAC denied, or namespace doesn't exist |
| Pod running but status `unhealthy` | Check wrapper logs — stdio cmd probably crashed |
| Undeploy leaves orphaned objects | Query by label: `kubectl get all,networkpolicy,secret -l pllm.io/deployment-id=<uuid> -A` |
