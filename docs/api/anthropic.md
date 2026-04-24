# Anthropic Messages API

pllm implements Anthropic's `/v1/messages` format natively. Use this endpoint
when you want the exact Anthropic request / response shape — tool use,
prefilled assistant turns, system prompt as a parameter, thinking blocks.

## Endpoint

### `POST /v1/messages`

```bash
curl http://localhost:8080/v1/messages \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet",
    "max_tokens": 1024,
    "system": "You are a helpful assistant.",
    "messages": [
      {"role": "user", "content": "Hello, world"}
    ]
  }'
```

Headers:
- `Authorization: Bearer $KEY` (pllm key — not an Anthropic key)
- `anthropic-version: 2023-06-01` (optional; pllm accepts latest)

## Request shape

Matches Anthropic's spec:

```json
{
  "model": "claude-3-sonnet",
  "max_tokens": 1024,
  "system": "optional system prompt",
  "messages": [
    {"role": "user", "content": "hello"},
    {"role": "assistant", "content": "hi, how can I help?"},
    {"role": "user", "content": "what's the time?"}
  ],
  "temperature": 0.7,
  "top_p": 0.9,
  "top_k": 40,
  "tools": [...],
  "tool_choice": {"type": "auto"},
  "stop_sequences": ["\n\nHuman:"]
}
```

## Response

```json
{
  "id": "msg_01...",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "Hello! How can I help?"}
  ],
  "model": "claude-3-sonnet",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 12,
    "output_tokens": 6
  }
}
```

## Tool use

```json
{
  "model": "claude-3-sonnet",
  "max_tokens": 1024,
  "tools": [
    {
      "name": "get_weather",
      "description": "Get current weather",
      "input_schema": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"]
      }
    }
  ],
  "messages": [{"role": "user", "content": "weather in SF?"}]
}
```

Response when Claude picks a tool:

```json
{
  "content": [
    {"type": "text", "text": "I'll check the weather."},
    {
      "type": "tool_use",
      "id": "toolu_01...",
      "name": "get_weather",
      "input": {"city": "San Francisco"}
    }
  ],
  "stop_reason": "tool_use"
}
```

The client runs the tool and sends back `tool_result`:

```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01...",
      "content": "60°F, partly cloudy"
    }
  ]
}
```

## Streaming

Pass `"stream": true`:

```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

event: message_stop
data: {"type":"message_stop"}
```

## Using the Anthropic SDK

```python
import anthropic

client = anthropic.Anthropic(
    api_key="$KEY",
    base_url="http://localhost:8080",   # note: no /v1 — SDK adds it
)

msg = client.messages.create(
    model="claude-3-sonnet",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}],
)
print(msg.content[0].text)
```

## Which endpoint should I use — `/v1/messages` or `/v1/chat/completions`?

Both work for Anthropic models. Choose based on the SDK you're already using:

- Anthropic Python SDK → `/v1/messages`
- OpenAI Python SDK → `/v1/chat/completions` (pllm translates under the hood
  when you target an Anthropic model)

Feature parity isn't perfect — Anthropic's `thinking` content blocks and
certain tool-use edge cases are only available on `/v1/messages`.

## Scope enforcement note

The chat handler checks scope (`AllowLLM`). As of writing, **the Messages
endpoint shares infrastructure but does not re-run the scope check**. If you
rely on scope for Anthropic traffic, prefer `/v1/chat/completions`. This is
tracked as a gap — see [Gotchas](/#known-gaps-next-steps).
