# API Reference

PLLM provides a 100% OpenAI-compatible API, allowing you to use existing OpenAI SDKs and tools without modification.

## Base URLs

- **Main API**: `http://localhost:8080/v1/`
- **Alt API**: `http://localhost:8080/api/v1/` (API key auth)

## Authentication

### Authorization Header

```bash
Authorization: Bearer your-api-key
```

### API Key Header (Alternative)

```bash  
X-API-Key: your-api-key
```

## Chat Completions

### Create Chat Completion

**Endpoint**: `POST /v1/chat/completions`

Create a completion for chat messages with streaming support.

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "my-gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "temperature": 0.7,
    "max_tokens": 150,
    "stream": false
  }'
```

#### Request Parameters

| Parameter | Type | Required | Description |
|:----------|:-----|:---------|:------------|
| `model` | string | Yes | Model name (e.g., "my-gpt-4") |
| `messages` | array | Yes | Array of message objects |
| `temperature` | number | No | Sampling temperature (0-2) |
| `max_tokens` | integer | No | Maximum tokens to generate |
| `stream` | boolean | No | Enable streaming responses |
| `stop` | string/array | No | Stop sequences |
| `presence_penalty` | number | No | Presence penalty (-2 to 2) |
| `frequency_penalty` | number | No | Frequency penalty (-2 to 2) |
| `top_p` | number | No | Nucleus sampling parameter |
| `n` | integer | No | Number of completions to generate |
| `user` | string | No | User identifier |

#### Response Format

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "my-gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 7,
    "total_tokens": 20
  }
}
```

### Streaming Chat Completions

Set `"stream": true` to enable Server-Sent Events (SSE) streaming:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "my-gpt-4", 
    "messages": [{"role": "user", "content": "Count to 10"}],
    "stream": true
  }'
```

Streaming response format:
```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1677652288,"model":"my-gpt-4","choices":[{"index":0,"delta":{"content":"1"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1677652288,"model":"my-gpt-4","choices":[{"index":0,"delta":{"content":" 2"},"finish_reason":null}]}

data: [DONE]
```

## Legacy Completions

**Endpoint**: `POST /v1/completions`

Legacy text completion endpoint:

```bash  
curl http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "my-gpt-35-turbo",
    "prompt": "Say hello",
    "max_tokens": 50
  }'
```

## Embeddings

**Endpoint**: `POST /v1/embeddings`

Create embeddings for text:

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "text-embedding-ada-002",
    "input": "Hello world"
  }'
```

## Models

### List Models

**Endpoint**: `GET /v1/models`

List available models:

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer your-api-key"
```

Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "my-gpt-4",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    },
    {
      "id": "my-gpt-35-turbo", 
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    }
  ]
}
```

### Get Model

**Endpoint**: `GET /v1/models/{model}`

Get specific model details:

```bash
curl http://localhost:8080/v1/models/my-gpt-4 \
  -H "Authorization: Bearer your-api-key"
```

## Images

### Generate Images

**Endpoint**: `POST /v1/images/generations`

Generate images (if supported by configured models):

```bash
curl http://localhost:8080/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "prompt": "A cute cat",
    "n": 1,
    "size": "1024x1024"
  }'
```

## Audio

### Transcriptions

**Endpoint**: `POST /v1/audio/transcriptions`

Transcribe audio to text:

```bash
curl http://localhost:8080/v1/audio/transcriptions \
  -H "Authorization: Bearer your-api-key" \
  -F file="@audio.mp3" \
  -F model="whisper-1"
```

### Speech

**Endpoint**: `POST /v1/audio/speech`

Generate speech from text:

```bash
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "tts-1",
    "input": "Hello world",
    "voice": "alloy"
  }' \
  --output speech.mp3
```

## Health Checks

### Health Endpoint

**Endpoint**: `GET /health`

Basic health check (no authentication required):

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Readiness Endpoint

**Endpoint**: `GET /ready`

Readiness check including dependencies:

```bash  
curl http://localhost:8080/ready
```

### Model Statistics

**Endpoint**: `GET /v1/admin/models/stats`

Get model performance statistics (requires authentication):

```bash
curl http://localhost:8080/v1/admin/models/stats \
  -H "Authorization: Bearer your-api-key"
```

## Error Responses

All errors follow OpenAI format:

```json
{
  "error": {
    "message": "The request is invalid",
    "type": "invalid_request_error", 
    "code": "bad_request"
  }
}
```

### Error Types

- `invalid_request_error` - Invalid request parameters
- `authentication_error` - Invalid or missing API key
- `permission_error` - Insufficient permissions  
- `rate_limit_error` - Rate limit exceeded
- `server_error` - Internal server error
- `service_unavailable_error` - Service temporarily unavailable

### HTTP Status Codes

- `200` - Success
- `400` - Bad Request  
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `429` - Too Many Requests
- `500` - Internal Server Error
- `503` - Service Unavailable

## SDK Compatibility

PLLM is compatible with official OpenAI SDKs:

### Python
```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="my-gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Node.js
```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:8080/v1'
});

const response = await openai.chat.completions.create({
  model: 'my-gpt-4',
  messages: [{ role: 'user', content: 'Hello!' }]
});
```

### cURL Examples

All examples use cURL for simplicity but work with any HTTP client or OpenAI SDK by changing the base URL.