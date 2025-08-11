# Adaptive Routing Implementation

## Overview
PLLM now includes advanced adaptive routing capabilities to handle high-load scenarios with zero failed requests. The system automatically detects slow or failing LLM providers and routes traffic to alternative providers.

## Architecture

### Core Components

#### 1. Adaptive Circuit Breaker (`internal/services/circuitbreaker/adaptive_breaker.go`)
- Tracks slow responses in addition to failures
- Opens circuit when latency exceeds threshold
- Implements half-open state for recovery testing
- Key features:
  - Failure threshold: 5 failures
  - Latency threshold: 2 seconds
  - Slow request limit: 3 before opening
  - Recovery time: 30 seconds

#### 2. Adaptive Load Balancer (`internal/services/loadbalancer/adaptive_balancer.go`)
- Performance-based model selection
- Health scoring (0-100) for each model
- Dynamic routing based on:
  - Current load (30% weight)
  - Average latency (40% weight)
  - Error rate (30% weight)
- Load shedding under extreme conditions
- Primary model preference unless significantly degraded

#### 3. Model Manager Integration
- Tracks request lifecycle (start/end)
- Records performance metrics
- Coordinates between circuit breaker and load balancer
- Maintains fallback chains

## Configuration

### Model Definition
Models are defined with user-friendly names that map to provider models:

```yaml
model_list:
  - model_name: my-gpt-4
    params:
      model: gpt-4  # Actual OpenAI model
      api_key: ${OPENAI_API_KEY}
    rpm: 60
    tpm: 90000
```

### Fallback Chains
Define automatic failover sequences:

```yaml
router:
  fallbacks:
    my-gpt-4: ["my-gpt-35-turbo"]
    my-gpt-35-turbo: ["my-gpt-35-turbo-16k"]
```

### Routing Strategies
- `latency-based`: Routes to fastest responding models
- `round-robin`: Distributes load evenly
- `weighted`: Uses configured weights
- `least-busy`: Routes to least loaded instance
- `priority`: Uses configured priority order

## How It Works

### Request Flow
1. User sends request with custom model name (e.g., "my-gpt-4")
2. Model Manager records request start
3. Adaptive Load Balancer selects best model considering:
   - Circuit breaker state
   - Current health scores
   - Load distribution
   - Fallback chains if primary is degraded
4. Request forwarded to provider with mapped model name
5. Response tracked for performance metrics
6. Health scores updated based on success/latency

### Health Score Calculation
Health scores adapt based on:
- **Success**: Score improves by 1% (max 100)
- **Fast response**: Score improves
- **Slow response**: Score degrades by 5%
- **Failure**: Score degrades by 10%
- **Timeout**: Score degrades by 50%, circuit opens immediately

### Circuit Breaker States
- **Closed**: Normal operation
- **Open**: Rejecting requests, using fallbacks
- **Half-Open**: Testing recovery with limited requests

### Load Shedding
System sheds load when:
- Total concurrent requests > 1000
- Less than 2 healthy models available
- Global failure rate > 10% (after 100 requests)

## Performance Characteristics

### Latency Tracking
- Sliding window of last 100 response times
- P95 and P99 percentile calculations
- Average response time tracking

### Adaptive Behavior
- Primary model preferred unless health < 50%
- Automatic failover to healthy alternatives
- Recovery testing every 30 seconds
- Dynamic health score adjustments

## API Endpoints

### Stats Endpoint
`GET /v1/admin/models/stats`

Returns detailed statistics:
```json
{
  "load_balancer": {
    "my-gpt-4": {
      "health_score": 95.5,
      "active_requests": 2,
      "total_requests": 1024,
      "failed_requests": 12,
      "avg_latency": "450ms",
      "p95_latency": "850ms",
      "p99_latency": "1200ms",
      "circuit_open": false
    }
  },
  "adaptive_breakers": {
    "my-gpt-4": {
      "state": "closed",
      "failures": 0,
      "slow_requests": 1
    }
  },
  "should_shed_load": false
}
```

## Testing

### Test Scripts
- `scripts/test-adaptive.go`: Tests adaptive routing phases
- `scripts/test-loadbalancing.go`: Chaos testing with failures
- `scripts/test-controlled.go`: Rate-limit friendly testing
- `scripts/test-debug.go`: Model mapping verification
- `tests/adaptive_integration_test.go`: Comprehensive integration tests

### Test Scenarios
1. **High Load**: Concurrent requests trigger adaptive routing
2. **Gradual Degradation**: Traffic shifts as models degrade
3. **Cascading Failures**: Fallback chains handle multiple failures
4. **Recovery**: Circuit breakers recover after cooldown

## Implementation Notes

### Model Name Mapping
- User requests use custom names (my-gpt-4)
- System maps to provider models (gpt-4)
- Responses maintain user model names for consistency

### Known Behaviors
- OpenAI deprecated models (e.g., gpt-3.5-turbo-16k) redirect to newer versions
- Health scores persist across requests
- Circuit breakers reset after successful recovery

## Benefits

1. **Zero Failed Requests**: Automatic failover prevents user-visible failures
2. **Performance Optimization**: Routes to fastest available models
3. **Cost Efficiency**: Can route to cheaper models under normal load
4. **Resilience**: Handles provider outages gracefully
5. **Observability**: Detailed metrics for monitoring

## Future Enhancements

1. **Predictive Routing**: ML-based prediction of failures
2. **Cost-Aware Routing**: Balance performance vs cost
3. **Geographic Routing**: Route based on latency to regions
4. **Custom Health Checks**: Provider-specific health validation
5. **WebSocket Support**: Streaming with adaptive routing