# OpenAI-Compatible Endpoints

pllm is a drop-in for the OpenAI API — set `base_url` and an API key, and
any OpenAI SDK works unchanged.

## Chat completions

### `POST /v1/chat/completions`

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
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

### Request parameters

| Parameter | Type | Notes |
|---|---|---|
| `model` | string | Must match a `model_name` from `model_list` |
| `messages` | array | `role: system\|user\|assistant\|tool` |
| `temperature` | number | 0–2 |
| `max_tokens` | int | Optional cap |
| `stream` | bool | SSE streaming |
| `stop` | string\|array | Stop sequences |
| `presence_penalty` | number | -2..2 |
| `frequency_penalty` | number | -2..2 |
| `top_p` | number | Nucleus sampling |
| `n` | int | Number of completions |
| `user` | string | User identifier for tracking |
| `tools` | array | OpenAI tool-calling spec |
| `tool_choice` | auto / none / specific | Force tool use |
| `response_format` | object | e.g. `{"type": "json_object"}` |

### Response

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

### Streaming

Pass `"stream": true`:

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"!"}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

## Legacy completions

### `POST /v1/completions`

```bash
curl http://localhost:8080/v1/completions \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "model": "my-gpt-35-turbo",
    "prompt": "Say hello",
    "max_tokens": 50
  }'
```

Legacy prompt→completion. Streaming works the same way.

## Embeddings

### `POST /v1/embeddings`

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "model": "text-embedding-ada-002",
    "input": "Hello world"
  }'
```

Returns `{"data": [{"embedding": [...], "index": 0}], ...}`.

`input` can be a string or array of strings (batch).

## Models

### `GET /v1/models`

Lists all configured models visible to the caller (filtered by `AllowedModels`
on the key).

```json
{
  "object": "list",
  "data": [
    { "id": "my-gpt-4", "object": "model", "owned_by": "openai" },
    { "id": "openrouter-claude", "object": "model", "owned_by": "anthropic" }
  ]
}
```

### `GET /v1/models/{model}`

Single model metadata.

## Images

### `POST /v1/images/generations`

```bash
curl http://localhost:8080/v1/images/generations \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "model": "dall-e-3",
    "prompt": "A cat astronaut",
    "n": 1,
    "size": "1024x1024"
  }'
```

Also: `POST /v1/images/edits`, `POST /v1/images/variations`.

## Audio

### `POST /v1/audio/transcriptions`

```bash
curl http://localhost:8080/v1/audio/transcriptions \
  -H "Authorization: Bearer $KEY" \
  -F file="@audio.mp3" \
  -F model="whisper-1"
```

### `POST /v1/audio/translations`

```bash
curl http://localhost:8080/v1/audio/translations \
  -H "Authorization: Bearer $KEY" \
  -F file="@french.mp3" \
  -F model="whisper-1"
```

### `POST /v1/audio/speech`

```bash
curl http://localhost:8080/v1/audio/speech \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "model": "tts-1",
    "input": "Hello world",
    "voice": "alloy"
  }' \
  --output speech.mp3
```

## Realtime

### `GET /v1/realtime` (WebSocket)

OpenAI Realtime API — bidirectional streaming for voice agents. Protocol and
message shapes match OpenAI's spec.

```
GET /v1/realtime?model=gpt-4o-realtime HTTP/1.1
Upgrade: websocket
Authorization: Bearer $KEY
```

See `internal/services/llm/realtime/` for the exact session lifecycle.

## SDK examples

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="$KEY",
    base_url="http://localhost:8080/v1",
)

resp = client.chat.completions.create(
    model="my-gpt-4",
    messages=[{"role": "user", "content": "Hi"}],
)
print(resp.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.PLLM_KEY,
  baseURL: "http://localhost:8080/v1",
});

const resp = await client.chat.completions.create({
  model: "my-gpt-4",
  messages: [{ role: "user", content: "Hi" }],
});
```

### curl + jq

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $KEY" \
  -d '{"model":"my-gpt-4","messages":[{"role":"user","content":"Hi"}]}' \
| jq -r '.choices[0].message.content'
```

## `/api/v1/*` mirror

Every route above also works under `/api/v1/*`. Use that prefix when:

- You have a client that hardcodes "api" in the path
- You want to separate API-key traffic from JWT traffic at a load balancer
  (no behavioral difference; handlers are shared)
