"""
Research reference agent for the pllm Phase 1+2 demo.

Exercises pllm's scope-enforcement layer by calling /v1/chat/completions
with two different model names:

  * test-model-allowed    — declared by the bound UseCase (scope passes)
  * test-model-restricted — not declared (scope denies, fires scope.denied
                            event + webhook)

The LLM routing layer will likely return an error for both (no real
provider configured for these test models), but pllm's 403 vs non-403
distinction is what matters for the compliance story. The agent's
response summarizes the two attempts so the presenter can point to
"Compliance → Scope denials" and show a matching audit row that just
landed.

Contract (Phase 2):
  POST /invoke
    request:  { "input": <any>, "task_id": "<string>" }
    response: { "output": <any>, "task_id": "..." }
"""

import json
import os
import sys
import time
from dataclasses import dataclass, asdict
from typing import Any, Optional

import requests
from flask import Flask, jsonify, request

app = Flask(__name__)

PLLM_BASE_URL = os.environ.get("PLLM_BASE_URL", "http://pllm.pllm.svc.cluster.local:8080")
PLLM_API_KEY = os.environ.get("PLLM_API_KEY", "")
AGENT_NAME = os.environ.get("PLLM_AGENT_NAME", "research")
USE_CASE = os.environ.get("PLLM_USE_CASE", "")

SESSION = requests.Session()
SESSION.headers.update({
    "Authorization": f"Bearer {PLLM_API_KEY}",
    "Content-Type": "application/json",
})


@dataclass
class ProbeResult:
    model: str
    scope_allowed: bool
    pllm_status: int
    note: str


def probe_llm(model: str, user_input: str) -> ProbeResult:
    """Call pllm's chat-completions with the given model and report what happened.

    pllm's scope layer sits in front of the LLM routing layer; if the
    bound use case doesn't declare this model, it returns 403 with a
    well-known phrase in the error message. Any other status (including
    an LLM routing error like 503 / 404 / 502) means scope passed and
    the call reached the routing layer.
    """
    url = f"{PLLM_BASE_URL.rstrip('/')}/v1/chat/completions"
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": user_input}],
        "max_tokens": 32,
    }
    try:
        r = SESSION.post(url, data=json.dumps(payload), timeout=10)
    except requests.RequestException as e:
        return ProbeResult(
            model=model,
            scope_allowed=False,
            pllm_status=0,
            note=f"transport error: {e}",
        )

    body_snippet = r.text[:200]
    body_lower = body_snippet.lower()
    # pllm's canonical scope-denial phrases. "not declared by the bound
    # use case" is emitted for LLM scope (chat/completions); "out of
    # scope" covers MCP tool scope denials for completeness. The plain
    # word "scope" is intentionally NOT a trigger — it appears in
    # unrelated contexts and would false-positive.
    is_scope_denial = r.status_code == 403 and (
        "not declared by the bound use case" in body_lower
        or "out of scope for bound use case" in body_lower
    )
    if is_scope_denial:
        return ProbeResult(
            model=model,
            scope_allowed=False,
            pllm_status=r.status_code,
            note="blocked by UseCase scope (scope.denied event fired)",
        )
    return ProbeResult(
        model=model,
        scope_allowed=True,
        pllm_status=r.status_code,
        note=f"reached routing layer (status {r.status_code}); body: {body_snippet[:120]}",
    )


@app.get("/healthz")
def healthz():
    return "", 200


@app.post("/invoke")
def invoke():
    body = request.get_json(force=True, silent=True) or {}
    user_input = body.get("input") or "summarize the last quarter"
    task_id = body.get("task_id", "")

    probes = [
        probe_llm("test-model-allowed", str(user_input)),
        probe_llm("test-model-restricted", str(user_input)),
    ]

    summary = {
        "agent": AGENT_NAME,
        "bound_use_case": USE_CASE or None,
        "input_received": user_input,
        "probes": [asdict(p) for p in probes],
        "narrative": (
            "Probed two models through pllm's chat-completions endpoint. "
            f"Scope-allowed probes: {sum(1 for p in probes if p.scope_allowed)}. "
            f"Scope-denied probes: {sum(1 for p in probes if not p.scope_allowed)}. "
            "Check the Compliance page for the scope.denied audit event that just fired."
        ),
    }
    return jsonify(output=summary, task_id=task_id)


if __name__ == "__main__":
    # Slight startup nudge so container boots resolve cleanly.
    print(f"research agent starting — base={PLLM_BASE_URL} use_case={USE_CASE}", file=sys.stderr, flush=True)
    time.sleep(0.1)
    app.run(host="0.0.0.0", port=int(os.environ.get("PORT", "8000")))
