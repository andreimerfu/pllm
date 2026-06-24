"""Risk-scorer role — implemented as a LangGraph StateGraph.

Three-node graph: fetch the registered prompt template, render it with
the customer/amount payload, then call the LLM. Each node emits its own
OTel span so the dependency graph in pllm shows the LangGraph workflow
clearly.

The resource attribute `pllm.agent.framework=langgraph` is set in
common.setup_otel() so the registry/compliance UI renders the LangGraph
vendor logo for this agent.
"""

from __future__ import annotations

import time
from typing import TypedDict

from opentelemetry.trace import SpanKind

import pllm_sdk

from .common import RoleContext, step


FRAMEWORK = "langgraph"


class RiskState(TypedDict, total=False):
    template: str
    rendered: str
    score: str


def _fetch_node(ctx: RoleContext):
    def fn(state: RiskState) -> RiskState:
        with ctx.tracer.start_as_current_span("langgraph.node.fetch_prompt", kind=SpanKind.INTERNAL):
            try:
                spec = pllm_sdk.get_prompt(ctx.prompt_name, ctx.prompt_version)
                tmpl = spec.get("template", "")
                print(f"  [fetch] template ({len(tmpl)} chars) from registry")
                return {**state, "template": tmpl}
            except pllm_sdk.PllmPolicyViolation as e:
                print(f"  [fetch] BLOCKED by policy: {e}")
                return {**state, "template": ""}
    return fn


def _render_node(ctx: RoleContext):
    def fn(state: RiskState) -> RiskState:
        with ctx.tracer.start_as_current_span("langgraph.node.render", kind=SpanKind.INTERNAL):
            tmpl = state.get("template") or ""
            if not tmpl:
                return {**state, "rendered": ""}
            rendered = pllm_sdk.render_prompt(tmpl, customer="alice", amount="4 USD")
            print(f"  [render] rendered locally ({len(rendered)} chars)")
            return {**state, "rendered": rendered}
    return fn


def _score_node(ctx: RoleContext):
    def fn(state: RiskState) -> RiskState:
        rendered = state.get("rendered") or ""
        if not rendered:
            return {**state, "score": "skipped (no template)"}
        with ctx.tracer.start_as_current_span("langgraph.node.score", kind=SpanKind.INTERNAL):
            sp = ctx.llm_span(ctx.allowed_model)
            try:
                resp = ctx.client.chat.completions.create(
                    model=ctx.allowed_model,
                    messages=[{"role": "user", "content": rendered}],
                    max_tokens=16,
                )
                score = resp.choices[0].message.content or ""
                print(f"  [score] -> {score!r}")
                return {**state, "score": score}
            except Exception as e:
                print(f"  [score] error: {type(e).__name__}: {e}")
                return {**state, "score": "error"}
            finally:
                sp.end()
    return fn


def _run_with_langgraph(ctx: RoleContext) -> None:
    from langgraph.graph import StateGraph, START, END

    g = StateGraph(RiskState)
    g.add_node("fetch", _fetch_node(ctx))
    g.add_node("render", _render_node(ctx))
    g.add_node("score", _score_node(ctx))
    g.add_edge(START, "fetch")
    g.add_edge("fetch", "render")
    g.add_edge("render", "score")
    g.add_edge("score", END)
    graph = g.compile()

    with step(ctx, "LangGraph workflow: fetch → render → score"):
        with ctx.tracer.start_as_current_span("langgraph.invoke", kind=SpanKind.INTERNAL) as sp:
            sp.set_attribute("langgraph.graph", "risk_score")
            graph.invoke({})


def _run_fallback(ctx: RoleContext) -> None:
    """If langgraph isn't installed, do the equivalent linear flow so the
    demo still works. The framework resource attr is set regardless, so
    the UI badge stays consistent.
    """
    state: RiskState = {}
    state = _fetch_node(ctx)(state)
    state = _render_node(ctx)(state)
    state = _score_node(ctx)(state)


def run(ctx: RoleContext) -> None:
    try:
        _run_with_langgraph(ctx)
    except Exception as e:
        # Catch broadly: not just ImportError. langchain-openai / langgraph
        # can raise pydantic-validation or runtime errors during graph
        # build. The framework attr is already set, so the demo's UI
        # signal is preserved either way.
        print(f"  [risk-scorer] LangGraph path failed ({type(e).__name__}: {e}); using fallback flow")
        _run_fallback(ctx)
    # Linger so docker compose --abort-on-container-exit doesn't cut the
    # orchestrator off mid-pipeline.
    time.sleep(ctx.linger_seconds)
