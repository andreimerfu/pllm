# Multi-Provider Support

PLLM supports multiple LLM providers through a unified OpenAI-compatible API interface.

## Supported Providers

Based on the codebase analysis, PLLM supports:

### OpenAI
- **Models**: GPT-4, GPT-3.5-Turbo, GPT-4 Turbo
- **Features**: Chat completions, embeddings, image generation, audio
- **Configuration**: Set `OPENAI_API_KEY` environment variable

### Anthropic Claude
- **Models**: Claude 3 (Opus, Sonnet, Haiku)
- **Features**: Chat completions
- **Implementation**: Via `/internal/services/providers/anthropic.go`

### Azure OpenAI
- **Models**: GPT-4, GPT-3.5 (Azure hosted)
- **Features**: Chat completions, embeddings
- **Configuration**: `AZURE_API_KEY` and endpoint URL

### AWS Bedrock
- **Models**: Claude, Titan, and other Bedrock models
- **Features**: Chat completions via AWS
- **Implementation**: Via `/internal/services/providers/bedrock.go`

### Google Vertex AI
- **Models**: Gemini Pro, Gemini Pro Vision
- **Features**: Chat completions, multimodal
- **Implementation**: Via `/internal/services/providers/vertex.go`

### OpenRouter
- **Models**: Access to 100+ models from multiple providers
- **Features**: Unified access to Anthropic, Meta, OpenAI, and more
- **Configuration**: See [OpenRouter Setup](providers/OPENROUTER.md)

## Configuration

Configure providers in `config.yaml`:

```yaml
# Model list - What users call in API requests
model_list:
  # GPT-4 from OpenAI
  - model_name: my-gpt-4
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}
    
  # GPT-3.5-Turbo
  - model_name: my-gpt-35-turbo
    params:
      model: gpt-3.5-turbo
      api_key: ${OPENAI_API_KEY}
    
  # Azure OpenAI deployment
  - model_name: azure-gpt-4
    params:
      model: azure/my-deployment-name
      api_base: https://my-azure-endpoint.openai.azure.com/
      api_key: ${AZURE_API_KEY}
      api_version: 2024-02-15-preview

  # OpenRouter models
  - model_name: openrouter-gpt-4
    params:
      model: openai/gpt-4-turbo
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
    rpm: 200
    tpm: 50000

  - model_name: openrouter-claude
    params:
      model: anthropic/claude-3-sonnet
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
```

## Model Aliases

Group models for easy access:

```yaml
model_aliases:
  smart: ["my-gpt-4", "azure-gpt-4", "openrouter-gpt-4"]
  fast: ["my-gpt-35-turbo"]
  claude: ["openrouter-claude"]
  openrouter: ["openrouter-gpt-4", "openrouter-claude"]
```

## Router Configuration

Configure routing strategy and fallbacks:

```yaml
router:
  routing_strategy: "latency-based"  # or "priority", "round-robin"
  circuit_breaker_enabled: true
  
  # Model fallbacks
  fallbacks:
    my-gpt-4: ["my-gpt-35-turbo"]
    my-gpt-35-turbo: ["my-gpt-35-turbo-16k"]
  
  # Context window fallbacks (when request is too large)
  context_window_fallbacks:
    my-gpt-35-turbo: ["my-gpt-35-turbo-16k"]
    my-gpt-4: ["my-gpt-4-32k"]
```

## Rate Limits

Set per-model rate limits:

```yaml
model_list:
  - model_name: fast-gpt
    params:
      model: gpt-3.5-turbo
      api_key: ${OPENAI_API_KEY}
    rpm: 500      # requests per minute
    tpm: 100000   # tokens per minute
```

## Environment Variables

Set these environment variables:

```bash
# OpenAI
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
GROQ_API_KEY_1=your-groq-key
MISTRAL_API_KEY_1=your-mistral-key
```

## Health Monitoring

The gateway automatically monitors provider health through:

- Response time tracking
- Error rate monitoring  
- Circuit breaker pattern
- Automatic failover

View provider status via:
- Admin UI: `/ui`
- API endpoint: `/v1/admin/models/stats`
- Prometheus metrics: `/metrics`