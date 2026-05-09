"""Fraud-investigator demo agent.

Emits OpenTelemetry traces to pllm at $OTEL_EXPORTER_OTLP_ENDPOINT/v1/traces.
The pllm.key_id resource attribute links spans to the API key so pllm can
look up the bound use case and apply policy.

Three steps run in sequence:

  1. Call gpt-4o via pllm gateway        -> allowed (in scope)
  2. Call gpt-3.5-turbo via pllm gateway -> hard-denied at gateway
  3. Search Qdrant directly              -> flagged after-the-fact via OTel
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
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams


SERVICE_NAME = os.environ.get("OTEL_SERVICE_NAME", "fraud-investigator")
PLLM_BASE_URL = os.environ.get("PLLM_BASE_URL", "http://localhost:8080")
PLLM_API_KEY = os.environ["PLLM_API_KEY"]
PLLM_KEY_ID = os.environ["PLLM_KEY_ID"]
QDRANT_URL = os.environ.get("QDRANT_URL", "http://localhost:6333")
OTEL_ENDPOINT = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", PLLM_BASE_URL)
ALLOWED_MODEL = os.environ.get("ALLOWED_MODEL", "gpt-4o")
DENIED_MODEL = os.environ.get("DENIED_MODEL", "gpt-3.5-turbo")


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

    # Best-effort OpenAI auto-instrumentation. The package name has
    # changed across versions; we tolerate either flavor and fall back
    # to manual spans inside call_allowed_llm if neither imports.
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
    print(f"\n--- {label} ---", flush=True)
    yield


def llm_span(tracer: trace.Tracer, model: str):
    """Manual fallback that records the gen_ai.* attrs pllm classifies on."""
    span = tracer.start_span(f"openai.chat.{model}", kind=SpanKind.CLIENT)
    span.set_attribute("gen_ai.system", "openai")
    span.set_attribute("gen_ai.request.model", model)
    return span


def call_allowed_llm(client: OpenAI, tracer: trace.Tracer):
    with step(f"1. Call ALLOWED LLM ({ALLOWED_MODEL})"):
        sp = llm_span(tracer, ALLOWED_MODEL)
        try:
            resp = client.chat.completions.create(
                model=ALLOWED_MODEL,
                messages=[
                    {"role": "user", "content": "Reply with a one-word risk label for: charge $4 in NY."},
                ],
                max_tokens=8,
            )
            print(f"  ok -> {resp.choices[0].message.content!r}")
        except Exception as e:
            print(f"  unexpected error: {type(e).__name__}: {e}")
        finally:
            sp.end()


def call_denied_llm(client: OpenAI, tracer: trace.Tracer):
    with step(f"2. Call DENIED LLM ({DENIED_MODEL}) - expect 403/4xx"):
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


def search_qdrant_unscoped(tracer: trace.Tracer):
    with step("3. Direct Qdrant search (NOT in policy) - expect flag via OTel"):
        # check_compatibility=False silences the noisy 1.17 client vs 1.12
        # server version warning; the call paths we use are unchanged.
        client = QdrantClient(url=QDRANT_URL, check_compatibility=False)
        coll = "customer-embeddings"
        existing = {c.name for c in client.get_collections().collections}
        if coll not in existing:
            client.create_collection(
                coll,
                vectors_config=VectorParams(size=4, distance=Distance.COSINE),
            )
            client.upsert(
                coll,
                points=[
                    PointStruct(
                        id=str(uuid.uuid4()),
                        vector=[0.1, 0.2, 0.3, 0.4],
                        payload={"name": "alice"},
                    )
                ],
            )
        with tracer.start_as_current_span("qdrant.search", kind=SpanKind.CLIENT) as sp:
            sp.set_attribute("db.system", "qdrant")
            sp.set_attribute("db.collection.name", coll)
            sp.set_attribute("db.operation", "query_points")
            try:
                resp = client.query_points(
                    collection_name=coll,
                    query=[0.1, 0.2, 0.3, 0.4],
                    limit=3,
                )
                hits = resp.points if hasattr(resp, "points") else resp
                print(f"  qdrant returned {len(hits)} hits (will be flagged)")
            except Exception as e:
                print(f"  qdrant error: {type(e).__name__}: {e}")


def main() -> int:
    tracer = setup_otel()
    client = OpenAI(api_key=PLLM_API_KEY, base_url=f"{PLLM_BASE_URL}/v1")

    call_allowed_llm(client, tracer)
    call_denied_llm(client, tracer)
    search_qdrant_unscoped(tracer)

    # Give the BatchSpanProcessor time to flush before exit.
    time.sleep(2)
    print("\nDone. Check pllm /compliance for discovered agent + flagged calls.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
