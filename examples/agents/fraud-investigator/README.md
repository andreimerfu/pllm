# fraud-investigator (demo agent)

A minimal Python agent that demonstrates pllm's OTel-driven discovery and
governance:

1. Calls `gpt-4o` via pllm — **allowed**
2. Calls `gpt-3.5-turbo` via pllm — **hard-denied at gateway** (use case scope)
3. Searches Qdrant directly — **flagged after the fact via OTel**, with
   a `scope.flagged` webhook firing. After 3 such flags (configurable
   via `discovery.flag_threshold`), the agent's API key is auto-revoked.

## Prerequisites

- pllm running at `http://localhost:8080` with `discovery.enabled: true`
- A UseCase that allows only `openai/gpt-4o` (or whatever you pick)
- An API key bound to that UseCase (`use_case_id` set on the key)

The included `scripts/test-otel-discovery.sh` automates this provisioning;
this directory is for the manual / demo flow.

## Run

```bash
# From this directory
docker compose up -d

python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

export PLLM_BASE_URL=http://localhost:8080
export PLLM_API_KEY=sk-...                  # the key bound to your UseCase
export PLLM_KEY_ID=<uuid of that key>       # link OTel spans to the key
export OTEL_SERVICE_NAME=fraud-investigator
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json

python main.py
```

Then open `http://localhost:8080/compliance` — you'll see `fraud-investigator`
in the Discovered Agents card. Promote it, run the agent again, and watch the
flagged calls accumulate; after the third, the key is revoked and step 1
starts failing too.

## Why three OpenAI instrumentation packages?

The OTel ecosystem has three lineages of OpenAI auto-instrumentation:

- `opentelemetry-instrumentation-openai` (early, deprecated)
- `opentelemetry-instrumentation-openai-v2` (current upstream)
- `traceloop-sdk` (alternative)

`main.py` tries v2 first, falls back to v1, and emits manual spans in either
case so the demo works regardless of which package is installed.
