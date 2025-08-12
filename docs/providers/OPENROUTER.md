# OpenRouter Provider

## Overview

PLLM includes full support for [OpenRouter](https://openrouter.ai), a unified API gateway that provides access to multiple LLM models through a single endpoint. OpenRouter aggregates models from OpenAI, Anthropic, Meta, Google, and many other providers.

## Features

- ✅ **100+ Models** - Access GPT-4, Claude, Llama, Gemini, and more through one API
- ✅ **Automatic Routing** - OpenRouter handles model availability and load balancing
- ✅ **Streaming Support** - Full streaming response support for all models
- ✅ **Cost Tracking** - Built-in usage tracking and cost management
- ✅ **Model Discovery** - Dynamic model fetching from OpenRouter's API

## Configuration

### Environment Variables

```bash
# Required
OPENROUTER_API_KEY=sk-or-v1-your-api-key-here

# Optional - for better tracking
OPENROUTER_HTTP_REFERER=https://yourapp.com
OPENROUTER_X_TITLE=Your App Name
OPENROUTER_APP_NAME=YourApp/1.0
```

### Configuration File (config.yaml)

```yaml
# Single OpenRouter model
model_list:
  - model_name: openrouter-gpt-4
    params:
      model: openai/gpt-4-turbo
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
    rpm: 200
    tpm: 50000

  # Claude via OpenRouter
  - model_name: claude-3-opus
    params:
      model: anthropic/claude-3-opus
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1

  # Llama via OpenRouter
  - model_name: llama-70b
    params:
      model: meta-llama/llama-2-70b-chat
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1

  # Mixtral via OpenRouter
  - model_name: mixtral
    params:
      model: mistralai/mixtral-8x7b-instruct
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1

# Model aliases for easy access
model_aliases:
  openrouter: ["openrouter-gpt-4", "claude-3-opus", "llama-70b", "mixtral"]
```

### Advanced Provider Configuration

For more control, you can configure OpenRouter as a provider with custom settings:

```yaml
providers:
  openrouter-main:
    type: openrouter
    api_key: ${OPENROUTER_API_KEY}
    base_url: https://openrouter.ai/api/v1
    priority: 10
    enabled: true
    models:
      - openai/gpt-4-turbo
      - openai/gpt-4
      - openai/gpt-3.5-turbo
      - anthropic/claude-3-opus
      - anthropic/claude-3-sonnet
      - anthropic/claude-3-haiku
      - meta-llama/llama-2-70b-chat
      - mistralai/mixtral-8x7b-instruct
      - google/gemini-pro
    extra:
      http_referer: "https://yourapp.com"
      x_title: "Your Application"
      app_name: "YourApp/1.0"
```

## Available Models

OpenRouter provides access to hundreds of models. Here are some popular ones:

### OpenAI Models
- `openai/gpt-4-turbo` - Latest GPT-4 Turbo
- `openai/gpt-4` - GPT-4
- `openai/gpt-3.5-turbo` - GPT-3.5 Turbo
- `openai/gpt-3.5-turbo-16k` - GPT-3.5 with 16K context

### Anthropic Models
- `anthropic/claude-3-opus` - Most capable Claude model
- `anthropic/claude-3-sonnet` - Balanced performance
- `anthropic/claude-3-haiku` - Fast and efficient
- `anthropic/claude-2.1` - Previous generation
- `anthropic/claude-instant-1.2` - Fastest Claude

### Meta Llama Models
- `meta-llama/llama-2-70b-chat` - Llama 2 70B
- `meta-llama/llama-2-13b-chat` - Llama 2 13B
- `meta-llama/llama-2-7b-chat` - Llama 2 7B
- `meta-llama/codellama-34b-instruct` - Code Llama 34B

### Google Models
- `google/gemini-pro` - Gemini Pro
- `google/gemini-pro-vision` - Gemini Pro with vision
- `google/palm-2-chat-bison` - PaLM 2

### Mistral Models
- `mistralai/mixtral-8x7b-instruct` - Mixtral 8x7B MoE
- `mistralai/mistral-7b-instruct` - Mistral 7B
- `mistralai/mistral-medium` - Mistral Medium

### Other Models
- `perplexity/pplx-70b-online` - Perplexity with web search
- `nous/nous-capybara-34b` - Nous Research Capybara
- `deepinfra/airoboros-70b` - Airoboros 70B
- `cognitivecomputations/dolphin-mixtral-8x7b` - Dolphin Mixtral

## Usage Examples

### Basic Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-pllm-api-key" \
  -d '{
    "model": "openrouter-gpt-4",
    "messages": [
      {"role": "user", "content": "Hello from OpenRouter!"}
    ]
  }'
```

### Python Example

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-pllm-api-key",
    base_url="http://localhost:8080/v1"
)

# Use any OpenRouter model configured in PLLM
response = client.chat.completions.create(
    model="claude-3-opus",  # Uses OpenRouter's Claude
    messages=[
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(response.choices[0].message.content)
```

