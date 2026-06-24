"""Instrumented end-to-end loan-advisor session — Beacon tracing + pllm.

Where ``loan_advisor_e2e.py`` proves *governance* (allow/block), this proves
*observability*: it plays the ``acmebank/loan-advisor`` agent through one real
applicant session and emits a full Beacon trace so the agent's Tracing view
shows a populated session — Session ID, Input/Output, Tokens, Cost — not bare
rows.

The agent identity is set **once** at ``beacon.init`` and flows to every span
(``gen_ai.agent.*``), so the whole session is attributed to the registry-synced
agent. The session is a tree of real work:

  loan_advisor_session            (AGENT root — input: request, output: decision)
  ├─ get_loan_products            (TOOL — live pllm MCP lending-tools)
  ├─ prequalify_applicant         (TOOL — live pllm MCP lending-tools)
  └─ advisor_recommendation       (GENERATION — live LLM through pllm, real tokens)

Config via env (defaults assume `kubectl port-forward` to localhost):
  BEACON_ENDPOINT  trace-collector OTLP   (default http://localhost:14318)
  BEACON_API_KEY   project-scoped key     (or read from BEACON_API_KEY_FILE)
  PLLM_URL         pllm gateway           (default http://localhost:8080)
  PLLM_TOKEN       pllm master key
  PLLM_MODEL       model id for the LLM step
  AGENT_NAME / AGENT_ID / AGENT_VERSION   canonical agent identity
  SESSION_ID       session label          (default loan-session-A-1007)
"""

import json
import os
import urllib.request

import lumenova_beacon as beacon
from lumenova_beacon import trace  # noqa: F401  (kept for parity with other demos)
from lumenova_beacon.types import SpanType

BEACON_ENDPOINT = os.getenv("BEACON_ENDPOINT", "http://localhost:14318")
PLLM_URL = os.getenv("PLLM_URL", "http://localhost:8080")
PLLM_TOKEN = os.getenv("PLLM_TOKEN", "sk-master-dev-key-change-in-production")
PLLM_MODEL = os.getenv("PLLM_MODEL", "NVIDIA: Nemotron 3 Super (free)")
MCP_SLUG = os.getenv("MCP_SLUG", "lending-tools")

AGENT_NAME = os.getenv("AGENT_NAME", "acmebank/loan-advisor")
AGENT_ID = os.getenv("AGENT_ID", "loan-advisor")
AGENT_VERSION = os.getenv("AGENT_VERSION", "1.0.0")
SESSION_ID = os.getenv("SESSION_ID", "loan-session-A-1007")

APPLICANT_ID = os.getenv("APPLICANT_ID", "A-1007")
AMOUNT = float(os.getenv("AMOUNT", "50000"))


def _api_key() -> str:
    key = os.getenv("BEACON_API_KEY")
    if key:
        return key.strip()
    path = os.getenv("BEACON_API_KEY_FILE", "/tmp/projkey.txt")
    with open(path) as fh:
        return fh.read().strip()


def _post(url: str, body: dict, headers: dict) -> dict:
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST")
    for k, v in headers.items():
        req.add_header(k, v)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode())


def pllm_tool(tool: str, args: dict) -> str:
    """Invoke a live MCP tool through pllm's gateway; return the text result."""
    out = _post(
        f"{PLLM_URL}/api/admin/mcp/tools/invoke",
        {"server_slug": MCP_SLUG, "tool_name": tool, "arguments": args},
        {"Authorization": f"Bearer {PLLM_TOKEN}"},
    )
    return out["content"][0]["text"]


def pllm_chat(messages: list[dict]) -> tuple[str, dict]:
    """Call a real model through pllm; return (text, usage)."""
    resp = _post(
        f"{PLLM_URL}/v1/chat/completions",
        {"model": PLLM_MODEL, "messages": messages, "max_tokens": 120},
        {"Authorization": f"Bearer {PLLM_TOKEN}"},
    )
    text = resp["choices"][0]["message"]["content"] or ""
    return text, resp.get("usage", {})


def main() -> None:
    # Identity set ONCE — flows to every span's gen_ai.agent.* + the OTel
    # resource. session_id ties the spans into one session in the Tracing view.
    client = beacon.init(
        endpoint=BEACON_ENDPOINT,
        api_key=_api_key(),
        session_id=SESSION_ID,
        agent_id=AGENT_ID,
        agent_name=AGENT_NAME,
        agent_version=AGENT_VERSION,
    )

    request = (
        f"Applicant {APPLICANT_ID} requests a ${AMOUNT:,.0f} personal loan. "
        "Recommend the best product and whether they prequalify."
    )

    with client.trace("loan_advisor_session", span_type=SpanType.AGENT) as root:
        root.set_input(request)

        with client.trace("get_loan_products", span_type=SpanType.TOOL) as t1:
            t1.set_input({})
            products = pllm_tool("get_loan_products", {})
            t1.set_output(products)
            print(f"products: {products}")

        with client.trace("prequalify_applicant", span_type=SpanType.TOOL) as t2:
            t2.set_input({"applicant_id": APPLICANT_ID, "amount": AMOUNT})
            prequal = pllm_tool(
                "prequalify_applicant",
                {"applicant_id": APPLICANT_ID, "amount": AMOUNT},
            )
            t2.set_output(prequal)
            print(f"prequal: {prequal}")

        messages = [
            {
                "role": "system",
                "content": "You are a retail loan advisor. Be concise.",
            },
            {
                "role": "user",
                "content": (
                    f"{request}\nAvailable products: {products}\n"
                    f"Prequalification: {prequal}\n"
                    "Give a one-sentence recommendation."
                ),
            },
        ]
        with client.trace("advisor_recommendation", span_type=SpanType.GENERATION) as gen:
            gen.set_input(messages)
            text, usage = pllm_chat(messages)
            gen.set_output(text)
            # gen_ai usage attributes → Beacon Tokens/Cost columns.
            gen.set_attribute("gen_ai.request.model", PLLM_MODEL)
            gen.set_attribute("gen_ai.response.model", PLLM_MODEL)
            if usage:
                gen.set_attribute("gen_ai.usage.input_tokens", usage.get("prompt_tokens", 0))
                gen.set_attribute("gen_ai.usage.output_tokens", usage.get("completion_tokens", 0))
            print(f"recommendation ({usage}): {text!r}")

        decision = (
            f"{APPLICANT_ID}: recommend personal loan; "
            f"prequalification → {prequal.split('):')[-1].strip() if '):' in prequal else prequal}"
        )
        root.set_output(decision)

    client.flush()
    print(
        f"\n✅ Emitted session {SESSION_ID!r} for agent {AGENT_NAME} "
        "— check the agent's Tracing view in Beacon."
    )


if __name__ == "__main__":
    main()
