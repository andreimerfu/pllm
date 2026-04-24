# Configuration Reference

pllm reads configuration from YAML with environment variable expansion
(`${VAR_NAME}`) and sensible defaults. This page is the complete reference.

## File discovery

Loaded in order (first wins):

1. Path from `-config` flag
2. `./config.yaml`
3. `./config/config.yaml`
4. `/etc/pllm/config.yaml`

In-repo examples:
- [`config.yaml`](https://github.com/andreimerfu/pllm/blob/main/config.yaml) — full dev config
- [`config.docker.yaml`](https://github.com/andreimerfu/pllm/blob/main/config.docker.yaml) — Docker Compose variant

## Server

```yaml
server:
  port: 8080                # HTTP listener
  host: "0.0.0.0"
  admin_port: 0             # 0 = same as port
  metrics_port: 0           # 0 = same as port; /metrics always available
  read_timeout: 60s
  write_timeout: 60s
  idle_timeout: 120s
  graceful_shutdown: 30s
  environment: ""           # "staging" / "prod" — shown as a pill in the UI
```

## Database

```yaml
database:
  url: postgres://pllm:pllm@localhost:5432/pllm?sslmode=disable
  max_open_conns: 25
  max_idle_conns: 25
  conn_max_lifetime: 300s
  auto_migrate: true        # Run AutoMigrate on startup
```

**Lite mode:** omit `database.url` to run without Postgres. Auth, keys, registry,
deployments, use cases, and webhooks are disabled.

## Redis

```yaml
redis:
  url: redis://localhost:6379
  password: ""
  db: 0
  pool_size: 10
  min_idle_conns: 5
  dial_timeout: 5s
```

**Lite mode:** omit `redis.url` to run without Redis. Rate limiting and cache
fall back to in-memory.

## LLM gateway

```yaml
model_list:
  - model_name: my-gpt-4
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}
      api_base: https://api.openai.com/v1
    rpm: 500
    tpm: 100000
    priority: 1

model_aliases:
  smart: ["my-gpt-4"]
  fast: ["my-gpt-35-turbo"]

router:
  routing_strategy: least-latency       # priority | least-latency | weighted-round-robin | random
  enable_load_balancing: true
  fallback_enabled: true
  circuit_breaker_enabled: true
  circuit_breaker_threshold: 3
  circuit_breaker_cooldown: 30s
  retry_attempts: 2
  timeout: 30s
  health_check_interval: 30s
  fallbacks:
    my-gpt-4: ["azure-gpt-4"]
  context_window_fallbacks:
    my-gpt-35-turbo: ["my-gpt-35-turbo-16k"]
```

See [LLM Overview](/llm/overview), [Routing](/guide/routing).

## MCP gateway

```yaml
mcp:
  enabled: true
  health_check_interval: 30s
  start_timeout: 30s
```

Backends are configured via the admin API / UI, not YAML — they live in the
database.

See [MCP Overview](/mcp/overview).

## Deployments

```yaml
deployment:
  enabled: true

  kubernetes:
    in_cluster: true                       # or false + kubeconfig_path for dev
    kubeconfig_path: ""
    namespace: pllm-mcp                    # MCP server workloads
    agent_namespace: pllm-agents           # agent workloads
    wrapper_image: pllm/mcp-wrapper:dev    # stdio→HTTP shim
    restrict_egress: false                 # NetworkPolicy DNS-only when true

  oom_autoscale:
    enabled: true
    multiplier: 2.0
    max_attempts: 3
    max_memory: "4Gi"
    reset_after_healthy: 30m
    poll_interval: 30s
```

See [Deployments](/deployments/overview), [K8s Adapter](/deployments/k8s),
[OOM](/deployments/oom).

## Agents

```yaml
agents:
  invoke_timeout: 60s                                              # /invoke call timeout
  public_url: "https://pllm.example.com"                           # in agent card
  internal_url: "http://pllm.pllm.svc.cluster.local:8080"          # injected into pods
```

`internal_url` defaults to `http://pllm.pllm.svc.cluster.local:8080` — that's
pllm's own Service DNS inside the cluster. Override for multi-namespace or
multi-cluster setups.

## Authentication

```yaml
jwt:
  secret_key: your-super-secret-jwt-key-change-this
  expiry: 24h
  refresh_expiry: 168h

auth:
  master_key: sk-master-dev-key-change-in-production
  require_auth: false          # true = enforce login
  allow_password_auth: true    # false = Dex only

  dex:
    enabled: false
    issuer: "http://dex:5556/dex"
    public_issuer: "http://localhost:5556/dex"
    client_id: "pllm-web"
    client_secret: "pllm-web-secret"
    redirect_url: "http://localhost:3000/auth/callback"
    scopes: ["openid", "profile", "email", "groups"]
    group_mappings:
      "admin": "administrators"
      "engineering": "default-team"

admin:
  username: admin
  password: changeme123!
  email: admin@pllm.io
```

See [Authentication](/auth), [API Keys](/security/keys).

## Performance & limits

```yaml
cache:
  enabled: true
  ttl: 300s
  max_size: 1000
  strategy: lru                # lru | lfu | fifo

rate_limit:
  enabled: true
  global_rpm: 1000
  chat_completions_rpm: 500
  completions_rpm: 500
  embeddings_rpm: 1000
  burst: 50
  cleanup_interval: 5m

cors:
  allowed_origins: ["http://localhost:3000", "https://yourdomain.com"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["*"]
  exposed_headers: []
  allow_credentials: true
  max_age: 86400
```

## Guardrails

```yaml
guardrails:
  enabled: true

  guardrails:
    - guardrail_name: pii-scrubber
      provider: presidio          # presidio | lakera | openai | aporia
      mode: pre_call              # pre_call | post_call | during_call | logging_only
      enabled: true
      default_on: true
      timeout: 5s

  providers:
    presidio:
      analyzer_url: http://presidio-analyzer:5001
      anonymizer_url: http://presidio-anonymizer:5002
      language: en
    lakera:
      api_key: ${LAKERA_API_KEY}
      base_url: https://api.lakera.ai/v1
      timeout: 3s
    openai:
      api_key: ${OPENAI_API_KEY}
      base_url: https://api.openai.com/v1
    aporia:
      api_key: ${APORIA_API_KEY}
      base_url: https://api.aporia.com
```

See [Guardrails](/security/guardrails).

## Observability

```yaml
monitoring:
  enable_metrics: true
  enable_tracing: true
  jaeger_endpoint: "http://jaeger:14268/api/traces"
  service_name: "pllm"

logging:
  level: info                  # debug | info | warn | error
  format: json                 # json | console
  output_path: ""              # empty = stdout
```

## Environment variable overrides

Any YAML key can be overridden with an env var using the snake-case path, all
caps, `_` separator. Examples:

```bash
SERVER_PORT=8080
DATABASE_URL=postgres://...
REDIS_URL=redis://...
JWT_SECRET_KEY=xxx
PLLM_MASTER_KEY=xxx
DEX_ENABLED=true
DEX_CLIENT_ID=pllm-web
LOG_LEVEL=info
MCP_ENABLED=true
DEPLOYMENT_KUBERNETES_NAMESPACE=pllm-mcp
```

Provider API keys are picked up by name inside `${VAR_NAME}` expansion in
`model_list` — any env var works. Common ones:

```bash
OPENAI_API_KEY=sk-...
OPENAI_API_KEY_1=sk-backup
ANTHROPIC_API_KEY=sk-ant-...
AZURE_API_KEY=...
OPENROUTER_API_KEY=sk-or-...
GROK_API_KEY=...
MISTRAL_API_KEY=...
```

## Hot reload

Send `SIGHUP` to the process to reload `config.yaml` without restart.
Reloadable sections:

- `model_list` and `model_aliases`
- `router.*`
- `rate_limit.*`
- `cache.*`
- `guardrails.*`

Non-reloadable (require restart):
- `server.*`
- `database.url`
- `redis.url`
- `auth.dex.*` (client config is fetched once at startup)

## Validation

On startup pllm validates configuration and:

- Reports invalid settings with actionable messages
- Uses defaults for omitted values
- Fails fast on critical misconfigurations (no DB URL in non-lite mode, etc.)
- Expands `${VAR_NAME}` everywhere, including inside API keys

Run `pllm --validate-config --config /path/to/config.yaml` to check without
starting the server.
