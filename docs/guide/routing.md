# Model Routing & Load Balancing

PLLM provides intelligent routing to distribute requests across multiple model instances based on latency, priority, or round-robin strategies.

## How Routing Works

When a request comes in:
1. **Filter instances**: Get all instances for the requested model
2. **Filter healthy**: Remove instances with circuit breakers open
3. **Apply strategy**: Select best instance based on configured strategy
4. **Return instance**: Route request to selected provider

The routing strategy is configured via `router.routing_strategy` in your config.

## Routing Strategies

PLLM supports four routing strategies (implemented in `selectInstanceByStrategy`):

### 1. Priority-Based (Default)

Routes to the highest priority instance (lowest priority number).

```yaml
router:
  routing_strategy: "priority"

model_list:
  - model_name: gpt-4-openai
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}
    priority: 1  # Try first

  - model_name: gpt-4-azure
    params:
      model: azure/gpt4-deployment
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}
    priority: 2  # Fallback
```

**Use case:** Simple failover, predictable routing

### 2. Least-Latency (Recommended for Production)

Routes to the instance with the lowest average latency using distributed Redis tracking.

```yaml
router:
  routing_strategy: "least-latency"

redis:
  url: redis://localhost:6379  # Required for distributed latency

model_list:
  - model_name: gpt-4-openai
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: gpt-4-azure
    params:
      model: azure/gpt4-deployment
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}
```

**How it works:**
1. Every request records latency to Redis (`pllm:latency:{model_name}`)
2. Router queries distributed latency for each instance
3. Selects instance with lowest average latency
4. All pods share latency data via Redis

**Use case:** Multi-instance deployments, performance optimization

### 3. Weighted Round-Robin

Distributes requests based on configured weights.

```yaml
router:
  routing_strategy: "weighted-round-robin"

model_list:
  - model_name: gpt-4-openai
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}
    weight: 70  # 70% of traffic

  - model_name: gpt-4-azure
    params:
      model: azure/gpt4-deployment
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}
    weight: 30  # 30% of traffic
```

**Use case:** Controlled load distribution, cost optimization

### 4. Random

Randomly selects an available instance.

```yaml
router:
  routing_strategy: "random"
```

**Use case:** Simple load distribution without state

## Distributed Latency Tracking

For multi-instance (Kubernetes) deployments, PLLM uses Redis to share latency metrics across all pods.

### Architecture

```
Pod 1 (Request with 2s latency)
  ↓
  Records to Redis ZSET: pllm:latency:gpt-4-openai
  ↓
Pod 2 (New request arrives)
  ↓
  Reads latency from Redis
  ↓
  Sees: gpt-4-openai = 2s, gpt-4-azure = 500ms
  ↓
  Routes to gpt-4-azure (faster)
```

### Configuration

```yaml
# Redis required for distributed latency
redis:
  url: redis://localhost:6379
  pool_size: 10

# Enable least-latency routing
router:
  routing_strategy: "least-latency"
```

### Latency Metrics

Each model instance tracks:
- **Average latency**: Exponential moving average (EMA)
- **P50/P95/P99**: Percentile latencies
- **Sample count**: Number of recent requests
- **Health score**: 0-100 based on P95 latency
- **Window**: 5 minutes (configurable)
- **Max samples**: 1000 per model (configurable)

### What Counts as Latency?

Latency includes the **full end-to-end response time**:

1. **Routing time** (~50ms): Selecting best instance
2. **Network to provider** (~100-500ms): HTTP request
3. **LLM processing** (variable): Token generation
4. **Network back** (~100-500ms): Receiving response
5. **Streaming**: All chunks sent to client

**Example:**
- Large prompt (10K tokens) to GPT-4
- LLM takes 25s to process
- **Recorded latency: 25-26s** (full response time)
- Other pods see "gpt-4 is slow" and route elsewhere

## Model Aliases

Create user-friendly aliases for groups of models:

```yaml
model_list:
  - model_name: gpt-4-openai
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: gpt-4-azure
    params:
      model: azure/gpt4-deployment
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}

  - model_name: claude-3-sonnet
    params:
      model: claude-3-sonnet-20240229
      api_key: ${ANTHROPIC_API_KEY}

# Create aliases
model_aliases:
  # Users call "smart" → routes to fastest
  smart: ["gpt-4-openai", "gpt-4-azure", "claude-3-sonnet"]

  # Provider-specific
  gpt-4: ["gpt-4-openai", "gpt-4-azure"]
  claude: ["claude-3-sonnet"]
```

**Usage:**
```bash
# User calls "smart" alias
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model": "smart", "messages": [...]}'

# PLLM routes to fastest of: gpt-4-openai, gpt-4-azure, claude-3-sonnet
```

