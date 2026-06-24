"""Shared setup + helpers for every role.

Picks role-specific API key envs, builds the OTel TracerProvider with the
framework attribute baked into resource attrs, and exposes a small
`step()` context manager for the readable step-by-step demo log.
"""

from __future__ import annotations

import os
from contextlib import contextmanager
from dataclasses import dataclass

from openai import OpenAI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import SpanKind

import pllm_sdk


# Per-role key envs are written into .env by setup-multiagent-demo.sh.
# Falling back to the single-agent PLLM_* lets the same module run
# standalone with a hand-set key.
ROLE_KEY_ENV = {
    "orchestrator":       ("ORCH_API_KEY", "ORCH_KEY_ID"),
    "risk-scorer":        ("RISK_API_KEY", "RISK_KEY_ID"),
    "compliance-auditor": ("COMP_API_KEY", "COMP_KEY_ID"),
}


@dataclass
class RoleContext:
    role: str
    service_name: str
    framework: str  # empty for no-framework roles
    api_key: str
    key_id: str
    base_url: str
    qdrant_url: str
    allowed_model: str
    denied_model: str
    allowed_peer: str
    denied_peer: str
    allowed_mcp_tool: str
    denied_mcp_tool: str
    mcp_echo_message: str
    prompt_name: str
    prompt_version: str
    sdk_enforce: bool
    linger_seconds: int
    client: OpenAI
    tracer: trace.Tracer

    def llm_span(self, model: str):
        sp = self.tracer.start_span(f"openai.chat.{model}", kind=SpanKind.CLIENT)
        sp.set_attribute("gen_ai.system", "openai")
        sp.set_attribute("gen_ai.request.model", model)
        return sp


def setup_otel(service_name: str, key_id: str, framework: str, endpoint: str) -> trace.Tracer:
    attrs = {
        "service.name": service_name,
        "service.version": "0.1.0",
        "pllm.key_id": key_id,
    }
    if framework:
        # Resource attr — picked up by the discovery processor and used to
        # render vendor logos in the UI.
        attrs["pllm.agent.framework"] = framework
    resource = Resource.create(attrs)
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=f"{endpoint}/v1/traces")
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


def build_context(role: str, framework: str) -> RoleContext:
    base_url = os.environ.get("PLLM_BASE_URL", "http://localhost:8080")
    api_var, id_var = ROLE_KEY_ENV.get(role, ("PLLM_API_KEY", "PLLM_KEY_ID"))
    api_key = os.environ.get(api_var) or os.environ["PLLM_API_KEY"]
    key_id = os.environ.get(id_var) or os.environ["PLLM_KEY_ID"]
    service_name = os.environ.get("OTEL_SERVICE_NAME", role)
    otel_endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", base_url)
    sdk_enforce = os.environ.get("SDK_ENFORCE", "0") == "1"

    tracer = setup_otel(service_name, key_id, framework, otel_endpoint)

    # Always init the SDK — its gateway helpers (get_prompt, call_mcp_tool)
    # need the base URL + API key wired up regardless of enforcement.
    # SDK_ENFORCE only toggles whether the httpx patch fails-closed on
    # out-of-scope hosts; the helpers themselves are always available.
    pllm_sdk.init(base_url=base_url, api_key=api_key, fail_closed=sdk_enforce)

    client = OpenAI(api_key=api_key, base_url=f"{base_url}/v1")

    return RoleContext(
        role=role,
        service_name=service_name,
        framework=framework,
        api_key=api_key,
        key_id=key_id,
        base_url=base_url,
        qdrant_url=os.environ.get("QDRANT_URL", "http://localhost:6333"),
        allowed_model=os.environ.get("ALLOWED_MODEL", "gpt-4o"),
        denied_model=os.environ.get("DENIED_MODEL", "gpt-3.5-turbo"),
        allowed_peer=os.environ.get("ALLOWED_PEER", "risk-scorer"),
        denied_peer=os.environ.get("DENIED_PEER", "compliance-auditor"),
        allowed_mcp_tool=os.environ.get("ALLOWED_MCP_TOOL", "reg-server-everything-1-0-0__echo"),
        denied_mcp_tool=os.environ.get("DENIED_MCP_TOOL", "reg-filesystem-1-0-0__list_directory"),
        mcp_echo_message=os.environ.get("MCP_ECHO_MESSAGE", "hello from orchestrator"),
        prompt_name=os.environ.get("PROMPT_NAME", "demo/loan-risk-eval"),
        prompt_version=os.environ.get("PROMPT_VERSION", "1.0.0"),
        sdk_enforce=sdk_enforce,
        linger_seconds=int(os.environ.get("LINGER_SECONDS", "15")),
        client=client,
        tracer=tracer,
    )


@contextmanager
def step(ctx: RoleContext, label: str):
    print(f"\n--- [{ctx.service_name}] {label} ---", flush=True)
    yield
