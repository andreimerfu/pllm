"""
Minimal reference agent for pllm Phase 2.

Satisfies the pod contract:
  POST /invoke
    request:  { "input": <any>, "task_id": "<string>" }
    response: { "output": <any>, "task_id": "..." }

Runs on :8000 under a non-root UID so it matches the SecurityContext
the AgentK8sAdapter applies (runAsNonRoot, runAsUser=10001).

Also exposes /healthz — not required by the contract today, but cheap
insurance for operators adding real probes later.
"""

import os
from flask import Flask, request, jsonify

app = Flask(__name__)

AGENT_NAME = os.environ.get("PLLM_AGENT_NAME", "echo")
USE_CASE = os.environ.get("PLLM_USE_CASE", "")


@app.get("/healthz")
def healthz():
    return "", 200


@app.post("/invoke")
def invoke():
    body = request.get_json(force=True, silent=True) or {}
    user_input = body.get("input")
    task_id = body.get("task_id", "")
    return jsonify(
        output={
            "echo": user_input,
            "from_agent": AGENT_NAME,
            "bound_use_case": USE_CASE or None,
        },
        task_id=task_id,
    )


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.environ.get("PORT", "8000")))
