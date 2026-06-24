"""Governed + audited loan-advisor loop — the full control-plane story.

For each tool the agent wants to call:
  1. ask **Beacon governance** (PDP) whether the action is allowed, by agent
     identity (no policy IDs in the agent — policies are attached in Beacon,
     sourced from the pllm registry);
  2. if allowed -> execute via **pllm's MCP gateway**;
     if blocked  -> refuse;
  3. either way, report the observation to **pllm** as an OTLP discovery span,
     marking refused calls with ``pllm.policy.blocked=true``. pllm's discovery
     turns that into a ``ScopeEvent`` on the bound key's use case, so the block
     lands in the registry-side **audit trail** (and fires scope webhooks).

This closes the demo loop: registry -> policy by use case -> enforcement ->
audit trail -> external alert.

Config via env (defaults assume `kubectl port-forward` to localhost):
  BEACON_URL   Beacon backend           (default http://localhost:18000)
  BEACON_KEY   Beacon master/project    PROJECT_ID  Beacon project
  PLLM_URL     pllm gateway             (default http://localhost:8080)
  PLLM_TOKEN   pllm master key (MCP exec)
  PLLM_KEY_ID  pllm Key UUID bound to the use case (drives the audit trail)
  AGENT_NAME   canonical agent identity (default acmebank/loan-advisor)
"""

import json
import os
import time
import urllib.error
import urllib.request

BEACON_URL = os.getenv("BEACON_URL", "http://localhost:18000")
BEACON_KEY = os.getenv("BEACON_KEY", "lum_dev_vLFKxAICDPxHyLGnGQpwNbfd30C783f2p0DtxgpBOyA")
PROJECT_ID = os.getenv("PROJECT_ID", "00000000-0000-0000-0000-000000000000")
PLLM_URL = os.getenv("PLLM_URL", "http://localhost:8080")
PLLM_TOKEN = os.getenv("PLLM_TOKEN", "sk-master-dev-key-change-in-production")
PLLM_KEY_ID = os.getenv("PLLM_KEY_ID", "0fcd8a01-2325-4590-9a7f-e810b9bd89e7")
AGENT_NAME = os.getenv("AGENT_NAME", "acmebank/loan-advisor")
MCP_SLUG = os.getenv("MCP_SLUG", "lending-tools")

TOOLS = {
    "get_loan_products": {},
    "prequalify_applicant": {"applicant_id": "A-1007", "amount": 50000},
    "approve_loan": {"applicant_id": "A-1007", "amount": 50000},
    "issue_credit_decision": {"applicant_id": "A-1007", "decision": "approve"},
}


def govern(tool: str) -> dict:
    """Ask Beacon's PDP to decide on this tool action, by agent identity."""
    body = {
        "input": {
            "phase": "pre",
            "type": "tool",
            "source": {"agent_name": AGENT_NAME},
            "tool": {"name": tool},
        }
    }
    req = urllib.request.Request(
        f"{BEACON_URL}/api/v1/governance/evaluate",
        data=json.dumps(body).encode(),
        method="POST",
    )
    req.add_header("X-API-Key", BEACON_KEY)
    req.add_header("X-Project-ID", PROJECT_ID)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read().decode())


def pllm_exec(tool: str, args: dict) -> str:
    req = urllib.request.Request(
        f"{PLLM_URL}/api/admin/mcp/tools/invoke",
        data=json.dumps({"server_slug": MCP_SLUG, "tool_name": tool, "arguments": args}).encode(),
        method="POST",
    )
    req.add_header("Authorization", f"Bearer {PLLM_TOKEN}")
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read().decode())["content"][0]["text"]


def report_to_pllm(observations: list[tuple[str, bool]]) -> None:
    """Emit OTLP discovery spans to pllm so blocks land in its audit trail.

    Each span carries ``pllm.mcp.server`` (so pllm classifies it as an MCP
    edge) and, for refused calls, ``pllm.policy.blocked=true``. The resource's
    ``pllm.key_id`` ties the spans to the use-case-bound key.
    """
    from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
        ExportTraceServiceRequest,
    )
    from opentelemetry.proto.common.v1.common_pb2 import AnyValue, KeyValue
    from opentelemetry.proto.resource.v1.resource_pb2 import Resource
    from opentelemetry.proto.trace.v1.trace_pb2 import ResourceSpans, ScopeSpans, Span

    def kv(k, v):
        return KeyValue(key=k, value=AnyValue(string_value=v))

    spans = []
    for i, (tool, blocked) in enumerate(observations):
        attrs = [kv("pllm.mcp.server", MCP_SLUG)]
        if blocked:
            attrs.append(kv("pllm.policy.blocked", "true"))
        now = int(time.time() * 1e9)
        spans.append(
            Span(
                name=tool,
                trace_id=bytes([0x20 + i]) * 16,
                span_id=bytes([0x20 + i]) * 8,
                kind=Span.SPAN_KIND_CLIENT,
                start_time_unix_nano=now,
                end_time_unix_nano=now + 1_000_000,
                attributes=attrs,
            )
        )

    req = ExportTraceServiceRequest(
        resource_spans=[
            ResourceSpans(
                resource=Resource(
                    attributes=[
                        kv("service.name", AGENT_NAME),
                        kv("gen_ai.agent.name", AGENT_NAME),
                        kv("pllm.key_id", PLLM_KEY_ID),
                        kv("pllm.agent.framework", "beacon-sdk"),
                    ]
                ),
                scope_spans=[ScopeSpans(spans=spans)],
            )
        ]
    )
    r = urllib.request.Request(
        f"{PLLM_URL}/v1/traces", data=req.SerializeToString(), method="POST"
    )
    r.add_header("Content-Type", "application/x-protobuf")
    with urllib.request.urlopen(r, timeout=20) as resp:
        resp.read()


def main() -> None:
    print(f"=== governed + audited loop  (agent={AGENT_NAME}) ===\n")
    observations: list[tuple[str, bool]] = []
    for tool, args in TOOLS.items():
        d = govern(tool)
        if d.get("decision") == "allow":
            try:
                result = pllm_exec(tool, args)
                print(f"✅ ALLOW  {tool}\n          -> {result}")
            except urllib.error.HTTPError as e:
                print(f"✅ ALLOW  {tool}  (pllm exec {e.code})")
            observations.append((tool, False))
        else:
            print(f"🛑 BLOCK  {tool}\n          refused — \"{d.get('message')}\"")
            observations.append((tool, True))

    report_to_pllm(observations)
    print(
        "\n📋 reported observations to pllm — blocked calls now appear in the "
        "registry-side audit trail:\n"
        "   GET /api/admin/use-cases/events?outcome=blocked"
    )


if __name__ == "__main__":
    main()
