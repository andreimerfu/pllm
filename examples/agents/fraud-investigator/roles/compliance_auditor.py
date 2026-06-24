"""Compliance-auditor role — implemented as a CrewAI Crew.

A single CrewAI Agent + Task + Crew. The crew is pointed at pllm's LLM
gateway via OPENAI_API_KEY / OPENAI_API_BASE so every model call still
goes through the gateway and is governed by the bound use case.

The resource attribute `pllm.agent.framework=crewai` is set in
common.setup_otel() so the UI renders the CrewAI vendor logo.

Also exercises the deny path: tries to call an MCP tool the use case
doesn't declare, expecting a gateway 403.
"""

from __future__ import annotations

import os

# Suppress CrewAI's interactive "view execution traces?" stdin prompt.
# CREWAI_TESTING=true is the *only* env var that short-circuits both
# `should_auto_collect_first_time_traces()` and the post-execution
# stdin prompt — checked via `_is_test_environment()` in
# crewai/events/listeners/tracing/utils.py. The first-execution flag
# is otherwise persisted to ~/.crewai_user.json which a freshly-built
# container never has, so the prompt would fire on every demo run.
# Also disable LiteLLM's GitHub model-cost-map fetch so the SDK's
# fail-closed mode doesn't have to allowlist raw.githubusercontent.com.
os.environ.setdefault("CREWAI_TESTING", "true")
os.environ.setdefault("LITELLM_LOCAL_MODEL_COST_MAP", "True")

import time

from opentelemetry.trace import SpanKind

import pllm_sdk

from .common import RoleContext, step


FRAMEWORK = "crewai"


def _audit_with_crewai(ctx: RoleContext) -> None:
    from crewai import Agent, Crew, Task

    # CrewAI uses LiteLLM under the hood. The "openai/<model>" prefix
    # tells LiteLLM to use its OpenAI provider, which honors
    # OPENAI_API_BASE / OPENAI_API_KEY — both already pointed at pllm.
    os.environ["OPENAI_API_KEY"] = ctx.api_key
    os.environ["OPENAI_API_BASE"] = f"{ctx.base_url}/v1"

    auditor = Agent(
        role="KYC Compliance Auditor",
        goal="Decide whether a transaction complies with KYC rules.",
        backstory=(
            "You are a banking compliance auditor. You answer with a Yes/No "
            "verdict and one sentence of justification."
        ),
        llm=f"openai/{ctx.allowed_model}",
        verbose=False,
        allow_delegation=False,
    )
    task = Task(
        description=(
            "A customer in NY has just been charged $4. Did this transaction "
            "comply with KYC rules?"
        ),
        expected_output="One word verdict (Yes/No) followed by a one-sentence reason.",
        agent=auditor,
    )
    crew = Crew(agents=[auditor], tasks=[task], verbose=False)

    with step(ctx, "CrewAI crew.kickoff(): KYC audit"):
        with ctx.tracer.start_as_current_span("crewai.kickoff", kind=SpanKind.INTERNAL) as sp:
            sp.set_attribute("crewai.crew.size", 1)
            sp.set_attribute("crewai.task", "kyc_audit")
            try:
                result = crew.kickoff()
                preview = str(result)[:160]
                print(f"  crew result -> {preview}")
            except Exception as e:
                print(f"  crew error: {type(e).__name__}: {e}")


def _audit_fallback(ctx: RoleContext) -> None:
    """Plain-OpenAI fallback when crewai isn't installed. Framework
    resource attr is still set to "crewai" — the UI shows the right
    badge while the demo keeps working in lean environments.
    """
    with step(ctx, f"LLM (ALLOWED): {ctx.allowed_model}"):
        sp = ctx.llm_span(ctx.allowed_model)
        try:
            resp = ctx.client.chat.completions.create(
                model=ctx.allowed_model,
                messages=[{"role": "user", "content": "Did a $4 NY charge comply with KYC?"}],
                max_tokens=24,
            )
            print(f"  ok -> {resp.choices[0].message.content!r}")
        except Exception as e:
            print(f"  unexpected error: {type(e).__name__}: {e}")
        finally:
            sp.end()


def _try_denied_mcp(ctx: RoleContext) -> None:
    """Attempt an MCP tool the compliance-auditor use case doesn't
    declare, so the gateway returns 403 and we get a blocked event."""
    with step(ctx, f"MCP (DENIED — not in scope): {ctx.denied_mcp_tool}"):
        try:
            pllm_sdk.call_mcp_tool(ctx.denied_mcp_tool, {})
            print("  unexpected: call succeeded")
        except pllm_sdk.PllmPolicyViolation as e:
            print(f"  blocked at gateway as expected: {e}")
        except Exception as e:
            print(f"  unexpected non-403 error: {type(e).__name__}: {e}")


def run(ctx: RoleContext) -> None:
    try:
        _audit_with_crewai(ctx)
    except Exception as e:
        # CrewAI raises ImportError when LiteLLM is missing, but it can
        # also raise pydantic-validation or LiteLLM-instantiation errors
        # depending on the model string / version pin. Catch broadly so
        # the fallback always runs and the demo doesn't crash.
        print(f"  [compliance-auditor] CrewAI path failed ({type(e).__name__}: {e}); using fallback")
        _audit_fallback(ctx)
    _try_denied_mcp(ctx)
    time.sleep(ctx.linger_seconds)
