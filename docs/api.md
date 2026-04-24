---
layout: doc
---

# API Reference

The API docs moved — pick the category that matches what you need:

- [**Overview & common shapes**](/api/) — auth, error shapes, streaming
- [**OpenAI-compatible**](/api/openai) — `/v1/chat/completions`, embeddings, audio, images, realtime
- [**Anthropic Messages**](/api/anthropic) — `/v1/messages`
- [**MCP & Registry**](/api/mcp-registry) — `/v1/mcp`, `/v1/registry/*`, agent A2A
- [**Admin endpoints**](/api/admin) — everything under `/api/admin/*`

## Quick cheat sheet

```bash
# Chat completion
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $KEY" \
  -d '{"model":"my-gpt-4","messages":[{"role":"user","content":"hi"}]}'

# List models
curl http://localhost:8080/v1/models -H "Authorization: Bearer $KEY"

# MCP tool call
curl -X POST http://localhost:8080/v1/mcp \
  -H "Authorization: Bearer $KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Registry browse
curl "http://localhost:8080/v1/registry/servers?search=github" \
  -H "Authorization: Bearer $KEY"

# Invoke a deployed agent
curl -X POST http://localhost:8080/v1/a2a/agent-echo/tasks/send \
  -H "Authorization: Bearer $KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"input":{"text":"hi"}}}'
```

For OpenAPI/Swagger, visit `/swagger` on a running instance — it's served
directly by the gateway.