## Multi-Instance Routing Example

**Scenario:** 3 Kubernetes pods, multiple GPT-4 backends

```yaml
router:
  routing_strategy: "least-latency"
  circuit_breaker_enabled: true

redis:
  url: redis://redis-service:6379

model_list:
  # Primary: OpenAI (fast, expensive)
  - model_name: gpt-4-openai
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  # Secondary: Azure (slower, cheaper)
  - model_name: gpt-4-azure
    params:
      model: azure/gpt4
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}

  # Tertiary: OpenRouter (slowest, cheapest)
  - model_name: gpt-4-openrouter
    params:
      model: openai/gpt-4-turbo
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1

model_aliases:
  gpt-4: ["gpt-4-openai", "gpt-4-azure", "gpt-4-openrouter"]
```

**Behavior:**
1. **Initial state**: All instances equal, routes to `gpt-4-openai` (priority)
2. **After 10 requests**:
   - `gpt-4-openai`: 800ms average → most traffic
   - `gpt-4-azure`: 1500ms average → some traffic
   - `gpt-4-openrouter`: 2000ms average → minimal traffic
3. **OpenAI degrades**: Latency spikes to 5s
4. **Automatic shift**: Routes to `gpt-4-azure` (now fastest)
5. **All pods see shift**: Shared Redis latency

## Health Checks & Circuit Breakers

PLLM automatically excludes unhealthy instances:

```yaml
router:
  circuit_breaker_enabled: true
  circuit_breaker_threshold: 3      # Open after 3 failures
  circuit_breaker_cooldown: 30s     # Retry after 30s
  health_check_interval: 30s        # Health check frequency
```

**Circuit Breaker States:**
- **CLOSED**: Normal operation, all requests allowed
- **OPEN**: Too many failures, block all requests
- **HALF_OPEN**: Testing recovery, allow limited requests

## Monitoring Routing Decisions

```bash
# View model stats (includes routing metrics)
curl http://localhost:8080/api/admin/models/stats

# Response includes per-model:
{
  "gpt-4-openai": {
    "health_score": 95,
    "avg_latency": "823ms",
    "total_requests": 1523,
    "requests_minute": 45
  },
  "gpt-4-azure": {
    "health_score": 78,
    "avg_latency": "1456ms",
    "total_requests": 892,
    "requests_minute": 12
  }
}
```

## Best Practices

### 1. Use Unique Model Names

❌ **Wrong:**
```yaml
model_list:
  - model_name: gpt-4  # Same name!
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: gpt-4  # Same name!
    params:
      model: azure/gpt4
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}
```

✅ **Correct:**
```yaml
model_list:
  - model_name: gpt-4-openai     # Unique!
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: gpt-4-azure      # Unique!
    params:
      model: azure/gpt4
      api_base: https://endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}

model_aliases:
  gpt-4: ["gpt-4-openai", "gpt-4-azure"]  # User-friendly
```

### 2. Multi-Instance Deployments

For Kubernetes/multi-pod deployments:
- ✅ Use `routing_strategy: "least-latency"`
- ✅ Configure Redis for distributed tracking
- ✅ Set appropriate `circuit_breaker_threshold`
- ✅ Monitor health scores via admin API

### 3. Single Instance Deployments

For single-server deployments:
- ✅ Use `routing_strategy: "priority"` (simpler)
- ✅ Redis optional (uses in-memory fallback)
- ✅ Configure fallback chains

### 4. Cost Optimization

```yaml
router:
  routing_strategy: "weighted-round-robin"

model_list:
  # Expensive but fast
  - model_name: gpt-4-openai
    weight: 20  # 20% of traffic

  # Cheaper alternative
  - model_name: gpt-4-azure
    weight: 80  # 80% of traffic
```

## Troubleshooting

### All Requests Go to One Instance

**Cause:** Other instances have higher latency or are unhealthy

**Solution:**
```bash
# Check health scores
curl http://localhost:8080/api/admin/models/stats

# Reset circuit breakers
curl -X POST http://localhost:8080/api/admin/circuit-breakers/reset
```

### Latency Not Updating

**Cause:** Redis connection issue

**Solution:**
```bash
# Check Redis connectivity
redis-cli -h localhost ping

# Verify latency data
redis-cli ZRANGE "pllm:latency:gpt-4-openai" 0 -1 WITHSCORES
```

### Routing to Wrong Instance

**Cause:** Model name mismatch

**Solution:**
- Ensure `model_name` is unique per instance
- Check `model_aliases` configuration
- Verify user requests match alias or model name
