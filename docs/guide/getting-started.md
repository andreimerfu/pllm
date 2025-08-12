# Getting Started

## Overview

pllm is a high-performance LLM gateway that provides a unified interface to multiple LLM providers. It acts as a proxy between your applications and various LLM services, offering enterprise features like load balancing, rate limiting, caching, and monitoring.

## Prerequisites

- Go 1.21+ (for building from source)
- Docker & Docker Compose (for containerized deployment)
- PostgreSQL (optional, for full feature set)
- Redis (optional, for caching)

## Installation

### Using Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/amerfu/pllm.git
cd pllm

# Copy environment template
cp .env.example .env

# Edit .env with your provider API keys
nano .env

# Start all services
docker-compose up -d
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/amerfu/pllm.git
cd pllm

# Install dependencies
go mod download

# Build the binary
make build

# Run the server
./bin/pllm-server
```

## Configuration

### Environment Variables

Create a `.env` file with your provider API keys:

```env
# OpenAI
OPENAI_API_KEY=sk-...

# Anthropic
ANTHROPIC_API_KEY=sk-ant-...

# Google Vertex AI
GOOGLE_API_KEY=...

# Azure OpenAI
AZURE_OPENAI_API_KEY=...
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
```

### YAML Configuration

Edit `config.yaml` to configure models and routing:

```yaml
models:
  - name: gpt-4
    provider: openai
    model: gpt-4
    api_key: ${OPENAI_API_KEY}
    
  - name: claude-3
    provider: anthropic
    model: claude-3-opus-20240229
    api_key: ${ANTHROPIC_API_KEY}
    
router:
  strategy: round-robin  # or: least-latency, weighted, priority
  retry:
    max_attempts: 3
    backoff: exponential
```

## Making Your First Request

Once the server is running, you can make requests using any OpenAI-compatible client:

### Using curl

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello! How are you?"}
    ]
  }'
```

### Using OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello! How are you?"}
    ]
)

print(response.choices[0].message.content)
```

### Using Node.js

```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:8080/v1'
});

const response = await openai.chat.completions.create({
  model: 'gpt-4',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'Hello! How are you?' }
  ]
});

console.log(response.choices[0].message.content);
```

## Admin UI

Access the admin interface at `http://localhost:8080/ui` to:

- Monitor request metrics
- View provider status
- Manage API keys
- Configure routing rules
- View real-time logs

## Health Checks

The gateway provides health check endpoints:

```bash
# Basic health check
curl http://localhost:8080/health

# Readiness check (includes dependency checks)
curl http://localhost:8080/ready

# Prometheus metrics
curl http://localhost:8080/metrics
```

## Next Steps

- [Configure Multiple Providers](/features/providers)
- [Set Up Load Balancing](/features/load-balancing)
- [Enable Caching](/features/caching)
- [Configure Rate Limiting](/features/rate-limiting)
- [Deploy to Production](/deployment/production)