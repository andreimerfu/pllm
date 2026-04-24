---
layout: home

hero:
  name: "pllm"
  text: "Control plane for agentic systems"
  tagline: LLM gateway, MCP gateway, curated registry, one-click deployments, and use-case-scoped governance — under one roof.
  actions:
    - theme: brand
      text: Quick Start
      link: /guide/quickstart
    - theme: alt
      text: Architecture
      link: /guide/architecture
    - theme: alt
      text: View on GitHub
      link: https://github.com/andreimerfu/pllm

features:
  - icon: 🚪
    title: LLM Gateway
    details: OpenAI-compatible endpoints with multi-provider routing, distributed latency-based load balancing, circuit breakers, async budgets, and real-time streaming.

  - icon: 🔌
    title: MCP Gateway
    details: Aggregate multiple Model Context Protocol backends under a single endpoint. Stdio, SSE, and HTTP transports. Per-tool ACLs on API keys.

  - icon: 📚
    title: Curated Registry
    details: Versioned catalog of servers, agents, skills, and prompts with OSV scores, dependency graph, and npm/PyPI importers.

  - icon: ☸️
    title: One-Click Deployments
    details: Publish to the registry, click deploy, pllm renders Kubernetes manifests, loads a wrapper for stdio packages, and auto-registers the backend with the MCP gateway.

  - icon: 🛡️
    title: Use-Case Governance
    details: Bind API keys to approved use cases. Every LLM, MCP tool, and agent invocation is checked against the use case's declared component set — flag or block.

  - icon: 🔎
    title: Continuous Audit
    details: Every allow/deny decision is persisted as a ScopeEvent. Export CSV, query by use case, or push HMAC-signed webhooks to your SIEM/GRC in real time.

  - icon: 🧩
    title: Guardrails
    details: Plug in Presidio, Lakera, OpenAI moderation, or Aporia. Pre-call, post-call, during-call, or logging-only modes.

  - icon: 🔐
    title: Enterprise Auth
    details: Master key, JWT, or Dex-federated OIDC (LDAP, SAML, GitHub, Google, Azure AD, Keycloak). Team-scoped keys with RegistryPermissions globs.

  - icon: 📊
    title: Full Observability
    details: Prometheus metrics, Jaeger tracing, Grafana dashboards, structured JSON logs, and a React admin UI ready out of the box.
---

## What pllm is

pllm is a single Go binary plus a React UI that sits between your applications
and every AI-native resource your organization uses — model providers, MCP
servers, and autonomous agents. It gives platform teams a governance surface
that developers don't have to opt into.

Three layers coexist under one API and one admin UI:

```mermaid
graph TB
    subgraph Clients
        Dev[Developer apps]
        Agent[Autonomous agents]
        Tool[Claude Desktop / Cursor]
    end

    subgraph pllm["pllm control plane"]
        Gateway[LLM Gateway<br/>/v1/chat/completions]
        MCP[MCP Gateway<br/>/v1/mcp]
        Registry[Registry<br/>/v1/registry/*]
        Deploy[Deployments<br/>K8s adapter]
        UseCase[Use Cases<br/>+ scope enforcement]
        Webhook[Webhook dispatcher]
    end

    subgraph Resources
        Providers[OpenAI, Anthropic, Azure,<br/>Bedrock, Vertex, OpenRouter]
        MCPBackends[MCP servers<br/>stdio / SSE / HTTP]
        Agents[Published agents]
    end

    subgraph Governance
        SIEM[SIEM / GRC]
    end

    Dev --> Gateway
    Agent --> Gateway
    Agent --> MCP
    Tool --> MCP
    Gateway --> UseCase
    MCP --> UseCase
    UseCase -.denies.-> Webhook
    Webhook --> SIEM
    Registry --> Deploy
    Deploy --> MCPBackends
    Deploy --> Agents
    MCP --> MCPBackends
    Gateway --> Providers
```

## Who it's for

- **Platform teams at regulated enterprises** who need to ship agents without
  losing track of what each one is allowed to touch.
- **ML platform / infra engineers** who want an OpenAI-compatible gateway with
  real failover, budget tracking, and observability — not just a reverse proxy.
- **Developers** who want to publish an agent once and let the platform handle
  secrets, K8s manifests, scope enforcement, and audit logging.

## Why it exists

The agentic stack fragmented fast: every team picks a framework, a model, and a
set of tools. Security and compliance end up chasing shadow infrastructure.
pllm consolidates the runtime so platform teams can answer three questions with
evidence at any moment:

1. **Who is calling what?** Every request is keyed, attributed, and tokenized.
2. **Is this call in scope?** Keys bind to approved use cases; the use case
   declares the allowed component set; runtime enforces it.
3. **Can I prove it?** Scope events are persisted, queryable, and fanned out as
   HMAC-signed webhooks to whatever tool your risk team already uses.

## First request, 10 seconds

```bash
# Spin up the full stack (Postgres, Redis, Dex, Grafana, Prometheus, Jaeger)
docker-compose up -d

# Make an OpenAI-compatible call
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

Continue with the [Quick Start](/guide/quickstart) or jump to the
[Concepts overview](/guide/concepts) to understand the moving parts.
