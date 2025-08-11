# Adaptive Routing & High-Load Handling

## Overview

PLLM now includes advanced adaptive routing capabilities designed to handle high-load scenarios intelligently. The system can detect when LLM providers become slow or unresponsive and automatically route traffic to healthier alternatives.

## Key Features

### 1. Adaptive Circuit Breaker

Unlike traditional circuit breakers that only track failures, our adaptive circuit breaker monitors:
- **Response latency**: Detects when models become slow
- **Concurrent requests**: Tracks load on each model
- **Performance degradation**: Opens circuit proactively before failures occur

```go
// Configuration
adaptiveBreaker := circuitbreaker.NewAdaptiveBreaker(
    5,                // failure threshold
    2*time.Second,    // latency threshold
    3,                // slow request limit
)
```

States:
- **CLOSED**: Normal operation
- **OPEN**: Model is unhealthy, reject requests
- **HALF_OPEN**: Testing recovery with limited requests

### 2. Performance-Based Load Balancer

The adaptive load balancer considers real-time metrics:
- **Health Score**: 0-100 score based on recent performance
- **Response Times**: P95 and P99 latency tracking
- **Active Requests**: Current load on each model
- **Error Rate**: Recent failure percentage

```go
// Model selection algorithm
score = healthScore * (
    loadWeight * loadFactor +
    latencyWeight * latencyFactor +
    errorWeight * errorFactor
)
```

### 3. Intelligent Fallback Chains

Cross-provider fallback support with performance awareness:

```yaml
router:
  fallbacks:
    "gpt-4": ["claude-3-opus", "gpt-3.5-turbo"]
    "claude-3-opus": ["gpt-4"]
    "gpt-3.5-turbo": ["claude-3-haiku", "mistral-7b"]
```

## High-Load Behavior

### Request Flow

1. **Request arrives** → Record start in load balancer
2. **Model selection** → Choose best model based on:
   - Current health scores
   - Active request count
   - Recent latency metrics
   - Circuit breaker state
3. **Fallback on degradation** → If primary model is slow/failing:
   - Try fallback models in order
   - Skip models with open circuits
   - Select based on performance
4. **Record metrics** → Update health scores and latency data

### Load Shedding

When the system detects overload:
- Total active requests exceed threshold
- Too few healthy models available
- Global failure rate > 10%

The system will start rejecting requests early to maintain stability.

## Configuration

### Enable Adaptive Routing

```yaml
router:
  routing_strategy: "latency-based"  # or "adaptive"
  circuit_breaker_enabled: true
  circuit_breaker_threshold: 5
  circuit_breaker_cooldown: 30s
  
  # Fallback chains
  fallbacks:
    "model-a": ["model-b", "model-c"]
    "model-b": ["model-a", "model-c"]
```

### Model Configuration

```yaml
model_list:
  - id: "gpt-4-primary"
    model_name: "gpt-4"
    priority: 100
    timeout: 60s          # Max response time
    rpm: 500              # Rate limit
    cooldown_period: 30s  # Recovery time after failures
```

## Monitoring

### Performance Metrics Endpoint

```bash
GET /v1/admin/models/stats
```

Returns:
```json
{
  "load_balancer": {
    "gpt-4": {
      "health_score": 95.5,
      "active_requests": 12,
      "avg_latency": "450ms",
      "p95_latency": "1.2s",
      "p99_latency": "2.5s",
      "circuit_open": false
    }
  },
  "adaptive_breakers": {
    "gpt-4": {
      "state": "CLOSED",
      "failures": 0,
      "slow_requests": 1,
      "concurrent": 12
    }
  },
  "should_shed_load": false
}
```

### Grafana Dashboard

Monitor in real-time:
- Model health scores
- Request distribution
- Latency percentiles
- Circuit breaker states
- Fallback usage

## Load Testing

Use the included load test tool:

```bash
# Basic load test
go run scripts/load-test.go \
  -url http://localhost:8080/v1/chat/completions \
  -model gpt-4 \
  -concurrent 50 \
  -requests 1000

# Rate-limited test
go run scripts/load-test.go \
  -url http://localhost:8080/v1/chat/completions \
  -model gpt-4 \
  -concurrent 20 \
  -rps 100 \
  -duration 60s

# High-load stress test
go run scripts/load-test.go \
  -concurrent 100 \
  -requests 5000 \
  -verbose
```

## Tuning Guide

### For Low Latency

```yaml
router:
  routing_strategy: "latency-based"
  circuit_breaker_threshold: 3      # Open circuit quickly
  circuit_breaker_cooldown: 20s     # Faster recovery attempts
```

### For High Throughput

```yaml
router:
  routing_strategy: "least-busy"
  circuit_breaker_threshold: 10     # Tolerate more failures
  circuit_breaker_cooldown: 60s     # Longer cooldown
```

### For Cost Optimization

```yaml
router:
  routing_strategy: "priority"      # Use cheapest models first
  fallbacks:
    "expensive-model": ["cheap-model-1", "cheap-model-2"]
```

## Troubleshooting

### All models failing

1. Check `/v1/admin/models/stats` for circuit states
2. Verify API keys and network connectivity
3. Check if rate limits are being hit
4. Review fallback configuration

### High latency despite adaptive routing

1. Increase latency threshold in adaptive breaker
2. Add more model instances
3. Review model timeout settings
4. Check network latency to providers

### Frequent circuit breaker trips

1. Increase failure threshold
2. Adjust latency threshold for your use case
3. Add more fallback options
4. Check provider status pages

## Best Practices

1. **Configure appropriate timeouts**: Set realistic timeouts for each model
2. **Use cross-provider fallbacks**: Don't rely on a single provider
3. **Monitor health scores**: Set up alerts for degraded models
4. **Test under load**: Use the load test tool before production
5. **Progressive rollout**: Start with conservative thresholds and adjust

## Implementation Details

### Components

- `internal/services/circuitbreaker/adaptive.go`: Adaptive circuit breaker
- `internal/services/loadbalancer/adaptive_balancer.go`: Performance-based load balancer
- `internal/services/models/manager.go`: Integration with model management
- `internal/handlers/llm.go`: Request handling with adaptive routing

### Performance Impact

- **Memory**: ~1KB per model for metrics tracking
- **CPU**: <1% overhead for metric calculations
- **Latency**: <1ms routing decision time

## Future Enhancements

- [ ] Request queue with backpressure
- [ ] Predictive routing based on historical patterns
- [ ] Auto-scaling based on load
- [ ] Cost-aware routing with budget limits
- [ ] Geographic routing for latency optimization