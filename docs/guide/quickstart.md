# Quick Start Guide

Get pllm running in minutes with Docker Compose or build from source.

## Docker Compose (Recommended)

The fastest way to get started with full observability stack:

```bash
# Clone the repository
git clone https://github.com/amerfu/pllm.git
cd pllm

# Copy example environment file
cp .env.example .env

# Edit with your API keys (required)
# Set at minimum: OPENAI_API_KEY, PLLM_MASTER_KEY
nano .env

# Start all services (pllm, postgres, redis, dex, grafana, etc.)
docker-compose up -d
```

Your gateway will be available at:
- API: `http://localhost:8080`
- Admin UI: `http://localhost:8080/ui`
- Docs: `http://localhost:8080/docs`
- Grafana: `http://localhost:3001` (admin/admin)
- Prometheus: `http://localhost:9091`

## Build from Source

For development:

```bash
# Prerequisites: Go 1.23+, PostgreSQL, Redis
make dev        # Start with hot reload
make build      # Build binary
make test       # Run tests
```

## First API Request

Test with OpenAI-compatible API:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  -d '{
    "model": "my-gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## Admin Interface

Access the web UI at `http://localhost:8080/ui` to:

- Monitor API usage and costs
- Manage users, teams, and API keys
- Configure model routing
- View provider health status

## Next Steps

- [Understand the architecture](/guide/architecture)
- [Configure providers](/providers)
- [Set up authentication](/auth)
- [Deploy to production](/deployment)