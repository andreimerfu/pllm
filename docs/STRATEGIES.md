# pLLM Strategies: Implementation Audit

> Generated from codebase analysis. Every claim is verified against actual source code.

---

## Table of Contents

1. [Routing Strategies](#1-routing-strategies)
2. [Failover System](#2-failover-system)
3. [Route System](#3-route-system)
4. [Rate Limiting](#4-rate-limiting)
5. [Caching](#5-caching)
6. [Health Tracking](#6-health-tracking)
7. [Budget Management](#7-budget-management)
8. [Async Usage Processing](#8-async-usage-processing)
9. [Distributed Coordination](#9-distributed-coordination)
10. [Scaling Assessment](#10-scaling-assessment)
11. [Known Gaps & Improvement Areas](#11-known-gaps--improvement-areas)

---

## 1. Routing Strategies

**Interface**: `internal/services/llm/models/routing/strategy.go`

```go
type Strategy interface {
    Name() string
    SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error)
}
```

Four strategies are implemented. The factory is in `NewStrategy()` (strategy.go:34) with fallback to priority on missing dependencies.

### 1.1 Priority Strategy (Default)

**File**: `routing/priority.go`
**Config name**: `"priority"`

Simply returns `instances[0]`. Relies on the registry pre-sorting instances by priority descending (higher number = preferred) in `model_registry.go:75-80`.

```yaml
# Example config
router:
  routing_strategy: "priority"

model_list:
  - model_name: "gpt-4"
    provider:
      type: "openai"
      api_key: "${OPENAI_KEY_1}"
    priority: 100            # Selected first

  - model_name: "gpt-4"
    provider:
      type: "azure"
      api_key: "${AZURE_KEY}"
    priority: 50             # Selected only if openai instance is removed (unhealthy)
```

**How it works at runtime**: When `ExecuteWithFailover` calls `tryModelInstances`, healthy instances are filtered, then `SelectInstance` returns the first one. If that instance fails, it's removed from the healthy list, and the next `SelectInstance` call returns the second-highest priority.

**Assessment**: Works correctly. Simple and deterministic. No scaling concerns since there's no shared state.

---

### 1.2 Least-Latency Strategy

**File**: `routing/latency.go`
**Config name**: `"least-latency"`
**Requires**: Redis (LatencyTracker). Falls back to priority if Redis unavailable.

**Selection logic** (latency.go:33-85):
1. If `LatencyTracker` exists, query Redis with a **50ms timeout** per instance (`queryCtx` at line 49)
2. For each instance, call `GetAverageLatency(ctx, config.ModelName)` from Redis
3. On Redis failure per-instance, fall back to in-memory `instance.GetAverageLatency().Load()` (an `atomic.Int64`)
4. Select instance with lowest latency
5. If Redis fails for all instances, fully fall back to `selectUsingInMemoryLatency`

```yaml
router:
  routing_strategy: "least-latency"

model_list:
  - model_name: "gpt-4"
    provider:
      type: "openai"
    # No priority/weight needed - latency drives selection
```

**Latency recording**: In-memory latency is updated via EMA in `MetricsCollector.RecordRequest` (metrics_collector.go:32-37):
```
newAvg = currentAvg * 0.9 + newLatency * 0.1
```

Distributed latency (Redis) uses sorted sets with a 5-minute sliding window and max 1000 samples per model (`latency_tracker.go:14-22`).

**Known issue**: `GetAverageLatency` queries by **model name**, not instance ID. When multiple instances serve the same model, they share the same Redis latency key. This means the distributed latency path does NOT differentiate between instances of the same model - the in-memory fallback is actually more accurate for per-instance routing.

**Assessment**: Partially working. In-memory path works correctly per-instance. Distributed path conflates instances of the same model under one key, which makes it ineffective for choosing between instances of the same model. It works for route-level model selection where each model name is different.

---

### 1.3 Weighted Round-Robin Strategy

**File**: `routing/roundrobin.go`
**Config name**: `"weighted-round-robin"`
**Requires**: ModelRegistry (for atomic counters). Falls back to priority if unavailable.

**Instance-level selection** (roundrobin.go:31-59): Uses a simple counter with modulo:
```go
index := counter.Add(1) % uint64(len(instances))
```

The comment on line 30 explicitly states:
```
// TODO: Implement weight support (currently ignores weights)
```

**This means instance-level round-robin ignores weights entirely.**

**Route-level selection** (manager.go:311-338): Implements smooth weighted round-robin (nginx-style):
```go
// For each counter value c and model i with weight w_i out of totalWeight T,
// model i is selected when floor((c)*w_i/T) > floor((c-1)*w_i/T).
c := route.rrCounter.Add(1)
```

This route-level WRR **does** respect weights and distributes traffic proportionally.

```yaml
# Route-level WRR (works with weights):
routes:
  - name: "My GPT Route"
    slug: "my-gpt"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt-4"
        weight: 70              # Gets ~70% of traffic
      - model_name: "gpt-4-azure"
        weight: 30              # Gets ~30% of traffic

# Instance-level WRR (ignores weights):
model_list:
  - model_name: "gpt-4"
    weight: 3.0                 # IGNORED - simple modulo used instead
    provider:
      type: "openai"
  - model_name: "gpt-4"
    weight: 1.0                 # IGNORED
    provider:
      type: "azure"
```

**Assessment**: Route-level WRR works correctly with proper interleaving. Instance-level WRR is just simple round-robin (weights are configured but not used). This is a significant gap if you want weighted distribution across instances of the same model.

---

### 1.4 Random Strategy

**File**: `routing/random.go`
**Config name**: `"random"`

Uses `rand.Intn(len(instances))` to select a random instance. No state, no dependencies.

```yaml
router:
  routing_strategy: "random"
```

**Assessment**: Works correctly. No scaling concerns. Provides statistical load distribution. Does not respect weights or priority.

---

## 2. Failover System

**File**: `internal/services/llm/models/manager.go`

### 2.1 Two-Level Architecture

**Level 1 - Instance retry** (`tryModelInstances`, manager.go:521-633):
1. Get all instances for a model
2. Filter to healthy instances only
3. For each retry (up to `InstanceRetryAttempts`, default 2):
   - Use routing strategy to pick best instance
   - Apply timeout: `instance.Config.Timeout * FailoverTimeoutMultiple` (default 1.5x)
   - Execute request
   - On failure: `RecordFailure` + remove instance from healthy list
   - On success: return immediately

**Level 2 - Model fallback** (`ExecuteWithFailover`, manager.go:423-518):
1. Check if model name is a route slug (if yes, delegate to route failover)
2. Try instance-level failover on current model
3. If all instances fail and `EnableModelFallback` is true:
   - Look up `ModelFallbacks[currentModel]` for next model
   - Recurse with fallback model
   - Max 10 failover entries to prevent infinite loops (line 514)

```yaml
router:
  enable_failover: true
  instance_retry_attempts: 3
  failover_timeout_multiple: 1.5
  enable_model_fallback: true
  model_fallbacks:
    "gpt-4": "gpt-3.5-turbo"         # If all gpt-4 instances fail, try gpt-3.5-turbo
    "claude-3-opus": "claude-3-sonnet" # Anthropic fallback chain
```

**Request flow example**:
```
Request for "gpt-4"
  -> Try gpt-4 instance A (priority 100) ... fails
  -> Try gpt-4 instance B (priority 50)  ... fails
  -> All gpt-4 instances exhausted
  -> Fallback to "gpt-3.5-turbo"
  -> Try gpt-3.5-turbo instance A ... succeeds
  -> Return response (AttemptCount=3, Failovers=["instance:A(err)", "instance:B(err)", "model:gpt-4(all instances failed)"])
```

### 2.2 Route-Level Failover

**Function**: `executeRouteWithFailover` (manager.go:349-419)

When the model name is a route slug:
1. Build working list of enabled route models
2. Use route strategy to select a model
3. Try all instances of selected model (via `tryModelInstances`)
4. If all instances fail, remove that model from list
5. Repeat with remaining models
6. If all route models fail, try route's `FallbackModels` (manager.go:431-438)

```yaml
routes:
  - slug: "smart-model"
    strategy: "least-latency"
    models:
      - model_name: "gpt-4"
        weight: 70
      - model_name: "claude-3-opus"
        weight: 30
    fallback_models:
      - "gpt-3.5-turbo"
```

**Assessment**: Failover is well-implemented with clear escalation. The `failovers` slice in `FailoverResult` provides full transparency for debugging. The timeout multiplier on retries prevents long hangs. The infinite-loop guard works but uses `len(failovers) > 10` which counts individual instance failures, not model-level fallbacks, so in practice the limit is lower than 5 model fallbacks.

---

## 3. Route System

**Config**: `internal/core/config/model_config.go:135-150`
**Runtime**: `internal/services/llm/models/manager.go:18-25`

Routes are named groups of models with their own strategy and fallbacks. They're accessed by slug name in API requests (e.g., `model: "my-route-slug"`).

```yaml
routes:
  - name: "Cost Optimized"
    slug: "cost-optimized"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt-3.5-turbo"
        weight: 80
        priority: 1
      - model_name: "gpt-4"
        weight: 20
        priority: 2
    fallback_models:
      - "claude-3-haiku"
    enabled: true
```

**Route model proxy** (`route_proxy.go`): Wraps route model entries to satisfy the `routing.ModelInstance` interface. Latency is aggregated from all instances of the model (manager.go:280-291).

Each route maintains its own `rrCounter atomic.Uint64` (manager.go:24) for independent round-robin state.

**Assessment**: Route system is functional and well-designed. It allows different strategies per route, which is a powerful feature.

---

## 4. Rate Limiting

**File**: `internal/services/monitoring/ratelimit/limiter.go`
**Middleware**: `internal/infrastructure/middleware/ratelimit.go`

### 4.1 Three Implementations

| Limiter | Algorithm | Used By | State |
|---------|-----------|---------|-------|
| `RedisLimiter` | Sliding Window (Sorted Sets) | Middleware (when Redis available) | Distributed |
| `InMemoryLimiter` | Token Bucket | Middleware (lite mode) | Per-process |
| `FixedWindowLimiter` | Fixed Counter Windows | **Not used by middleware** | Distributed |

**Redis Sliding Window** (limiter.go:36-80):
```
1. ZRemRangeByScore: remove entries older than window
2. ZCount: count entries in window
3. If count + n > limit: reject
4. ZAdd: add n members with nanosecond timestamps
5. Expire: set TTL = window duration
```

**In-Memory Token Bucket** (limiter.go:141-171):
```
1. Calculate elapsed time since last refill
2. refillRate = limit / window.Seconds()
3. tokens = min(limit, tokens + elapsed * refillRate)
4. If tokens >= n: consume and allow
5. Cleanup: every 5min, remove buckets idle > 1 hour
```

### 4.2 Middleware Behavior

**Key extraction** (ratelimit.go:41-51):
1. Extract `Authorization: Bearer <key>` or `X-API-Key` header
2. Fall back to client IP (`X-Forwarded-For` > `X-Real-IP` > `RemoteAddr`)

**Endpoint-specific limits** (via `getRateLimits`):
- `/v1/chat/completions` → ChatCompletionsRPM
- `/v1/completions` → CompletionsRPM
- `/v1/embeddings` → EmbeddingsRPM
- Everything else → GlobalRPM

**Skipped endpoints**: `/docs/*`, `/ui/*`, `/swagger`, `/health`, `/ready`, `/metrics`, static assets.

**Error handling**: On Redis failure, requests are **allowed through** (fail-open, line 83).

```yaml
rate_limit:
  enabled: true
  global_rpm: 60
  chat_completions_rpm: 100
  completions_rpm: 50
  embeddings_rpm: 200
```

### 4.3 Instance-Level Rate Limiting

Separate from middleware. `MetricsCollector.CheckRateLimit` (metrics_collector.go:99-117) checks per-instance RPM/TPM using in-memory atomic counters with 1-minute fixed windows.

**Known issue**: `CheckRateLimit` exists but is NOT called in the routing/selection path. The `GetBestInstance` and `tryModelInstances` functions select instances via the routing strategy without checking rate limits first. Instance-level rate limiting is defined but not enforced during request routing.

**Assessment**: HTTP-level rate limiting works well with Redis or in-memory. Instance-level rate limiting is not wired into the selection path and is therefore non-functional.

---

## 5. Caching

### 5.1 Response Cache

**File**: `internal/infrastructure/middleware/cache.go`

Caches LLM API responses at the HTTP level.

**What gets cached**:
- POST `/chat/completions`, `/completions`, `/embeddings`
- GET requests (except admin endpoints)

**What is NOT cached**:
- Streaming responses (line 88-91)
- Requests with temperature > 0 (non-deterministic, line 180)
- Admin/config endpoints

**Cache key generation** (cache.go:213):
- SHA256 of: method + path + query + normalized body
- Body normalization removes: user field, timestamp
- Authorization header is hashed for privacy

**Response headers on cache hit**: `X-Cache: HIT`, `X-Cache-Time`, `Age`

**Dual implementation**: Redis (distributed) or in-memory (lite mode).

### 5.2 Pricing Cache

**File**: `internal/services/data/cache/pricing_cache.go`

- Redis-backed, 24-hour TTL
- Key prefix: `pllm:pricing:`
- Batch-loaded at startup via Redis pipeline
- Two-tier lookup: cache first, then pricing manager
- Async cache-fill on miss

### 5.3 Budget Cache

**File**: `internal/services/data/redis/budget_cache.go`

- 5-minute TTL
- Key format: `budget:entityType:entityID`
- Optimistic on cache miss (allow request, refresh async)
- Atomic increment via `INCRBYFLOAT`

**Assessment**: Response caching is solid for deterministic requests. The temperature > 0 skip is correct. Pricing and budget caches work as expected with proper fallback chains.

---

## 6. Health Tracking

### 6.1 In-Memory Health

**File**: `internal/services/llm/models/health_tracker.go`

- `RecordSuccess`: marks healthy, resets failure count to 0
- `RecordFailure`: increments failure count, marks **unhealthy after 3 consecutive failures**
- `IsHealthy`: atomic bool check

### 6.2 Background Health Checker

**File**: `internal/services/llm/models/health_checker.go`

Periodic health checks at `HealthCheckInterval` (default 30s). Calls each provider's `HealthCheck(ctx)` method:
- **OpenAI**: `GET /models`
- **Anthropic**: `GET /v1/messages` (expects 405)
- **Azure**: `GET /openai/models`

Results stored to Redis `HealthStore` if available.

### 6.3 Distributed Health Store

**File**: `internal/services/data/redis/health_store.go`

- Key: `pllm:health:instance:{instanceID}`, TTL 5 minutes
- Model set: `pllm:health:model:{modelName}:instances`
- Aggregation: `GetModelHealth()` returns healthy count, avg latency

### 6.4 Circuit Breaker

**Defined in** `types.go:37-38`:
```go
CircuitState     atomic.Int32 // 0=closed, 1=half-open, 2=open
LastCircuitCheck atomic.Value // time.Time
```

**Implementation status**: The fields exist but **no circuit breaker logic is implemented**. There is no code that transitions between states (closed -> open -> half-open -> closed). The health tracker's 3-failure threshold is the only protection, but it lacks:
- Timeout-based recovery (open -> half-open after cooldown)
- Probe requests in half-open state
- Success threshold to close circuit

**Assessment**: Health tracking works for basic failure detection. Circuit breaker is a stub - the fields exist but aren't wired to any logic. Recovery from unhealthy state only happens when the background health checker succeeds, not through circuit breaker patterns.

---

## 7. Budget Management

### 7.1 Async Budget Middleware

**File**: `internal/infrastructure/middleware/budget_async.go`

**Flow** (line 78-170):
1. Skip non-LLM endpoints
2. Allow master key (bypass all checks)
3. **Estimate cost** before execution:
   - Input tokens: `len(content) / 4` (1 token ~ 4 chars)
   - Output tokens: `max_tokens` or default 150
   - Cost from pricing cache or $0.01 conservative estimate
4. Fast budget check via Redis cache (< 5ms)
5. On cache miss: **allow optimistically**, trigger async refresh
6. Execute request
7. **Async post-processing** (in goroutine):
   - Calculate actual cost
   - Enqueue usage record
   - Increment spent in budget cache
   - Publish usage event to Redis Stream

### 7.2 Unified Budget Service

**File**: `internal/services/data/budget/unified_service.go`

Two tiers:
- **Cached check** (`CheckBudgetCached`): Redis-first, fall back to DB
- **Full check** (`CheckBudget`): Database query with user/team relations

Budget reset periods: daily (midnight), weekly (Monday), monthly (1st), yearly (Jan 1).

**Assessment**: Budget enforcement is optimistic and non-blocking, which is good for performance but means overspending is possible during cache refresh windows (up to 5 minutes). The async tracking is well-designed.

---

## 8. Async Usage Processing

**File**: `internal/services/data/redis/usage_queue.go`

### Queue Architecture

| Queue | Type | Purpose |
|-------|------|---------|
| Main | Redis List (LPUSH/RPOP) | Primary processing |
| Retry | Redis Sorted Set (score = retry time) | Failed records |
| Dead Letter | Redis List | Records exceeding max retries |

### Processing

- **Enqueue**: `LPUSH` (< 5ms)
- **Dequeue**: Pipelined `RPOP` x batch_size (default 100)
- **Retry backoff**: `retries^2 * 10 seconds` (10s, 40s, 90s)
- **Max retries**: 3 (configurable)

### Usage Record Fields

Model, provider, method, path, status, input/output tokens, cost, latency, user/key/team IDs, timestamps.

**Assessment**: Well-designed async processing. The exponential backoff is reasonable. Dead letter queue prevents infinite retry loops. Batch dequeue via pipeline is efficient.

---

## 9. Distributed Coordination

### 9.1 Distributed Locks

**File**: `internal/services/data/redis/distributed_locks.go`

- `AcquireLock`: `SETNX` with TTL (prevents deadlocks)
- `Release`: Lua script ensuring only owner can release
- `Extend`: Lua script for TTL extension (long operations)
- `TryLockWithRetry`: Configurable retries and delay
- `WithLock(fn)`: Acquire -> execute -> release pattern with defer

### 9.2 Event Publishing

**File**: `internal/services/data/redis/events.go`

- Redis Streams (`XADD`) for usage and budget events
- Max 10,000 events per stream (approximate trimming)
- Streams: `usage_events`, `budget_events`

### 9.3 Distributed Latency Tracking

**File**: `internal/services/data/redis/latency_tracker.go`

- Sorted sets with timestamp scores
- 5-minute sliding window, max 1000 samples
- Percentile calculation (P50, P95, P99) via sorted set indexing
- Health score: 100 for < 500ms P95, degrades linearly
- EMA update: `new = old * 0.9 + current * 0.1`

---

## 10. Scaling Assessment

### What Works for Horizontal Scaling

| Component | Scales? | Mechanism | Notes |
|-----------|---------|-----------|-------|
| Rate Limiting (Redis) | **Yes** | Shared Redis sorted sets | Sliding window is distributed |
| Rate Limiting (Memory) | **No** | Per-process buckets | Each instance has its own counters |
| Response Cache | **Yes** | Shared Redis | Same key = same cache entry |
| Budget Cache | **Yes** | Shared Redis | Atomic INCRBYFLOAT |
| Usage Queue | **Yes** | Redis list | Multiple workers can dequeue |
| Health Store | **Yes** | Shared Redis | Cross-instance health aggregation |
| Distributed Locks | **Yes** | Redis SETNX | Prevents concurrent operations |
| Event Streams | **Yes** | Redis Streams | Multiple consumers supported |
| Routing Strategy | **Partial** | Depends on strategy | See below |
| Health Tracker (memory) | **No** | Per-process atomics | Each instance tracks independently |

### Routing Strategy Scaling

| Strategy | Scales Horizontally? | Why |
|----------|---------------------|-----|
| Priority | **Yes** | Stateless, deterministic |
| Least-Latency (Redis) | **Partial** | Distributed data but queries by model name not instance ID |
| Least-Latency (Memory) | **No** | Per-process latency data |
| Weighted Round-Robin | **No** | Atomic counter is per-process; two gateway instances will both maintain counter=0,1,2... independently |
| Random | **Yes** | Stateless |

### Critical Scaling Issues

1. **Round-Robin counter is not distributed**: `atomic.Uint64` counters in `ModelRegistry` and `RouteEntry` are per-process. With 2 gateway instances, each maintains its own counter. This means weighted distribution ratios are only correct within a single process. Across processes, the distribution is effectively random-within-weight-groups.

2. **In-memory health tracking is not shared**: Instance health (`Healthy`, `FailureCount`) is per-process. If gateway A sees 3 failures and marks an instance unhealthy, gateway B still considers it healthy until its own health checker runs. The Redis `HealthStore` records results but the routing code reads from in-memory atomics, not from Redis.

3. **Latency-based routing conflates instances**: Distributed latency keys are `pllm:latency:{modelName}`, not per-instance. When selecting between instances of the same model, the Redis path returns the same latency for all of them.

4. **Rate limit fail-open on Redis error**: If Redis goes down, all rate limiting is disabled (requests allowed through). This is a deliberate design choice for availability but means rate limits disappear during Redis outages.

### Lite Mode vs Full Mode

| Feature | Lite Mode | Full Mode |
|---------|-----------|-----------|
| LLM Proxy | Yes | Yes |
| Load Balancing | Yes | Yes |
| Routing Strategies | Yes (in-memory only) | Yes (with distributed features) |
| Health Checks | Yes (in-memory) | Yes (in-memory + Redis store) |
| Rate Limiting | In-memory (token bucket) | Redis (sliding window) |
| Response Cache | In-memory | Redis |
| Budget Enforcement | No | Yes |
| Usage Tracking | No | Yes (async queue) |
| User Management | No | Yes |
| Distributed Locks | No | Yes |

Lite mode detection is automatic: try PostgreSQL and Redis connections with 5s timeout. Can be forced with `PLLM_LITE_MODE=true`.

---

## 11. Known Gaps & Improvement Areas

### Not Implemented (stubs/fields exist)

| Feature | Evidence | Location |
|---------|----------|----------|
| Circuit Breaker | Fields `CircuitState`, `LastCircuitCheck` defined, no transition logic | types.go:37-38 |
| Instance-level weighted RR | TODO comment: "Implement weight support (currently ignores weights)" | roundrobin.go:30 |
| Instance-level rate limit enforcement | `CheckRateLimit()` exists but is never called in the selection/routing path | metrics_collector.go:99 |
| Cohere provider | Switch case stub | model_registry.go |
| HuggingFace provider | Switch case stub | model_registry.go |
| Custom provider | Switch case stub | model_registry.go |
| Cache hit/miss stats | `GetStats()` has TODO for hits/misses | cache.go |

### Behavioral Issues

1. **Health recovery is slow**: An instance marked unhealthy can only recover via the background health checker (default 30s interval). There's no probing or half-open circuit breaker to test recovery faster.

2. **Fallback loop guard is imprecise**: `len(failovers) > 10` counts instance-level failures too, not just model-level fallbacks. A model with 5 instances already produces 5 failover entries, leaving room for only 1 model-level fallback before hitting the limit.

3. **FixedWindowLimiter is unused**: Fully implemented (limiter.go:229-307) but the middleware always creates `RedisLimiter` or `InMemoryLimiter`. No code path instantiates `FixedWindowLimiter`.

4. **Optimistic budget can overspend**: On cache miss, requests are allowed and budget is checked asynchronously. During the 5-minute cache TTL, spending can exceed limits.

5. **No per-user routing awareness**: Routing strategies operate on model instances without considering which user/team is making the request. There's no per-tenant routing or isolation.

### Configuration Inconsistencies

The `RouterSettings.RoutingStrategy` field comment lists values that don't match the implemented strategy names:
```go
// Comment says: "simple", "least-busy", "usage-based", "latency-based", "priority", "weighted"
// Actual valid names: "priority", "least-latency", "weighted-round-robin", "random"
```

The `ModelFallbacks map[string]string` supports single fallback per model, while `Fallbacks map[string][]string` supports arrays. Both exist in the config struct but `ExecuteWithFailover` only uses `ModelFallbacks` (the single-value map).

---

## Summary

| Strategy | Instance-Level | Route-Level | Distributed | Production Ready |
|----------|---------------|-------------|-------------|-----------------|
| Priority | Working | Working | N/A (stateless) | Yes |
| Least-Latency | Partial (see issue #3) | Working | Partial | Needs fix for instance-level |
| Weighted Round-Robin | **Not weighted** (simple RR) | Working (smooth WRR) | No (per-process counter) | Route-level only |
| Random | Working | Working | N/A (stateless) | Yes |
| Failover (instance) | Working | Working | No (per-process health) | Yes (single instance) |
| Failover (model) | Working | Working | No (per-process health) | Yes (single instance) |
| Circuit Breaker | Not implemented | N/A | N/A | No |
| Rate Limiting (HTTP) | Working | N/A | Yes (Redis) | Yes |
| Rate Limiting (instance) | Exists, not enforced | N/A | No | No |
| Response Cache | Working | N/A | Yes (Redis) | Yes |
| Budget Enforcement | Working (optimistic) | N/A | Yes (Redis) | Yes (with caveats) |
| Health Tracking | Working (3-failure threshold) | N/A | Partial (store only) | Yes (single instance) |