### Streaming Example

```python
# Streaming is fully supported
stream = client.chat.completions.create(
    model="llama-70b",
    messages=[{"role": "user", "content": "Write a story"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Using Multiple Models

```python
models = ["openrouter-gpt-4", "claude-3-opus", "llama-70b"]

for model in models:
    response = client.chat.completions.create(
        model=model,
        messages=[{"role": "user", "content": "What is 2+2?"}],
        max_tokens=10
    )
    print(f"{model}: {response.choices[0].message.content}")
```

## Dynamic Model Discovery

PLLM can automatically fetch available models from OpenRouter:

```go
// The provider automatically fetches available models
provider.UpdateModels(ctx)

// List all available models
models := provider.ListModels()
```

## Headers and Tracking

OpenRouter requires specific headers for proper tracking:

### Required Headers
- **HTTP-Referer**: Identifies your application (defaults to `http://localhost:8080`)
- **Authorization**: Your OpenRouter API key

### Optional Headers
- **X-Title**: Shows in OpenRouter dashboard for tracking
- **User-Agent**: Application identifier

These are automatically set by PLLM when configured.

## Rate Limiting

OpenRouter has its own rate limits per model. PLLM respects these and can be configured with additional limits:

```yaml
model_list:
  - model_name: openrouter-gpt-4
    params:
      model: openai/gpt-4-turbo
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
    rpm: 100  # Requests per minute
    tpm: 40000  # Tokens per minute
```

## Cost Tracking

OpenRouter provides detailed cost tracking. Each response includes pricing information:

```json
{
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 200,
    "total_tokens": 300
  },
  "x-openrouter-cost": "0.0045"  // Cost in USD
}
```

## Error Handling

OpenRouter-specific errors are properly handled:

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "authentication_error",
    "code": "invalid_api_key"
  }
}
```

Common error codes:
- `invalid_api_key` - Invalid or missing API key
- `model_not_found` - Requested model not available
- `rate_limit_exceeded` - Rate limit hit
- `insufficient_credits` - Out of credits

## Best Practices

1. **Use Model Aliases**: Group OpenRouter models for easy switching
   ```yaml
   model_aliases:
     smart: ["openrouter-gpt-4", "claude-3-opus"]
     fast: ["openrouter-gpt-3.5-turbo", "claude-3-haiku"]
   ```

2. **Set Appropriate Timeouts**: Some models can be slower
   ```yaml
   timeout: 120s  # 2 minutes for complex models
   ```

3. **Configure Fallbacks**: Use PLLM's routing for resilience
   ```yaml
   router:
     fallbacks:
       openrouter-gpt-4: ["openrouter-gpt-3.5-turbo"]
       claude-3-opus: ["claude-3-sonnet", "claude-3-haiku"]
   ```

4. **Monitor Costs**: Track usage through OpenRouter dashboard

5. **Cache Responses**: Use PLLM's caching for repeated queries

## Troubleshooting

### API Key Issues
```bash
# Verify your API key is set
echo $OPENROUTER_API_KEY

# Test directly with OpenRouter
curl https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "HTTP-Referer: https://localhost:8080"
```

### Model Not Found
```bash
# List available models
curl http://localhost:8080/v1/models
```

### Rate Limiting
- Check OpenRouter dashboard for current limits
- Implement exponential backoff
- Use PLLM's built-in rate limiting

### Connection Issues
- Verify network connectivity to `openrouter.ai`
- Check firewall/proxy settings
- Ensure DNS resolution works

## Migration from Direct OpenRouter

If you're currently using OpenRouter directly, migration is simple:

1. Change your base URL:
   ```python
   # Before
   client = OpenAI(
       api_key="sk-or-v1-xxx",
       base_url="https://openrouter.ai/api/v1"
   )
   
   # After
   client = OpenAI(
       api_key="your-pllm-key",
       base_url="http://localhost:8080/v1"
   )
   ```

2. Keep the same model names or create aliases

3. Benefit from PLLM's additional features:
   - Caching
   - Rate limiting
   - Load balancing
   - Monitoring
   - Fallback routing

## Advanced Features

### Custom Headers
```yaml
extra:
  http_referer: "https://myapp.com"
  x_title: "Production App"
  custom_headers:
    X-Custom-ID: "abc123"
```

### Model-Specific Configuration
```yaml
model_list:
  - model_name: gpt4-careful
    params:
      model: openai/gpt-4-turbo
      api_key: ${OPENROUTER_API_KEY}
      api_base: https://openrouter.ai/api/v1
      temperature: 0.3  # Lower temperature for this instance
      top_p: 0.9
```

### Health Monitoring
PLLM automatically monitors OpenRouter health:
- Periodic health checks via `/models` endpoint
- Automatic failover on failures
- Health scores for routing decisions

## Support

- [OpenRouter Documentation](https://openrouter.ai/docs)
- [OpenRouter Discord](https://discord.gg/openrouter)
- [PLLM GitHub Issues](https://github.com/amerfu/pllm/issues)