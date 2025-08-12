---
layout: home

hero:
  name: "pLLM"
  text: "Blazing Fast LLM Gateway"
  tagline: High-performance gateway with OpenAI-compatible API, supporting multiple providers with enterprise features
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: API Reference
      link: /api/
    - theme: alt
      text: View on GitHub
      link: https://github.com/andreimerfu/pllm

features:
  - icon: 🚀
    title: High Performance
    details: Built with Go for maximum throughput and minimal latency. Handles thousands of requests per second.

  - icon: 🔄
    title: Multi-Provider Support
    details: Seamlessly integrate OpenAI, Anthropic, Google, Azure, and more through a unified API.

  - icon: ⚖️
    title: Smart Load Balancing
    details: Intelligent routing with round-robin, least-latency, weighted, and priority-based strategies.

  - icon: 🛡️
    title: Rate Limiting
    details: Protect your endpoints with configurable rate limits per user, API key, or global limits.

  - icon: 💾
    title: Response Caching
    details: Redis-backed caching for frequently used prompts to reduce costs and latency.

  - icon: 📊
    title: Comprehensive Monitoring
    details: Built-in Prometheus metrics, health checks, and detailed request logging.

  - icon: 🔐
    title: Enterprise Security
    details: JWT authentication, API key management, and role-based access control.

  - icon: 🎯
    title: Adaptive Routing
    details: Automatic failover, circuit breaking, and intelligent retry mechanisms.
---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/andreimerfu/pllm.git
cd pllm

# Run with Docker Compose
docker-compose up -d

# Or run directly with Go
go run cmd/server/main.go
```

## OpenAI Compatible API

```bash
# Use with OpenAI SDK
export OPENAI_API_BASE="http://localhost:8080/v1"
export OPENAI_API_KEY="your-api-key"

# Make a request
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Why pLLM?

- **Cost Optimization**: Route to the most cost-effective provider based on request type
- **High Availability**: Automatic failover between providers ensures uptime
- **Unified Interface**: Single API for all LLM providers
- **Enterprise Ready**: Production-grade monitoring, security, and scalability
- **Developer Friendly**: OpenAI-compatible API works with existing tools
