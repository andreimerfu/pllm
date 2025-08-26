# PLLM Gateway Architecture

## Overview

PLLM is a high-performance, production-ready LLM gateway that provides intelligent request routing, sophisticated protection mechanisms, and real-time streaming capabilities. This document explains how our key architectural components work together to deliver fast, reliable, and cost-effective AI inference.

## Core Architecture

```mermaid
graph TB
    Client[Client Applications] --> Gateway[PLLM Gateway]
    
    subgraph "Gateway Core"
        Router[Request Router] --> Auth[Authentication]
        Auth --> RateLimit[Rate Limiter]
        RateLimit --> LoadBalancer[Load Balancer]
        LoadBalancer --> CircuitBreaker[Circuit Breaker]
        CircuitBreaker --> ModelManager[Model Manager]
    end
    
    subgraph "Provider Layer"
        ModelManager --> OpenAI[OpenAI]
        ModelManager --> Anthropic[Anthropic Claude]
        ModelManager --> Azure[Azure OpenAI]
        ModelManager --> Bedrock[AWS Bedrock]
        ModelManager --> Vertex[Google Vertex]
    end
    
    subgraph "Data Layer"
        Redis[(Redis Cache)] --> RateLimit
        Redis --> CircuitBreaker
        Redis --> UsageQueue[Usage Queue]
        PostgreSQL[(PostgreSQL)] --> Auth
        PostgreSQL --> BudgetTracker[Budget Tracker]
    end
    
    subgraph "Background Services"
        UsageQueue --> UsageProcessor[Usage Processor]
        UsageProcessor --> BudgetTracker
    end
```

## Request Lifecycle

### 1. Authentication & Authorization
```mermaid
sequenceDiagram
    participant Client
    participant Gateway
    participant Auth
    participant DB
    
    Client->>Gateway: Request with API Key
    Gateway->>Auth: Validate Key
    Auth->>DB: Lookup User/Team
    DB-->>Auth: User/Team Info
    Auth-->>Gateway: Authorized Request
    Gateway->>Gateway: Continue Processing
```

**Implementation:** `internal/handlers/auth.go`
- API key validation
- Team-based access control
- Budget checking
- Request context enrichment

### 2. Rate Limiting

Our rate limiting uses a **sliding window** approach with Redis for distributed enforcement.

```mermaid
graph LR
    Request --> RateLimiter{Rate Limiter}
    RateLimiter --> |Within Limits| Continue[Continue]
    RateLimiter --> |Exceeded| Reject[429 Too Many Requests]
    
    subgraph "Redis Sliding Window"
        Window[Time Window: 60s]
        Counter[Request Counter]
        Expiry[Auto Expiry]
    end
    
    RateLimiter --> Window
```

**Configuration Example:**
```go
// Rate limiting per API key
rateLimiter := ratelimit.NewRedisLimiter(redisClient, logger)
allowed, err := rateLimiter.AllowN(ctx, 
    fmt.Sprintf("api_key:%s", keyID), 
    1, // Number of requests
    100, // Limit per window
    time.Minute, // Window duration
)
```

**Features:**
- **Sliding window**: More accurate than fixed windows
- **Token bucket**: Smooth burst handling
- **Distributed**: Works across multiple gateway instances
- **Per-key limits**: Different limits per API key/team

### 3. Load Balancing

Intelligent request routing based on provider health, latency, and current load.

```mermaid
graph TB
    Request --> LoadBalancer{Load Balancer}
    
    subgraph "Routing Strategies"
        Priority[Priority-based]
        RoundRobin[Weighted Round-Robin]
        LeastLatency[Least Latency]
        Random[Random]
    end
    
    LoadBalancer --> Strategy{Strategy?}
    Strategy --> Priority
    Strategy --> RoundRobin
    Strategy --> LeastLatency
    Strategy --> Random
    
    subgraph "Health Tracking"
        HealthChecker[Health Checker]
        Metrics[Latency Metrics]
        Available[Available Instances]
    end
    
    Priority --> HealthChecker
    RoundRobin --> Metrics
    LeastLatency --> Metrics
```

**Implementation Details:**

```go
// Adaptive load balancing with health filtering
func (m *ModelManager) GetBestInstance(ctx context.Context, modelName string) (*ModelInstance, error) {
    instances := m.registry.GetModelInstances(modelName)
    
    // Filter healthy instances
    var healthyInstances []*ModelInstance
    for _, instance := range instances {
        if m.healthTracker.IsHealthy(instance) {
            healthyInstances = append(healthyInstances, instance)
        }
    }
    
    // Select based on strategy
    return m.selectInstanceByStrategy(ctx, modelName, healthyInstances)
}
```

