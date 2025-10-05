# Configuration

PLLM supports comprehensive configuration through YAML files, environment variables, and defaults.

## Configuration Files

PLLM looks for configuration files in this order:
1. Path specified with `-config` flag
2. `./config.yaml` (current directory)
3. `./config/config.yaml`
4. `/etc/pllm/config.yaml`

## Core Configuration

### Server Settings

```yaml
server:
  port: 8080              # Main API port
  admin_port: 8081        # Admin API port
  metrics_port: 9090      # Prometheus metrics port
  read_timeout: 30s       # Request read timeout
  write_timeout: 300s     # Response write timeout (5min for streaming)
  idle_timeout: 120s      # Keep-alive timeout
  graceful_shutdown: 30s  # Shutdown timeout
```

### Database Configuration

PostgreSQL is required for authentication and user management:

```yaml
database:
  url: postgres://pllm:pllm@localhost:5432/pllm?sslmode=disable
  max_connections: 100      # Connection pool size
  max_idle_connections: 10  # Idle connections
  conn_max_lifetime: 1h     # Connection lifetime
```

### Redis Configuration

Redis is required for caching, rate limiting, and async budget processing:

```yaml
redis:
  url: redis://localhost:6379
  password: ""              # Redis password (optional)
  db: 0                    # Redis database number
  pool_size: 100           # Connection pool size
```

## Model Configuration

### Model List

Define available models and their providers:

```yaml
model_list:
  # OpenAI GPT-4
  - model_name: my-gpt-4          # User-facing name
    params:
      model: gpt-4                # Provider model name
      api_key: ${OPENAI_API_KEY}  # Environment variable
      temperature: 0.7            # Optional default params
      max_tokens: 2000
    rpm: 500                      # Rate limit (requests/min)
    tpm: 100000                  # Token limit (tokens/min)

  # Azure OpenAI
  - model_name: azure-gpt-4
    params:
      model: azure/my-deployment-name
      api_base: https://my-resource.openai.azure.com/
      api_key: ${AZURE_API_KEY}
      api_version: 2024-02-15-preview

  # OpenRouter
  - model_name: openrouter-claude
    params:
      model: anthropic/claude-3-sonnet
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
```

### Model Aliases

Group models for easy access:

```yaml
model_aliases:
  smart: ["my-gpt-4", "azure-gpt-4"]      # High-quality models
  fast: ["my-gpt-35-turbo"]               # Fast models
  claude: ["openrouter-claude"]           # Anthropic models
  openrouter: ["openrouter-gpt-4", "openrouter-claude"]
```

### Router Configuration

Control request routing, load balancing, and failover:

```yaml
router:
  # Routing strategy (see Routing Guide for details)
  routing_strategy: "least-latency"  # priority | least-latency | weighted-round-robin | random

  # Failover settings
  fallback_enabled: true
  circuit_breaker_enabled: true
  circuit_breaker_threshold: 3       # Failures before opening circuit
  circuit_breaker_cooldown: 30s      # Cooldown before retry

  # Request settings
  retry_attempts: 2
  timeout: 30s
  health_check_interval: 30s

  # Fallback chains (model -> list of fallbacks)
  fallbacks:
    gpt-4-openai: ["gpt-4-azure", "gpt-4-openrouter"]
    claude-3-opus: ["claude-3-sonnet"]
```

::: tip
For production multi-instance deployments, use `routing_strategy: "least-latency"` with Redis to share performance metrics across pods. See [Routing Guide](/guide/routing) for details.
:::

## Authentication Configuration

### JWT Settings

```yaml
jwt:
  secret_key: your-super-secret-jwt-key-change-this
  access_token_duration: 15m    # Access token lifetime
  refresh_token_duration: 168h  # Refresh token lifetime (7 days)
```

### Dex OIDC Integration

