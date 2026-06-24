# Policy Control-Plane Integration Contract (v1)

This document defines the contract between a **Gateway** (the data-plane that
sits in the agent/tool/LLM call path and knows the catalog of resources) and a
**Control Plane** (the policy authority that authors policy and decides
allow/deny). It lets a Gateway publish its catalog to a Control Plane and ask it
for runtime authorization decisions.

## Design rule: vendor neutrality (normative)

The contract is **symmetric and product-neutral**. Neither side hardcodes the
other's product name as a keyword.

- No product or vendor name appears in any **path**, **JSON field**, **header**,
  **enum value**, or **type name** in this contract.
- The vocabulary is **roles**, not products: *Gateway*, *Control Plane*, *PEP*,
  *PDP*, *catalog*, *use case*, *subject*, *target*.
- Product names appear **only** in the non-normative "Reference implementations"
  appendix, and only to describe how a given implementation fills a role.

A Gateway must be able to talk to any conforming Control Plane and vice versa.

## Roles & vocabulary

| Role | What it does |
|---|---|
| **Gateway** | Data plane in the call path; owns the **catalog** of governable resources and a binding of resources to use cases. Hosts one or more **PEPs**. |
| **PEP** (Policy Enforcement Point) | The chokepoint that calls the PDP and enforces the verdict. Lives in the Gateway, or as an **Interceptor** inside a third-party gateway. |
| **Control Plane** | Authors policy, stores it, and hosts the **PDP**. Ingests the Gateway catalog so operators can attach policy to resources per use case. |
| **PDP** (Policy Decision Point) | Evaluates a policy for one request and returns a verdict. |
| **Resource** | A governable target: `agent`, `server` (MCP server), `skill`, `prompt`, or `llm`. |
| **Use case** | A business-activity grouping that binds resources and carries data classifications and an approval status. The unit policy is attached to. |
| **Subject** | The calling identity (e.g. an API key / agent identity) behind a request. |

Two APIs make up the contract:

1. **Catalog API** — the Gateway exposes it; the Control Plane reads it.
2. **Decision API** — the Control Plane exposes it; the PEP calls it.

All paths below are relative; each side's base URL is deployment configuration.
All payloads are JSON. All endpoints require `Authorization: Bearer <token>`.

---

## 1. Canonical identity (the join key)

Everything hinges on **one stable identifier per resource** that is identical in
all three places:

1. the Gateway's **catalog** entry (`resource.name`),
2. the Gateway's **Decision API** request (`target.name`),
3. the Control Plane's **policy attachment** key.

Rules:

- A resource's identity is the pair **`(kind, name)`** — optionally narrowed by
  `version` and, for MCP tool calls, `tool`.
- `name` is the resource's **canonical catalog name** (e.g.
  `io.github.acme/refund-agent`). It is opaque, case-sensitive, and stable
  across the resource's lifetime; renames are a new identity.
- The Control Plane MUST attach policy on `(use_case, kind, name)` and MUST NOT
  invent its own primary key for matching. If it stores an internal id, that id
  is private and never crosses this contract.

If these three uses of the identifier diverge, decisions silently miss and
nothing enforces — so this is the first thing to lock between implementations.

---

## 2. Catalog API (Gateway exposes, Control Plane reads)

The Control Plane **pulls** the catalog (it drives sync; the Gateway stays
stateless about consumers). The Gateway SHOULD also emit change events
(webhooks) so the Control Plane can refresh promptly; pull remains the source of
truth.

### 2.1 Resources

```
GET {catalog-base}/resources?kind={agent|server|skill|prompt|llm}&updated_since={rfc3339}
```

```json
{
  "items": [
    {
      "kind": "agent",
      "name": "io.github.acme/refund-agent",
      "version": "1.4.0",
      "display_name": "Refund Agent",
      "description": "...",
      "source": "local",
      "data_classifications": ["PII"],
      "metadata": {},
      "updated_at": "2026-06-23T10:00:00Z"
    }
  ],
  "next_updated_since": "2026-06-23T10:00:00Z"
}
```

- `source` is provenance for federated/imported entries — an opaque token
  (`"local"` or an upstream id). Never a product name.
- `updated_since` + `next_updated_since` enable incremental sync.

### 2.2 Use cases

```
GET {catalog-base}/use-cases?updated_since={rfc3339}
```

```json
{
  "items": [
    {
      "id": "9f1c...",
      "slug": "refund-automation",
      "name": "Refund Automation",
      "risk_tier": "tier2",
      "status": "approved",
      "data_classifications": ["PII"],
      "effective_classifications": ["PII", "Financial"],
      "compliance_flag": false,
      "updated_at": "2026-06-23T10:00:00Z"
    }
  ]
}
```

### 2.3 Bindings (use case ↔ resource edges)

So the Control Plane can offer "attach a policy to *this resource* in *this use
case*", the Gateway exposes the edges:

```
GET {catalog-base}/bindings?use_case={slug-or-id}
```

```json
{
  "items": [
    { "use_case": "refund-automation", "resource_kind": "agent", "resource_name": "io.github.acme/refund-agent" }
  ]
}
```

A resource may appear in multiple use cases — that is exactly how the same agent
gets different policies in different use cases.

### 2.4 Change events (optional, recommended)

The Gateway MAY POST signed events to a Control-Plane-registered URL on catalog
or use-case change, so the Control Plane refreshes without polling:

