# pllm Quick Start Guide

## 🚀 Getting Started in 2 Minutes

### 1. Clone and Setup
```bash
git clone https://github.com/amerfu/pllm.git
cd pllm
cp .env.example .env
```

### 2. Configure Your API Key
Edit `.env` and add your OpenAI API key:
```env
OPENAI_API_KEY=sk-proj-your-actual-api-key-here
```

### 3. Start the Services
```bash
docker compose up -d
```

### 4. Test the API

#### Using Swagger UI
Open http://localhost:8080/swagger in your browser

#### Using cURL
```bash
# Check health
curl http://localhost:8080/health

# List models
curl http://localhost:8080/v1/models

# Chat completion
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### Using Python
```python
from openai import OpenAI

client = OpenAI(
    api_key="dummy-key",  # pllm uses the configured key
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="gpt-3.5-turbo",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

## 📊 Monitoring

- **Swagger UI**: http://localhost:8080/swagger
- **Health Check**: http://localhost:8080/health
- **Metrics**: http://localhost:9090/metrics
- **Logs**: `docker compose logs pllm -f`

## 🔧 Configuration

### Multiple API Keys
Add multiple keys for load balancing in `.env`:
```env
OPENAI_API_KEY_1=sk-proj-key-1
OPENAI_API_KEY_2=sk-proj-key-2
OPENAI_API_KEY_3=sk-proj-key-3
```

### Other Providers
```env
ANTHROPIC_API_KEY=sk-ant-your-key
AZURE_API_KEY_EAST=your-azure-key
```

## 🛑 Stopping

```bash
docker compose down
```

## 🐛 Troubleshooting

### No providers loaded
- Check your API keys in `.env`
- Ensure keys start with `sk-` and are valid
- Check logs: `docker compose logs pllm`

### Port conflicts
- Change ports in `docker-compose.yml`
- Default ports: 8080 (API), 8081 (Admin), 9090 (Metrics)

### Database issues
```bash
docker compose restart postgres
docker compose logs postgres
```

## 📚 Next Steps

- Read the full [README](README.md)
- Configure advanced settings in `config.yaml`
- Set up authentication and rate limiting
- Deploy to production with Kubernetes