**Health Scoring:**
- **Success rate**: Recent request success percentage
- **Latency**: Average response time
- **Error patterns**: Timeout, rate limit, server error tracking
- **Circuit state**: Open/closed status

### 4. Circuit Breaker

Advanced circuit breaker that monitors both failures and latency to prevent cascading failures.

```mermaid
stateDiagram-v2
    [*] --> Closed
    Closed --> Open: Failures >= Threshold OR\nSlow Requests >= Threshold
    Open --> HalfOpen: Cooldown Period Elapsed
    HalfOpen --> Closed: Success Requests >= Required
    HalfOpen --> Open: Any Failure
    
    note right of Closed
        Normal operation
        Tracking failures & latency
    end note
    
    note right of Open
        Blocking all requests
        Fast-fail responses
    end note
    
    note right of HalfOpen
        Limited test requests
        Gradual recovery
    end note
```

**Adaptive Features:**

```go
// Circuit breaker monitors both failures and latency
type AdaptiveBreaker struct {
    // Failure tracking
    failures        int
    failureThreshold int  // e.g., 5 failures
    
    // Latency tracking  
    slowRequests     int
    latencyThreshold time.Duration  // e.g., 30 seconds
    slowRequestLimit int            // e.g., 10 slow requests
    
    // State management
    state State // CLOSED, OPEN, HALF_OPEN
}

// Example: Record request outcome
breaker.RecordSuccess(latency) // Considers both success and speed
breaker.RecordFailure()        // Hard failure
breaker.RecordTimeout()        // Critical - opens immediately
```

**Protection Scenarios:**
- **Provider outages**: Fast-fail instead of waiting for timeouts
- **Latency spikes**: Detect and route around slow providers
- **Rate limiting**: Avoid hitting provider limits repeatedly
- **Gradual recovery**: Test with limited requests before full restoration

### 5. Streaming Implementation

Real-time response streaming using Server-Sent Events (SSE).

```mermaid
sequenceDiagram
    participant Client
    participant Gateway
    participant Provider
    
    Client->>Gateway: POST /chat/completions (stream: true)
    Gateway->>Gateway: Setup SSE Headers
    Gateway->>Provider: Stream Request
    
    loop Streaming Response
        Provider-->>Gateway: Response Chunk
        Gateway-->>Client: data: {"choices":[{"delta":...}]}
        Gateway->>Gateway: Flush Immediately
    end
    
    Provider-->>Gateway: [DONE]
    Gateway-->>Client: data: [DONE]
```

**Implementation:**
```go
// SSE streaming with immediate flushing
func (h *LLMHandler) handleStreamingChat(w http.ResponseWriter, r *http.Request, 
    request *providers.ChatRequest, instance *models.ModelInstance, startTime time.Time) {
    
    // Setup SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache") 
    w.Header().Set("Connection", "keep-alive")
    
    flusher := w.(http.Flusher)
    streamChan, err := instance.Provider.ChatCompletionStream(r.Context(), request)
    
    // Stream responses in real-time
    for chunk := range streamChan {
        data, _ := json.Marshal(chunk)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush() // Immediate delivery
    }
}
```

### 6. Caching Strategy

Multi-layer caching for performance and cost optimization.

```mermaid
graph TB
    Request --> CacheCheck{Cache Check}
    CacheCheck --> |Hit| CacheReturn[Return Cached Response]
    CacheCheck --> |Miss| Provider[Forward to Provider]
    Provider --> CacheStore[Store in Cache]
    CacheStore --> Response[Return Response]
    
    subgraph "Cache Layers"
        Redis[Redis Cache]
        Memory[In-Memory Cache]
        CDN[CDN/Edge Cache]
    end
    
    subgraph "Cache Keys"
        ModelKey[Model + Params]
        UserKey[User Context] 
        HashKey[Request Hash]
    end
```

**Cache Implementation:**
```go
// Smart caching with TTL and invalidation
type CacheManager struct {
    redis  *redis.Client
    memory *ristretto.Cache
}

// Cache key generation
func (c *CacheManager) GenerateKey(request *ChatRequest) string {
    // Include model, messages, temperature, etc.
    hash := sha256.Sum256([]byte(
        request.Model + 
        fmt.Sprintf("%.2f", *request.Temperature) +
        // ... other parameters
    ))
    return fmt.Sprintf("chat:%x", hash)
}

// Cache with appropriate TTL
func (c *CacheManager) Set(key string, response *ChatResponse, ttl time.Duration) {
    data, _ := json.Marshal(response)
    c.redis.Set(context.Background(), key, data, ttl)
}
```

