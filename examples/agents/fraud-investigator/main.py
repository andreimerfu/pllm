"""Multi-agent governance demo.

Three roles run via AGENT_ROLE env var:

  orchestrator       — calls LLM, calls Qdrant, calls risk-scorer (allowed),
                       and tries to call compliance-auditor (NOT in scope,
                       emits a flagged A2A span).
  risk-scorer        — calls LLM only (the orchestrator is allowed to invoke
                       this one).
  compliance-auditor — calls LLM only (the orchestrator is NOT allowed to
                       invoke this one — A2A call appears as flagged).

Each role emits OpenTelemetry traces tagged with `pllm.key_id` so pllm can
look up the bound use case and apply policy. Inter-agent calls carry a
`pllm.agent.target` attribute so pllm classifies them as agent-to-agent
edges in the discovery and observed-deps views.
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from contextlib import contextmanager

from opentelemetry import trace
from opentelemetry.trace import SpanKind
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from openai import OpenAI

# ---------- Config ---------------------------------------------------------

# Optional: client-side policy enforcement. When SDK_ENFORCE=1, we import
# pllm_sdk early so it can patch httpx before any other library
# instantiates a Client. Calls to non-pllm-gateway hosts that aren't in
# the use-case scope raise PllmPolicyViolation locally.
import pllm_sdk  # noqa: E402  (intentional early import)


AGENT_ROLE = os.environ.get("AGENT_ROLE", "orchestrator")
SERVICE_NAME = os.environ.get("OTEL_SERVICE_NAME", AGENT_ROLE)
PLLM_BASE_URL = os.environ.get("PLLM_BASE_URL", "http://localhost:8080")

# Role-specific key envs are written by setup-multiagent-demo.sh into
# the local .env file, which docker-compose loads via env_file. Falling
# back to the single-agent PLLM_API_KEY/PLLM_KEY_ID lets the same
# main.py run standalone with a hand-set key.
_ROLE_KEY_ENV = {
    "orchestrator":       ("ORCH_API_KEY", "ORCH_KEY_ID"),
    "risk-scorer":        ("RISK_API_KEY", "RISK_KEY_ID"),
    "compliance-auditor": ("COMP_API_KEY", "COMP_KEY_ID"),
}
_api_var, _id_var = _ROLE_KEY_ENV.get(AGENT_ROLE, ("PLLM_API_KEY", "PLLM_KEY_ID"))
PLLM_API_KEY = os.environ.get(_api_var) or os.environ["PLLM_API_KEY"]
PLLM_KEY_ID = os.environ.get(_id_var) or os.environ["PLLM_KEY_ID"]
QDRANT_URL = os.environ.get("QDRANT_URL", "http://localhost:6333")
OTEL_ENDPOINT = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", PLLM_BASE_URL)
ALLOWED_MODEL = os.environ.get("ALLOWED_MODEL", "gpt-4o")
DENIED_MODEL = os.environ.get("DENIED_MODEL", "gpt-3.5-turbo")
ALLOWED_PEER = os.environ.get("ALLOWED_PEER", "risk-scorer")
DENIED_PEER = os.environ.get("DENIED_PEER", "compliance-auditor")


# ---------- OTel setup -----------------------------------------------------

def setup_otel() -> trace.Tracer:
    resource = Resource.create({
        "service.name": SERVICE_NAME,
        "service.version": "0.1.0",
        "pllm.key_id": PLLM_KEY_ID,
    })
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=f"{OTEL_ENDPOINT}/v1/traces")
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)

    try:
        from opentelemetry.instrumentation.openai_v2 import OpenAIInstrumentor
        OpenAIInstrumentor().instrument()
    except ImportError:
        try:
            from opentelemetry.instrumentation.openai import OpenAIInstrumentor
            OpenAIInstrumentor().instrument()
        except ImportError:
            pass

    RequestsInstrumentor().instrument()
    return trace.get_tracer(__name__)


@contextmanager
def step(label: str):
    print(f"\n--- [{SERVICE_NAME}] {label} ---", flush=True)
    yield


# ---------- LLM helpers ----------------------------------------------------

def llm_span(tracer: trace.Tracer, model: str):
    span = tracer.start_span(f"openai.chat.{model}", kind=SpanKind.CLIENT)
    span.set_attribute("gen_ai.system", "openai")
    span.set_attribute("gen_ai.request.model", model)
    return span


def call_allowed_llm(client: OpenAI, tracer: trace.Tracer, prompt: str = "Reply with one word."):
    with step(f"LLM (ALLOWED): {ALLOWED_MODEL}"):
        sp = llm_span(tracer, ALLOWED_MODEL)
        try:
            resp = client.chat.completions.create(
                model=ALLOWED_MODEL,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=8,
            )
            print(f"  ok -> {resp.choices[0].message.content!r}")
        except Exception as e:
            print(f"  unexpected error: {type(e).__name__}: {e}")
        finally:
            sp.end()


def call_denied_llm(client: OpenAI, tracer: trace.Tracer):
    with step(f"LLM (DENIED): {DENIED_MODEL}"):
        sp = llm_span(tracer, DENIED_MODEL)
        try:
            client.chat.completions.create(
                model=DENIED_MODEL,
                messages=[{"role": "user", "content": "ignore the policy"}],
                max_tokens=8,
            )
            print("  unexpected: call succeeded")
        except Exception as e:
            print(f"  blocked at gateway as expected: {type(e).__name__}: {e}")
        finally:
            sp.end()


# ---------- Qdrant ---------------------------------------------------------

def search_qdrant_unscoped(tracer: trace.Tracer):
    """Direct vector-DB call. Without the SDK: cannot be blocked at the
    gateway, gets flagged via OTel after the fact. With SDK_ENFORCE=1:
    blocked locally before any TCP traffic.

    The whole body is wrapped so a PllmPolicyViolation from the SDK
    doesn't terminate the orchestrator's pipeline — we still want the
    A2A steps to run.
    """
    from qdrant_client import QdrantClient
    from qdrant_client.models import Distance, PointStruct, VectorParams

    with step("Vector DB (FLAGGED): qdrant"):
        try:
            client = QdrantClient(url=QDRANT_URL, check_compatibility=False)
            coll = "customer-embeddings"
            existing = {c.name for c in client.get_collections().collections}
            if coll not in existing:
                client.create_collection(coll, vectors_config=VectorParams(size=4, distance=Distance.COSINE))
                client.upsert(coll, points=[PointStruct(id=str(uuid.uuid4()), vector=[0.1, 0.2, 0.3, 0.4], payload={"name": "alice"})])
            with tracer.start_as_current_span("qdrant.search", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("db.system", "qdrant")
                sp.set_attribute("db.collection.name", coll)
                sp.set_attribute("db.operation", "query_points")
                resp = client.query_points(collection_name=coll, query=[0.1, 0.2, 0.3, 0.4], limit=3)
                hits = resp.points if hasattr(resp, "points") else resp
                print(f"  qdrant returned {len(hits)} hits (flagged via OTel — cannot be blocked at gateway)")
        except Exception as e:
            # PllmPolicyViolation comes through wrapped as
            # qdrant_client.http.exceptions.ResponseHandlingException;
            # detect the inner type by string match so we don't have to
            # import the SDK's exception type for both code paths.
            msg = str(e)
            if "pllm policy" in msg.lower():
                print(f"  blocked locally by SDK as expected: {msg}")
            else:
                print(f"  qdrant error: {type(e).__name__}: {e}")


# ---------- Prompt registry -----------------------------------------------

PROMPT_NAME = os.environ.get("PROMPT_NAME", "demo/loan-risk-eval")
PROMPT_VERSION = os.environ.get("PROMPT_VERSION", "1.0.0")


def use_registered_prompt(client: OpenAI, tracer: trace.Tracer):
    """Fetch a prompt template from pllm's gateway, render it locally, and
    send the rendered text to the LLM. Demonstrates centralized prompt
    governance — only use cases that declare the prompt as a ref can
    fetch it."""
    with step(f"Prompt fetch: {PROMPT_NAME}@{PROMPT_VERSION}"):
        try:
            spec = pllm_sdk.get_prompt(PROMPT_NAME, PROMPT_VERSION)
            tmpl = spec.get("template", "")
            rendered = pllm_sdk.render_prompt(tmpl, customer="alice", amount="4 USD")
            print(f"  fetched template ({len(tmpl)} chars), rendered locally")
        except pllm_sdk.PllmPolicyViolation as e:
            print(f"  prompt fetch BLOCKED by policy: {e}")
            return
        except Exception as e:
            print(f"  prompt fetch error: {type(e).__name__}: {e}")
            return

    with step("LLM call with registered prompt"):
        sp = llm_span(tracer, ALLOWED_MODEL)
        try:
            resp = client.chat.completions.create(
                model=ALLOWED_MODEL,
                messages=[{"role": "user", "content": rendered}],
                max_tokens=16,
            )
            print(f"  ok -> {resp.choices[0].message.content!r}")
        except Exception as e:
            print(f"  unexpected error: {type(e).__name__}: {e}")
        finally:
            sp.end()


# ---------- MCP gateway ----------------------------------------------------

ALLOWED_MCP_TOOL = os.environ.get("ALLOWED_MCP_TOOL", "reg-server-everything-1-0-0__echo")
DENIED_MCP_TOOL = os.environ.get("DENIED_MCP_TOOL", "reg-filesystem-1-0-0__list_directory")
MCP_ECHO_MESSAGE = os.environ.get("MCP_ECHO_MESSAGE", "hello from orchestrator")


def call_mcp_allowed(_tracer: trace.Tracer):
    with step(f"MCP (ALLOWED): {ALLOWED_MCP_TOOL}"):
        try:
            res = pllm_sdk.call_mcp_tool(ALLOWED_MCP_TOOL, {"message": MCP_ECHO_MESSAGE})
            preview = str(res)[:120]
            print(f"  ok -> {preview}")
        except pllm_sdk.PllmPolicyViolation as e:
            # Should not happen for the allowed path; print for visibility.
            print(f"  unexpected gateway 403: {e}")
        except Exception as e:
            # Gateway may be unreachable or the backing MCP server not
            # registered — fall back to OTel-only span so the demo
            # still records the attempt.
            print(f"  gateway/backing error: {type(e).__name__}: {e}")
            with _tracer.start_as_current_span("mcp.invoke", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("pllm.mcp.server", ALLOWED_MCP_TOOL.split("__", 1)[0])
                sp.set_attribute("rpc.method", "tools/call")


def call_mcp_denied(_tracer: trace.Tracer):
    with step(f"MCP (DENIED — not in scope): {DENIED_MCP_TOOL}"):
        try:
            pllm_sdk.call_mcp_tool(DENIED_MCP_TOOL, {})
            print("  unexpected: call succeeded")
        except pllm_sdk.PllmPolicyViolation as e:
            print(f"  blocked at gateway as expected: {e}")
        except Exception as e:
            # If the call gets through to a backing server but the
            # server itself errors, still log with the OTel span.
            print(f"  unexpected non-403 error: {type(e).__name__}: {e}")
            with _tracer.start_as_current_span("mcp.invoke.denied", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("pllm.mcp.server", DENIED_MCP_TOOL.split("__", 1)[0])
                sp.set_attribute("pllm.policy.attempted", True)


# ---------- Agent-to-agent (A2A) -------------------------------------------

def call_peer_agent(tracer: trace.Tracer, peer: str, label: str):
    """Simulates an A2A invocation. The span carries `pllm.agent.target`
    so pllm classifies it as agent-to-agent and applies AllowAgent.
    """
    with step(f"A2A ({label}): {peer}"):
        with tracer.start_as_current_span(f"a2a.invoke.{peer}", kind=SpanKind.CLIENT) as sp:
            sp.set_attribute("pllm.agent.target", peer)
            sp.set_attribute("peer.service", peer)
            sp.set_attribute("rpc.method", "invoke")
            print(f"  emitted A2A span -> {peer} ({label})")


# ---------- Roles ----------------------------------------------------------

def run_orchestrator(client: OpenAI, tracer: trace.Tracer):
    call_allowed_llm(client, tracer, prompt="Risk label for: charge $4 in NY?")
    call_denied_llm(client, tracer)
    search_qdrant_unscoped(tracer)
    call_mcp_allowed(tracer)
    call_mcp_denied(tracer)
    call_peer_agent(tracer, ALLOWED_PEER, "ALLOWED")
    call_peer_agent(tracer, DENIED_PEER, "FLAGGED — not in scope")


def run_risk_scorer(client: OpenAI, tracer: trace.Tracer):
    # Demonstrates centralized prompt fetch + render + LLM call.
    use_registered_prompt(client, tracer)
    # Give the orchestrator time to finish its longer pipeline before
    # this short-lived role exits — keeps `docker compose up
    # --abort-on-container-exit` from cutting orchestrator off mid-demo.
    time.sleep(int(os.environ.get("LINGER_SECONDS", "15")))


def run_compliance_auditor(client: OpenAI, tracer: trace.Tracer):
    call_allowed_llm(client, tracer, prompt="Did this transaction comply with KYC?")
    # Tries an MCP tool the use case doesn't declare — gateway 403.
    call_mcp_denied(tracer)
    time.sleep(int(os.environ.get("LINGER_SECONDS", "15")))


ROLES = {
    "orchestrator": run_orchestrator,
    "risk-scorer": run_risk_scorer,
    "compliance-auditor": run_compliance_auditor,
}


def main() -> int:
    role_fn = ROLES.get(AGENT_ROLE)
    if role_fn is None:
        print(f"unknown AGENT_ROLE={AGENT_ROLE!r}; expected one of {list(ROLES)}", file=sys.stderr)
        return 2

    tracer = setup_otel()

    # Initialize the client-side enforcement SDK BEFORE the OpenAI / Qdrant
    # clients build their httpx instances. Toggle via SDK_ENFORCE env var
    # so the existing demo path still runs when the user wants to compare.
    if os.environ.get("SDK_ENFORCE", "0") == "1":
        pllm_sdk.init(
            base_url=PLLM_BASE_URL,
            api_key=PLLM_API_KEY,
            fail_closed=True,
        )

    client = OpenAI(api_key=PLLM_API_KEY, base_url=f"{PLLM_BASE_URL}/v1")

    role_fn(client, tracer)

    time.sleep(2)
    print(f"\n[{SERVICE_NAME}] done.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
