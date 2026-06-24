"""End-to-end governed-agent demo for the Retail Loan Origination use case.

Plays the `acmebank/loan-advisor` agent through a realistic session. For every
tool action it:
  1. asks **Beacon governance** to evaluate the action (allow / block), then
  2. if allowed, executes the tool via **pllm's MCP gateway**; if blocked, refuses.

This is the runtime control loop a real agent SDK (or a gateway interceptor)
would perform. No LLM required — the action sequence is scripted so the result
is deterministic and provable.

Config via env (defaults assume `kubectl port-forward` to localhost):
  PLLM_URL   (default http://localhost:18090)   PLLM_TOKEN  (pllm master key)
  BEACON_URL (default http://localhost:18000)   BEACON_KEY  (Beacon master key)
  PROJECT_ID (Beacon project)                   AGENT_NAME  (canonical identity)
"""

import json
import os
import urllib.error
import urllib.request

PLLM_URL = os.getenv("PLLM_URL", "http://localhost:18090")
PLLM_TOKEN = os.getenv("PLLM_TOKEN", "sk-master-dev-key-change-in-production")
BEACON_URL = os.getenv("BEACON_URL", "http://localhost:18000")
BEACON_KEY = os.getenv("BEACON_KEY", "lum_dev_vLFKxAICDPxHyLGnGQpwNbfd30C783f2p0DtxgpBOyA")
PROJECT_ID = os.getenv("PROJECT_ID", "00000000-0000-0000-0000-000000000000")
AGENT_NAME = os.getenv("AGENT_NAME", "acmebank/loan-advisor")
MCP_SLUG = os.getenv("MCP_SLUG", "lending-tools")


def _post(url, body, headers):
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST")
    for k, v in headers.items():
        req.add_header(k, v)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            t = r.read().decode()
            return r.status, (json.loads(t) if t else {})
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()[:300]


def govern(tool_name):
    """Ask Beacon governance whether the agent may take this tool action."""
    body = {
        # NOTE: the throwaway tag only satisfies the *current* deployed validator
        # (requires stack_ids|tags). After the validator fix ships, the agent
        # identity alone is enough and this can be dropped.
        "tags": ["__no_match__"],
        "input": {
            "phase": "pre",
            "type": "tool",
            "source": {"agent_name": AGENT_NAME},
            "tool": {"name": tool_name},
        },
    }
    s, b = _post(
        f"{BEACON_URL}/api/v1/governance/evaluate", body,
        {"X-API-Key": BEACON_KEY, "X-Project-ID": PROJECT_ID},
    )
    if isinstance(b, dict):
        return b.get("decision", "error"), b.get("message")
    return "error", str(b)


def execute(tool_name, arguments):
    """Execute the tool through pllm's MCP gateway."""
    s, b = _post(
        f"{PLLM_URL}/api/admin/mcp/tools/invoke",
        {"server_slug": MCP_SLUG, "tool_name": tool_name, "arguments": arguments},
        {"Authorization": f"Bearer {PLLM_TOKEN}"},
    )
    if isinstance(b, dict) and b.get("content"):
        return b["content"][0].get("text", json.dumps(b))
    return f"(http {s}) {json.dumps(b)[:160]}"


# A realistic loan-advisor session: gather info, attempt to auto-approve
# (must be blocked), then take the compliant human-in-the-loop path.
SESSION = [
    ("get_loan_products", {}),
    ("pull_credit_report", {"applicant_id": "A-1007"}),
    ("prequalify_applicant", {"applicant_id": "A-1007", "amount": 50000}),
    ("approve_loan", {"applicant_id": "A-1007", "amount": 50000}),       # should BLOCK
    ("submit_to_underwriting", {"applicant_id": "A-1007", "amount": 50000}),
]


def main():
    print(f"=== Governed agent run: {AGENT_NAME} ===\n")
    for tool, args in SESSION:
        decision, msg = govern(tool)
        if decision == "block":
            print(f"🛑 {tool:22} BLOCKED by governance — {msg}")
            print(f"   → agent does NOT call the tool.\n")
            continue
        if decision != "allow":
            print(f"⚠️  {tool:22} governance error: {decision} {msg}\n")
            continue
        result = execute(tool, args)
        print(f"✅ {tool:22} allowed → executed via pllm")
        print(f"   → {result}\n")


if __name__ == "__main__":
    main()
