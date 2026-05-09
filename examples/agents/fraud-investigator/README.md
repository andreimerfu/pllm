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

## Run (docker compose, recommended)

The compose file starts Qdrant and the agent together. The agent
runs once, prints its three steps, then exits.

```bash
# From this directory
export PLLM_BASE_URL=http://host.docker.internal:8080   # or your minikube URL
export PLLM_API_KEY=sk-...                              # bound key's plaintext
export PLLM_KEY_ID=<uuid of that key>                   # links OTel spans

docker compose up --build agent
```

To run the agent again (e.g. to push the flagged_count past the threshold
and trigger key revocation), just `docker compose up agent` again — Qdrant
keeps its data in the `qdrant-data` volume.

## Run (host Python, no docker for the agent)

```bash
# Start Qdrant only:
docker compose up -d qdrant

python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

export PLLM_BASE_URL=http://localhost:8080
export PLLM_API_KEY=sk-...
export PLLM_KEY_ID=<uuid of that key>
export OTEL_SERVICE_NAME=fraud-investigator
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json

python main.py
```

After the first run, open `/compliance` — `fraud-investigator` appears in the
Discovered Agents card. Promote it, run the agent again, and watch the
flagged calls accumulate; after the third, the key is revoked and step 1
starts failing too.

## Why three OpenAI instrumentation packages?

The OTel ecosystem has three lineages of OpenAI auto-instrumentation:

- `opentelemetry-instrumentation-openai` (early, deprecated)
- `opentelemetry-instrumentation-openai-v2` (current upstream)
- `traceloop-sdk` (alternative)

`main.py` tries v2 first, falls back to v1, and emits manual spans in either
case so the demo works regardless of which package is installed.