**Cache Policies:**
- **Deterministic requests**: Long TTL (1 hour)
- **Temperature > 0.1**: Short TTL (5 minutes)
- **User-specific**: No caching
- **Streaming**: No caching (real-time required)

### 7. Usage Tracking & Analytics

Asynchronous processing pipeline for usage analytics and billing.

```mermaid
graph LR
    Request --> Handler[Request Handler]
    Handler --> Response[Send Response]
    Handler --> Queue[Usage Queue]
    
    subgraph "Async Processing"
        Queue --> Worker[Usage Worker]
        Worker --> DB[(Database)]
        Worker --> Analytics[Analytics Engine]
        Worker --> Billing[Billing System]
    end
    
    subgraph "Queue Features"
        Retry[Retry Logic]
        DLQ[Dead Letter Queue]
        Priority[Priority Processing]
    end
    
    Queue --> Retry
    Queue --> DLQ
    Queue --> Priority
```

**Usage Record Structure:**
```go
type UsageRecord struct {
    RequestID    string    `json:"request_id"`
    Timestamp    time.Time `json:"timestamp"`
    UserID       string    `json:"user_id"`
    TeamID       string    `json:"team_id"`
    Model        string    `json:"model"`
    Provider     string    `json:"provider"`
    InputTokens  int       `json:"input_tokens"`
    OutputTokens int       `json:"output_tokens"`
    TotalCost    float64   `json:"total_cost"`
    Latency      int64     `json:"latency_ms"`
}
```

## Configuration Examples

### Complete Gateway Configuration

```yaml
# config.yaml
server:
  port: 8080
  timeout: 30s
  
router:
  routing_strategy: "least-latency"  # priority-based, weighted-round-robin, least-latency, random
  circuit_breaker_enabled: true
  circuit_breaker_threshold: 5
  circuit_breaker_cooldown: 30s
  
rate_limiting:
  enabled: true
  default_rpm: 100
  burst_size: 10
  
providers:
  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    timeout: 120s
    models:
      - name: "gpt-4"
        priority: 1
        cost_per_1k_tokens: 0.03
        
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    base_url: "https://api.anthropic.com"
    models:
      - name: "claude-3-sonnet"
        priority: 2
        cost_per_1k_tokens: 0.015

redis:
  url: "${REDIS_URL}"
  pool_size: 10
  
database:
  url: "${DATABASE_URL}"
  max_connections: 20
```

### Environment Variables

```bash
# .env
DATABASE_URL=postgres://pllm:pllm@localhost:5432/pllm
REDIS_URL=redis://localhost:6379
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
AZURE_OPENAI_KEY=...
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
GOOGLE_CREDENTIALS_JSON=...
```

## Monitoring & Observability

### Key Metrics

**Request Metrics:**
- Request rate (RPM/RPS)
- Response latency (P50, P95, P99)
- Error rate by status code
- Token usage and costs

**Provider Metrics:**
- Provider health scores
- Circuit breaker states
- Latency distribution per provider
- Success/failure rates

**System Metrics:**
- Memory usage and GC pressure
- Redis connection pool health
- Database query performance
- Queue depth and processing rate

### Health Check Endpoint

```bash
# System health
GET /health

# Detailed provider status
GET /admin/models/stats

# Usage analytics
GET /admin/analytics/usage
```

## Best Practices

### 1. **Provider Configuration**
- Configure multiple providers for redundancy
- Set appropriate timeouts (30s for chat, 120s for complex tasks)
- Use priority-based routing with fallbacks

### 2. **Rate Limiting**
- Set conservative default limits
- Implement per-user/team limits
- Use burst allowances for peak usage

### 3. **Circuit Breaker Tuning**
- Start with 5 failures / 30s cooldown
- Monitor latency thresholds (adjust based on use case)
- Enable gradual recovery

### 4. **Caching Strategy**
- Cache deterministic requests aggressively
- Avoid caching personalized or streaming responses
- Set appropriate TTLs based on content volatility

### 5. **Error Handling**
- Implement proper retry logic with exponential backoff
- Use circuit breakers to prevent cascade failures
- Provide meaningful error messages to clients

## Performance Characteristics

**Typical Performance:**
- **Latency**: Sub-100ms overhead (excluding LLM processing)
- **Throughput**: 10K+ requests/second (depending on configuration)
- **Availability**: 99.9%+ (with proper redundancy)
- **Cost Optimization**: 15-30% savings through intelligent routing and caching

This architecture provides a robust, scalable foundation for LLM inference while maintaining the real-time streaming experience that users expect.