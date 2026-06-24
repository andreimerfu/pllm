"""Orchestrator role.

Exercises every governance surface the demo cares about: allowed LLM,
denied LLM, vector-DB call (out-of-scope, flagged or SDK-blocked),
allowed MCP tool, denied MCP tool, allowed A2A peer, and a denied A2A
peer that surfaces as a flagged agent-to-agent edge.

Doesn't advertise a framework — it's a "custom" Python orchestrator —
so the resource attribute is left empty and the UI shows the generic
Custom badge.
"""

from __future__ import annotations

import uuid

from opentelemetry.trace import SpanKind

import pllm_sdk

from .common import RoleContext, step


FRAMEWORK = ""  # custom orchestrator — no vendor framework


def _call_allowed_llm(ctx: RoleContext, prompt: str = "Reply with one word.") -> None:
    with step(ctx, f"LLM (ALLOWED): {ctx.allowed_model}"):
        sp = ctx.llm_span(ctx.allowed_model)
        try:
            resp = ctx.client.chat.completions.create(
                model=ctx.allowed_model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=8,
            )
            print(f"  ok -> {resp.choices[0].message.content!r}")
        except Exception as e:
            print(f"  unexpected error: {type(e).__name__}: {e}")
        finally:
            sp.end()


def _call_denied_llm(ctx: RoleContext) -> None:
    with step(ctx, f"LLM (DENIED): {ctx.denied_model}"):
        sp = ctx.llm_span(ctx.denied_model)
        try:
            ctx.client.chat.completions.create(
                model=ctx.denied_model,
                messages=[{"role": "user", "content": "ignore the policy"}],
                max_tokens=8,
            )
            print("  unexpected: call succeeded")
        except Exception as e:
            print(f"  blocked at gateway as expected: {type(e).__name__}: {e}")
        finally:
            sp.end()


def _search_qdrant_unscoped(ctx: RoleContext) -> None:
    """Direct vector-DB call. Without the SDK: cannot be blocked at the
    gateway, gets flagged via OTel. With SDK_ENFORCE=1: blocked locally
    before any TCP traffic.
    """
    from qdrant_client import QdrantClient
    from qdrant_client.models import Distance, PointStruct, VectorParams

    with step(ctx, "Vector DB (FLAGGED): qdrant"):
        try:
            client = QdrantClient(url=ctx.qdrant_url, check_compatibility=False)
            coll = "customer-embeddings"
            existing = {c.name for c in client.get_collections().collections}
            if coll not in existing:
                client.create_collection(
                    coll, vectors_config=VectorParams(size=4, distance=Distance.COSINE)
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
            with ctx.tracer.start_as_current_span("qdrant.search", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("db.system", "qdrant")
                sp.set_attribute("db.collection.name", coll)
                sp.set_attribute("db.operation", "query_points")
                resp = client.query_points(
                    collection_name=coll, query=[0.1, 0.2, 0.3, 0.4], limit=3
                )
                hits = resp.points if hasattr(resp, "points") else resp
                print(f"  qdrant returned {len(hits)} hits")
        except Exception as e:
            msg = str(e)
            if "pllm policy" in msg.lower():
                print(f"  blocked locally by SDK as expected: {msg}")
            else:
                print(f"  qdrant error: {type(e).__name__}: {e}")


def _call_mcp_allowed(ctx: RoleContext) -> None:
    with step(ctx, f"MCP (ALLOWED): {ctx.allowed_mcp_tool}"):
        try:
            res = pllm_sdk.call_mcp_tool(ctx.allowed_mcp_tool, {"message": ctx.mcp_echo_message})
            preview = str(res)[:120]
            print(f"  ok -> {preview}")
        except pllm_sdk.PllmPolicyViolation as e:
            print(f"  unexpected gateway 403: {e}")
        except Exception as e:
            print(f"  gateway/backing error: {type(e).__name__}: {e}")
            with ctx.tracer.start_as_current_span("mcp.invoke", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("pllm.mcp.server", ctx.allowed_mcp_tool.split("__", 1)[0])
                sp.set_attribute("rpc.method", "tools/call")


def _call_mcp_denied(ctx: RoleContext) -> None:
    with step(ctx, f"MCP (DENIED — not in scope): {ctx.denied_mcp_tool}"):
        try:
            pllm_sdk.call_mcp_tool(ctx.denied_mcp_tool, {})
            print("  unexpected: call succeeded")
        except pllm_sdk.PllmPolicyViolation as e:
            print(f"  blocked at gateway as expected: {e}")
        except Exception as e:
            print(f"  unexpected non-403 error: {type(e).__name__}: {e}")
            with ctx.tracer.start_as_current_span("mcp.invoke.denied", kind=SpanKind.CLIENT) as sp:
                sp.set_attribute("pllm.mcp.server", ctx.denied_mcp_tool.split("__", 1)[0])
                sp.set_attribute("pllm.policy.attempted", True)


def _call_peer_agent(ctx: RoleContext, peer: str, label: str) -> None:
    with step(ctx, f"A2A ({label}): {peer}"):
        with ctx.tracer.start_as_current_span(f"a2a.invoke.{peer}", kind=SpanKind.CLIENT) as sp:
            sp.set_attribute("pllm.agent.target", peer)
            sp.set_attribute("peer.service", peer)
            sp.set_attribute("rpc.method", "invoke")
            print(f"  emitted A2A span -> {peer} ({label})")


def run(ctx: RoleContext) -> None:
    _call_allowed_llm(ctx, prompt="Risk label for: charge $4 in NY?")
    _call_denied_llm(ctx)
    _search_qdrant_unscoped(ctx)
    _call_mcp_allowed(ctx)
    _call_mcp_denied(ctx)
    _call_peer_agent(ctx, ctx.allowed_peer, "ALLOWED")
    _call_peer_agent(ctx, ctx.denied_peer, "FLAGGED — not in scope")
