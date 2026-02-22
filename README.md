<div align="center">

# pLLM

### High-Performance LLM Gateway Built in Go

[![CI Status](https://img.shields.io/github/actions/workflow/status/andreimerfu/pllm/ci.yml?branch=main&style=classic&logo=github-actions&label=CI)](https://github.com/andreimerfu/pllm/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/codecov/c/github/andreimerfu/pllm?style=classic&logo=codecov)](https://codecov.io/gh/andreimerfu/pllm)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=classic&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=classic)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=classic&logo=docker)](https://hub.docker.com/r/amerfu/pllm)
[![Helm Chart](https://img.shields.io/badge/Helm-Chart-0F1689?style=classic&logo=helm)](https://github.com/andreimerfu/pllm/tree/main/deploy/helm)
[![OpenAI Compatible](https://img.shields.io/badge/OpenAI-Compatible-412991?style=classic&logo=openai)](https://platform.openai.com)

**Drop-in OpenAI replacement** · **Intelligent route-based model orchestration** · **Enterprise-grade reliability**

[Quick Start](#quick-start) · [Routes](#routes--intelligent-model-orchestration) · [Documentation](docs/)

</div>

---

## What is pLLM?

pLLM is an LLM gateway that sits between your application and LLM providers. Instead of hardcoding a single provider into your app, you point your existing OpenAI SDK calls at pLLM and gain intelligent routing, automatic failover, load balancing, caching, and observability — with zero code changes.

```
Your App (OpenAI SDK)  ──>  pLLM Gateway  ──>  OpenAI / Anthropic / Azure / Bedrock / Vertex AI / Grok / Cohere
```

Built in Go for high throughput, low latency, and minimal resource usage. A single instance handles thousands of concurrent requests with sub-millisecond overhead.

## Why pLLM?

| | |
|:--|:--|
| **Zero code changes** | Uses the standard OpenAI API format. Change `base_url` and you're done. |
| **Multi-provider** | OpenAI, Anthropic, Azure OpenAI, AWS Bedrock, Google Vertex AI, Grok, Cohere — all behind one endpoint. |
| **Intelligent routing** | Routes are virtual model endpoints that select the best real model using configurable strategies. |
| **Automatic failover** | If a provider goes down, requests transparently retry on healthy alternatives. Zero failed requests. |
| **High performance** | Go's native concurrency handles thousands of parallel requests on a single instance. No GIL, no interpreter overhead. |
| **Cost control** | Budget limits, usage tracking, multi-key load balancing, and caching reduce LLM spend. |
| **Production ready** | JWT auth, rate limiting, Prometheus metrics, distributed tracing, Helm chart, auto-scaling. |

---

## Routes — Intelligent Model Orchestration

Routes are the core concept that makes pLLM powerful. A **route** is a virtual model endpoint that maps to multiple real models and uses a **strategy** to decide which one handles each request.

### The Problem

LLM applications face hard trade-offs:

- **Vendor lock-in** — Hardcoding `gpt-4` means you can't switch to Claude or Gemini without code changes.
- **No resilience** — If OpenAI has an outage, your app goes down with it.
- **Cost inefficiency** — Sending all traffic to one provider means you can't optimize spend across providers with different pricing.
- **Rate limit walls** — A single API key has fixed RPM/TPM limits that cap your throughput.
- **No adaptability** — You can't route latency-sensitive requests differently from batch workloads.

### The Solution: Routes

With routes, your application sends requests to a virtual model name (e.g., `smart` or `fast`), and pLLM handles the rest:

```
App sends: model="smart"
                │
                ▼
        ┌──── Route: "smart" ────┐
        │  Strategy: least-latency│
        │                         │
        │  ┌─────────────────┐   │
        │  │ GPT-4 Turbo     │◄──┼── lowest latency? → selected
        │  │ Claude 3 Opus   │   │
        │  │ Azure GPT-4     │   │
        │  └─────────────────┘   │
        │                         │
        │  Fallbacks:             │
        │  → GPT-3.5 Turbo       │
        │  → Claude 3 Haiku      │
        └─────────────────────────┘
```

If the selected model fails, pLLM automatically tries the next model in the route. If all route models fail, it falls through to the fallback chain. Your application never sees an error as long as any alternative is available.

### Routing Strategies

Each route uses one of four strategies to select a model:

| Strategy | Behavior | Best For |
|:---------|:---------|:---------|
| **priority** (default) | Always picks the highest-priority model that is healthy | Cost optimization — prefer cheap models, fall back to expensive ones |
| **least-latency** | Picks the model with the lowest observed response time (tracked via Redis across all gateway instances) | Latency-sensitive applications — always routes to the fastest provider |
| **weighted-round-robin** | Distributes traffic proportionally by weight (smooth WRR like nginx) | Load distribution — split traffic 70/30 across providers |
| **random** | Picks a random model | Simple distribution, chaos testing |

### Route Configuration

Routes can be defined in `config.yaml` or created dynamically via the admin API.

**config.yaml:**

```yaml
routes:
  - name: "Smart Models"
    slug: "smart"
    description: "Routes to the best available high-quality model"
    strategy: "least-latency"
    models:
      - model_name: "gpt-4-turbo"
        weight: 50
        priority: 1
      - model_name: "claude-3-opus"
        weight: 50
        priority: 2
      - model_name: "azure-gpt-4"
        weight: 50
        priority: 3
    fallback_models: ["gpt-35-turbo", "claude-3-haiku"]
    enabled: true

  - name: "Fast Models"
    slug: "fast"
    description: "Speed-optimized models for low-latency use cases"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt-35-turbo"
        weight: 60
      - model_name: "claude-3-haiku"
        weight: 40
    enabled: true
```

**Admin API:**

```bash
curl -X POST http://localhost:8080/api/routes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production LLM",
    "slug": "prod-llm",
    "strategy": "weighted-round-robin",
    "models": [
      {"model_name": "gpt-4-turbo", "weight": 70, "priority": 1},
      {"model_name": "claude-3-opus", "weight": 30, "priority": 2}
    ],
    "fallback_models": ["gpt-35-turbo"]
  }'
```

Then use it from any OpenAI-compatible client:

```python
client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-key")
response = client.chat.completions.create(
    model="prod-llm",  # ← this is a route, not a real model
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Why Routes Matter for LLM Applications

**1. Decouple your application from providers.** Your code references `smart` or `fast`, not `gpt-4` or `claude-3-opus`. Swap providers, add new ones, or change strategies without touching application code.

**2. Eliminate single points of failure.** Every LLM provider has outages. Routes with failover chains mean your application stays up even when individual providers go down. The three-level failover system (instance retry → next model in route → fallback models) makes zero-downtime a realistic goal.

**3. Optimize cost dynamically.** Use a priority route that prefers your cheapest provider and only escalates to expensive models when the cheap one is unavailable or rate-limited. Or use weighted round-robin to distribute spend across providers with different pricing tiers.

**4. Break through rate limits.** A single OpenAI API key might cap at 60 RPM. Define a route with the same model across 5 API keys (or 5 Azure deployments), and pLLM distributes traffic across all of them — effectively multiplying your throughput.

**5. Adapt to real-world conditions.** The least-latency strategy continuously measures provider response times (distributed across all gateway instances via Redis) and routes to the fastest one. If a provider degrades, traffic shifts automatically.

**6. Manage routes at runtime.** Create, update, and monitor routes through the admin API and dashboard without restarting the gateway. View traffic distribution stats to understand where requests are going and how much they cost.

---

## Features

### Compatibility
- **100% OpenAI Compatible** — Drop-in replacement, no code changes
- **Multi-Provider** — OpenAI, Anthropic, Azure, Bedrock, Vertex AI, Grok, Cohere
- **Streaming** — Real-time streaming responses for all providers

### Routing & Reliability
- **Routes** — Virtual model endpoints with strategy-based selection
- **4 Routing Strategies** — Priority, least-latency, weighted round-robin, random
- **3-Level Failover** — Instance retry → route model fallback → fallback chain
- **Health Tracking** — Real-time health scores with circuit breakers
- **Multi-Key Load Balancing** — Distribute load across multiple API keys

### Security & Access Control
- **JWT Authentication** — Role-based access with Dex OIDC support
- **API Key Management** — Per-key permissions and usage tracking
- **Rate Limiting** — Per-user, per-model, per-endpoint controls
- **Budget Management** — User and team-based spending limits

### Observability
- **Prometheus Metrics** — Request rates, latencies, token usage, costs
- **Distributed Tracing** — OpenTelemetry integration
- **Route Stats** — Traffic distribution, cost breakdown per route
- **Admin Dashboard** — Web UI for monitoring and configuration
- **Swagger UI** — Interactive API documentation

---

## Quick Start

### Docker Compose (Development)

```bash
# Clone and setup
git clone https://github.com/andreimerfu/pllm.git && cd pllm
cp .env.example .env

# Add your API key
echo "OPENAI_API_KEY=sk-your-key-here" >> .env

# Launch
docker compose up -d

# Test
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello!"}]}'
```

### Kubernetes with Helm (Production)

```bash
# Add the Helm repository
helm repo add pllm https://andreimerfu.github.io/pllm
helm repo update

# Install
helm install pllm pllm/pllm \
  --set pllm.secrets.jwtSecret="your-jwt-secret" \
  --set pllm.secrets.masterKey="sk-master-your-key" \
  --set pllm.secrets.openaiApiKey="sk-your-openai-key"
```

<details>
<summary><b>Production Helm values</b></summary>

```yaml
pllm:
  secrets:
    jwtSecret: "your-super-secret-jwt-key-min-32-chars"
    masterKey: "sk-master-production-key"
    openaiApiKey: "sk-your-openai-key"
    anthropicApiKey: "sk-ant-your-anthropic-key"

replicaCount: 3
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: pllm.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: pllm-tls
      hosts:
        - pllm.yourdomain.com

serviceMonitor:
  enabled: true

postgresql:
  enabled: true
  auth:
    database: pllm
    username: pllm
    password: "your-secure-db-password"

redis:
  enabled: true
  auth:
    enabled: true
    password: "your-secure-redis-password"
```

</details>

<details>
<summary><b>External dependencies (cloud managed services)</b></summary>

```yaml
postgresql:
  enabled: false
redis:
  enabled: false

pllm:
  config:
    database:
      host: "your-rds-instance.amazonaws.com"
      port: 5432
      name: pllm
      sslMode: require
    redis:
      host: "your-redis-cluster.cache.amazonaws.com"
      port: 6379
      tls: true

  secrets:
    databasePassword: "your-db-password"
    redisPassword: "your-redis-password"
    jwtSecret: "your-jwt-secret"
    masterKey: "sk-master-key"
    openaiApiKey: "sk-openai-key"
```

</details>

### Standalone Docker

```bash
docker run -d \
  --name pllm \
  -p 8080:8080 \
  -e OPENAI_API_KEY=sk-your-key \
  -e JWT_SECRET=your-jwt-secret \
  -e MASTER_KEY=sk-master-key \
  amerfu/pllm:latest
```

### Local Development

```bash
# Prerequisites: Go 1.23+, Node.js, PostgreSQL, Redis
git clone https://github.com/andreimerfu/pllm.git && cd pllm

# Start dependencies
docker compose up postgres redis -d

# Install and run
go mod download
cd web && npm ci && cd ..
make dev  # Hot reload with air
```

### Endpoints

| Endpoint | Description |
|:---------|:------------|
| `http://localhost:8080/v1` | OpenAI-compatible API |
| `http://localhost:8080/swagger` | Interactive API docs |
| `http://localhost:8080/ui` | Admin dashboard |
| `http://localhost:8080/docs` | Documentation |
| `http://localhost:8080/metrics` | Prometheus metrics |
| `http://localhost:8080/health` | Health check |

---

## Integration

pLLM works with any OpenAI-compatible client. Change `base_url` and you're done.

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="smart",  # route name or model name
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:8080/v1'
});

const response = await client.chat.completions.create({
  model: 'smart',
  messages: [{ role: 'user', content: 'Hello!' }]
});
```

### LangChain

```python
from langchain.chat_models import ChatOpenAI

llm = ChatOpenAI(
    openai_api_base="http://localhost:8080/v1",
    openai_api_key="your-api-key",
    model="smart"
)
```

---

## Configuration

### Environment Variables

```bash
# .env
OPENAI_API_KEY=sk-your-key-here
ANTHROPIC_API_KEY=your-anthropic-key
AZURE_API_KEY=your-azure-key

# Multi-key load balancing
OPENAI_API_KEY_2=sk-second-key
OPENAI_API_KEY_3=sk-third-key
```

### Model Configuration (config.yaml)

```yaml
model_list:
  - model_name: my-gpt-4
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: my-claude
    params:
      model: claude-3-opus-20240229
      api_key: ${ANTHROPIC_API_KEY}
```

### Router Configuration

```yaml
router:
  routing_strategy: "least-latency"
  enable_failover: true
  instance_retry_attempts: 2
  failover_timeout_multiple: 1.5
  circuit_breaker_enabled: true

  model_fallbacks:
    gpt-4: gpt-35-turbo
    claude-3-opus: claude-3-sonnet
```

### Route Examples

<details>
<summary><b>Cost optimization — prefer cheap, fall back to expensive</b></summary>

```yaml
routes:
  - name: "Cost Optimized"
    slug: "cost-opt"
    strategy: "priority"
    models:
      - model_name: "gpt-35-turbo"
        priority: 1    # try first (cheapest)
      - model_name: "gpt-4-turbo"
        priority: 2    # fall back if needed
      - model_name: "claude-3-opus"
        priority: 3    # last resort
```

</details>

<details>
<summary><b>Provider redundancy — survive any single provider outage</b></summary>

```yaml
routes:
  - name: "Reliable"
    slug: "reliable"
    strategy: "priority"
    models:
      - model_name: "openai-gpt4"
        priority: 1
      - model_name: "azure-gpt4"
        priority: 2
      - model_name: "anthropic-claude"
        priority: 3
    fallback_models: ["local-llm"]
```

</details>

<details>
<summary><b>Load distribution — split traffic across providers</b></summary>

```yaml
routes:
  - name: "Balanced"
    slug: "balanced"
    strategy: "weighted-round-robin"
    models:
      - model_name: "openai-gpt4"
        weight: 40    # 40% of traffic
      - model_name: "azure-gpt4"
        weight: 35    # 35% of traffic
      - model_name: "anthropic-claude"
        weight: 25    # 25% of traffic
```

</details>

<details>
<summary><b>Rate limit multiplication — same model, multiple keys</b></summary>

```yaml
routes:
  - name: "High Throughput GPT-4"
    slug: "ht-gpt4"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt4-key1"
        weight: 50
      - model_name: "gpt4-key2"
        weight: 50
```

Each `model_name` points to a different API key for the same underlying model, effectively doubling your rate limit.

</details>

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        pLLM Gateway                         │
│                                                             │
│  Request ──> Auth ──> Rate Limit ──> Cache Check            │
│                                         │                   │
│                                    Route Resolution         │
│                                         │                   │
│                              ┌──── Strategy ────┐           │
│                              │  priority         │           │
│                              │  least-latency    │           │
│                              │  weighted-rr      │           │
│                              │  random           │           │
│                              └──────────────────┘           │
│                                         │                   │
│                              Instance Selection             │
│                              Health Check + Failover        │
│                                         │                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │  OpenAI  │ │ Anthropic│ │  Azure   │ │ Bedrock  │ ...  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
│                                                             │
│  PostgreSQL (state) · Redis (cache, latency, locks, queues) │
│  Prometheus (metrics) · OpenTelemetry (tracing)             │
└─────────────────────────────────────────────────────────────┘
```

**Tech stack:** Go · Chi router · GORM + PostgreSQL · Redis · Prometheus · Swagger

---

## Scaling for High Volume

The gateway itself is rarely the bottleneck — LLM providers are. A single pLLM instance handles thousands of concurrent requests, but a single OpenAI deployment might cap at 60-100 RPM.

To scale:

1. **Multiple deployments of the same model** — Create several Azure OpenAI deployments or use multiple API keys, then put them behind a weighted round-robin route.
2. **Multi-provider redundancy** — Use the same model from different providers (OpenAI + Azure + Bedrock) to multiply throughput and add resilience.
3. **Geographic distribution** — Deploy models across regions and use least-latency routing to minimize response times.

```yaml
# Example: 5 GPT-4 deployments behind one route
routes:
  - name: "High-Scale GPT-4"
    slug: "gpt4"
    strategy: "weighted-round-robin"
    models:
      - model_name: "azure-gpt4-east-1"
        weight: 20
      - model_name: "azure-gpt4-east-2"
        weight: 20
      - model_name: "azure-gpt4-west-1"
        weight: 20
      - model_name: "bedrock-gpt4-1"
        weight: 20
      - model_name: "bedrock-gpt4-2"
        weight: 20
    fallback_models: ["gpt-35-turbo"]
```

Your application still sends `model="gpt4"`. pLLM handles the rest.

---

## Helm Chart

Available from multiple registries:

| Registry | Command |
|:---------|:--------|
| GitHub Pages | `helm repo add pllm https://andreimerfu.github.io/pllm` |
| Docker Hub (OCI) | `helm install pllm oci://registry-1.docker.io/amerfu/pllm` |
| ArtifactHub | [View on ArtifactHub](https://artifacthub.io/packages/helm/pllm/pllm) |

```bash
# List versions
helm search repo pllm/pllm --versions

# Upgrade
helm upgrade pllm pllm/pllm -f your-values.yaml

# Rollback
helm rollback pllm 1
```

---

## Monitoring

Prometheus metrics at `/metrics`, with optional ServiceMonitor for Kubernetes auto-discovery.

| Endpoint | Description |
|:---------|:------------|
| `/health` | Basic health check |
| `/ready` | Full readiness (all dependencies) |
| `/metrics` | Prometheus metrics export |
| `/api/routes/{id}/stats` | Per-route traffic and cost stats |

---

## Contributing

- [GitHub Issues](https://github.com/andreimerfu/pllm/issues) — Bug reports and feature requests
- [Documentation](docs/) — Guides and references
- Pull requests welcome

## License

[MIT License](LICENSE)

---

<div align="center">

**[Star on GitHub](https://github.com/andreimerfu/pllm)**

</div>