```yaml
auth:
  master_key: sk-master-dev-key-change-in-production
  require_auth: true            # Enforce authentication

  dex:
    enabled: true              # Enable OIDC via Dex
    issuer: "http://localhost:5556/dex"          # Dex backend URL
    public_issuer: "http://localhost:5556/dex"   # Frontend OAuth URL
    client_id: "pllm-web"
    client_secret: "pllm-web-secret"
    redirect_url: "http://localhost:3000/auth/callback"
    scopes: ["openid", "profile", "email", "groups"]

    # Map Dex groups to PLLM teams
    group_mappings:
      "admin": "administrators"
      "users": "default-team"
```

## Performance & Limits

### Caching

```yaml
cache:
  enabled: true
  ttl: 3600s          # Cache TTL (1 hour)
  max_size: 1000      # Max cache entries
  strategy: "lru"     # Cache eviction strategy
```

### Rate Limiting

```yaml
rate_limit:
  enabled: true
  requests_per_minute: 600        # Global RPM limit
  burst: 10                       # Burst allowance
  cleanup_interval: 1m            # Cleanup interval

  # Per-endpoint limits
  global_rpm: 10000               # Overall system limit
  chat_completions_rpm: 5000      # Chat endpoint limit
  completions_rpm: 3000           # Completions limit
  embeddings_rpm: 2000            # Embeddings limit
```

### CORS Settings

```yaml
cors:
  allowed_origins:
    - "http://localhost:3000"
    - "https://yourdomain.com"
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["*"]
  exposed_headers: []
  allow_credentials: true
  max_age: 86400
```

## Observability

### Monitoring

```yaml
monitoring:
  enable_metrics: true                    # Prometheus metrics
  enable_tracing: true                    # Distributed tracing
  jaeger_endpoint: "http://localhost:14268/api/traces"
  service_name: "pllm"
```

### Logging

```yaml
logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json, console
  output_path: ""         # File path (empty = stdout)
```

## Environment Variables

All configuration can be overridden with environment variables:

### Server & Infrastructure
```bash
SERVER_PORT=8080
ADMIN_PORT=8081
METRICS_PORT=9090
DATABASE_URL=postgres://...
REDIS_URL=redis://...
```

### Authentication
```bash
JWT_SECRET_KEY=your-jwt-secret
PLLM_MASTER_KEY=sk-master-key
DEX_ENABLED=true
DEX_ISSUER=http://localhost:5556/dex
DEX_CLIENT_ID=pllm-web
DEX_CLIENT_SECRET=pllm-web-secret
```

### Model Providers
```bash
# OpenAI (supports multiple keys)
OPENAI_API_KEY=sk-your-key
OPENAI_API_KEY_1=sk-backup-key-1
OPENAI_API_KEY_2=sk-backup-key-2

# OpenRouter
OPENROUTER_API_KEY=sk-or-your-key
OPENROUTER_HTTP_REFERER=http://localhost:8080
OPENROUTER_X_TITLE=PLLM Gateway

# Other providers
ANTHROPIC_API_KEY_1=sk-ant-your-key
AZURE_API_KEY_EAST=your-azure-key
GROK_API_KEY_1=your-grok-key
MISTRAL_API_KEY_1=your-mistral-key
```

### Observability
```bash
LOG_LEVEL=info
LOG_FORMAT=json
ENABLE_METRICS=true
ENABLE_TRACING=true
JAEGER_ENDPOINT=http://localhost:14268/api/traces
```

## Configuration Examples

### Development Setup

See [`config.yaml`](../config.yaml) for a complete development configuration.

### Docker Setup

See [`config.docker.yaml`](../config.docker.yaml) for containerized deployments.

### Lite Setup

See [`config.lite.yaml`](../config.lite.yaml) for minimal configuration without authentication.

## Configuration Validation

PLLM validates configuration on startup and will:
- Report invalid settings
- Use sensible defaults where possible
- Fail fast on critical misconfigurations
- Expand environment variables in API keys (`${VAR_NAME}` format)

## Hot Reloading

Configuration supports runtime updates for:
- Model list changes
- Rate limit adjustments
- Router strategy changes
- Cache settings

Send `SIGHUP` signal to reload configuration without restart.
