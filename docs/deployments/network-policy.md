# Network Policy

Third-party MCP servers often have alarming network reach — the average npm
MCP server can connect to anything on the internet. Turn on `restrict_egress`
to generate a `NetworkPolicy` that locks pod egress down to DNS + internal
endpoints only.

## Enabling it

Globally in config:

```yaml
deployment:
  kubernetes:
    restrict_egress: true
```

Or per-deployment:

```bash
curl -X POST http://localhost:8080/api/admin/deployments \
  -d '{
    "registry_name": "untrusted-server",
    "registry_version": "0.1.0",
    "restrict_egress": true
  }'
```

## What the policy looks like

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pllm-mcp-<slug>-egress
  namespace: pllm-mcp
spec:
  podSelector:
    matchLabels:
      pllm.io/deployment-id: <uuid>
  policyTypes:
    - Egress
  egress:
    # DNS to kube-dns
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53

    # Back to pllm itself (so the pod can make LLM/MCP calls through the gateway)
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: pllm
      ports:
        - protocol: TCP
          port: 8080
```

In words: **DNS and pllm — nothing else**. No direct OpenAI API calls, no
GitHub, no PyPI. Anything the server wants to reach has to come through the
gateway, where ACLs + scope checks apply.

## What this breaks

Many MCP servers expect direct outbound. You'll see:

- `brave-search-mcp` trying to hit `api.search.brave.com` — fails
- `github-mcp` trying to hit `api.github.com` — fails
- `openai-mcp` calling OpenAI directly — fails

Three ways to handle this:

### Option 1 — allow specific destinations

Add a supplementary NetworkPolicy allowing the server's outbound:

```yaml
egress:
  - to:
      - ipBlock:
          cidr: 140.82.112.0/20   # GitHub's range (example)
    ports:
      - protocol: TCP
        port: 443
```

Maintain a list of approved CIDRs. Tedious but auditable.

### Option 2 — reverse-proxy through pllm

If the remote API has an OpenAI-compatible shape (e.g., OpenRouter), configure
it as a provider in pllm and have the server call pllm instead. Traffic stays
inside the cluster.

### Option 3 — accept the tradeoff

For high-trust internal servers, don't enable `restrict_egress`. Scope
enforcement still happens at the pllm gateway; NetworkPolicy is defense in
depth, not the primary control.

## Interaction with cluster NetworkPolicy defaults

If your cluster runs a `default-deny` egress policy already, pllm's
`restrict_egress` is additive — it allowlists DNS + pllm on top of your
default. Verify by running:

```bash
kubectl -n pllm-mcp describe networkpolicy
```

You should see two policies: yours (default-deny) and pllm's (targeted
allowlist).

## CNI requirements

NetworkPolicy is only enforced if your CNI supports it. Calico, Cilium,
Weave, Antrea — yes. Flannel plain — no, the NetworkPolicy object exists but
nothing honors it. If you can't enforce, the flag is documentation theater.

Test:

```bash
# From an unrestricted pod
kubectl run tmp --image=nicolaka/netshoot --rm -it -- curl -m 2 https://example.com

# From a restricted pllm-mcp pod (should fail)
kubectl -n pllm-mcp exec <pod> -- curl -m 2 https://example.com
```