```
POST <control-plane-callback-url>
Headers: X-Governance-Event, X-Governance-Delivery-Id, X-Governance-Signature: sha256=<hmac>
Body: { "type": "resource.changed" | "use_case.changed" | "binding.changed", "data": { ... } }
```

Header and event names are neutral; they do not encode a product.

---

## 3. Decision API (Control Plane exposes, PEP calls)

One synchronous call per governed request, before the action executes.

```
POST /v1/authorize
```

### 3.1 Request — the policy input document

```json
{
  "action": "call_llm | call_mcp_tool | invoke_agent | fetch_prompt",
  "use_case": {
    "id": "9f1c...",
    "name": "Refund Automation",
    "slug": "refund-automation",
    "risk_tier": "tier2",
    "status": "approved",
    "enforce_scope": true,
    "data_classifications": ["PII"],
    "effective_classifications": ["PII", "Financial"],
    "compliance_flag": false
  },
  "target": {
    "kind": "agent",
    "name": "io.github.acme/refund-agent",
    "tool": null
  },
  "subject": { "key_id": "uuid|null", "key_name": "string|null", "remote_addr": "string|null" },
  "context": { "request_id": "string|null" }
}
```

- `action` enum is fixed and neutral.
- `target.tool` is optional, set for `call_mcp_tool` when per-tool granularity is
  needed (otherwise the decision is server-granular).
- `subject` and `context` are **reserved/optional** for forward compatibility; a
  PEP MAY omit them today. Policies should not require them in v1.
- A Control Plane evaluates its policy for `(use_case, target)` against this
  document and returns the verdict.

### 3.2 Response — the verdict

```json
{ "allow": true, "enforce": true, "reason": "string" }
```

- `allow` — whether the action is permitted.
- `enforce` — **nullable**. `true` = block on deny; `false` = flag/audit only.
  When omitted/null, the PEP falls back to `use_case.enforce_scope`.
- `reason` — human-readable; the PEP records it in its audit trail.

### 3.3 PEP obligations (fail-mode, latency)

The **PEP owns the failure policy** — the Control Plane cannot. On timeout/error
the PEP applies one of:

- `fallback_local` — fall back to the Gateway's built-in decision logic
  (default; degrades to baseline enforcement).
- `deny` — fail closed.
- `allow` — fail open.

The Control Plane SHOULD answer fast (target p99 ≈ tens of ms) and decisions
SHOULD be cacheable by `(action, use_case.id, target)` for a short TTL; the PEP
MAY cache.

---

## 4. Enforcement tiers

The same Decision API serves every tier; only *who calls it* changes.

1. **Inline** — the Gateway's own PEP calls `/v1/authorize` and blocks. Strongest.
2. **Interceptor** — a third-party gateway (one not built around this contract)
   runs a **pre-hook / interceptor** that calls `/v1/authorize` before the tool
   executes. Same request/response shape. This is how a customer with their own
   gateway enforces without re-routing traffic.
3. **Observe-only** — the Gateway cannot block; it emits events and the Control
   Plane evaluates post-hoc for audit/alerting. No `/v1/authorize` call in the
   hot path.

An **Interceptor** is just a PEP hosted outside the Gateway. It MUST send the
same input document and honour the same verdict + fail-mode rules.

---

## 5. Versioning & security

- **Versioning:** the `/v1/` path segment. Only additive changes within `v1`
  (new optional fields, new enum values a peer may ignore). Breaking changes →
  `/v2/`.
- **Auth:** bearer tokens in both directions, issued per integration. mTLS
  optional and orthogonal.
- **Integrity:** change-event callbacks SHOULD be HMAC-signed
  (`X-Governance-Signature: sha256=<hmac>`). Decision requests MAY be signed.
- **No secrets in the input document.** The policy input carries identifiers and
  classifications, never credentials.

---

## Appendix A — Reference implementations (non-normative)

> This is the **only** section that names concrete products, and only to record
> how each fills a role. Nothing here is part of the normative contract; an
> implementation of either role must not depend on the other's identity.

**Gateway role.** This repository (pllm) fills the Gateway role:

- Decision API client is already implemented — it builds the §3.1 input document
  and posts it (`internal/services/use_cases/pdp.go`, `PDPDecider`), with the
  fail-mode of §3.3 (`CompositeDecider`, config `pdp.on_error`). The wire shape
  (`pdpRequest`/`pdpResponse`) matches §3 verbatim.
- Catalog API (§2) is implemented as a read-only facade
  (`internal/api/handlers/catalog`) over the existing registry, use-case, and
  ref services, keyed on the §1 canonical identity. It is mounted (admin-gated,
  since it exposes use-case governance data) at `{catalog-base}` =
  `/api/admin/catalog/`, i.e. `/api/admin/catalog/{resources,use-cases,bindings}`.
- Canonical identity (§1) is the registry resource name used in `RegistryRef`.

**Control Plane role.** A governance product fills the Control Plane role by:

- ingesting §2 to list resources/use-cases/bindings for policy attachment;
- attaching policy on `(use_case, kind, name)` per §1;
- implementing `POST /v1/authorize` (§3) backed by its policy engine (e.g. OPA),
  reading the input document as `input.*`.

**Third-party gateways** (e.g. an NGINX-fronted MCP registry, or a managed
agent-gateway with interceptors) fill the Gateway role at tier 2 (§4) by running
an Interceptor that speaks the Decision API.
