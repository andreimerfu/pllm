"""Multi-agent governance demo dispatcher.

Three roles run via AGENT_ROLE env var:

  orchestrator       — Custom Python orchestrator. Calls LLM, Qdrant,
                       MCP tools (allowed + denied), and emits A2A spans
                       to risk-scorer (allowed) and compliance-auditor
                       (flagged).
  risk-scorer        — LangGraph StateGraph. Fetches a registered
                       prompt from pllm, renders it, calls the LLM.
  compliance-auditor — CrewAI Crew. Single agent + task running through
                       pllm's LLM gateway, plus a denied MCP attempt.

Each role module sets `pllm.agent.framework` on its OTel resource so
pllm's UI can render the framework's vendor logo.
"""

from __future__ import annotations

import os
import sys
import time

# The SDK monkey-patches httpx — import it before anything that builds
# an httpx Client (openai, qdrant, langchain-openai, litellm).
import pllm_sdk  # noqa: F401  (intentional early import)

from roles import common, compliance_auditor, orchestrator, risk_scorer


ROLES = {
    "orchestrator":       (orchestrator.run, orchestrator.FRAMEWORK),
    "risk-scorer":        (risk_scorer.run, risk_scorer.FRAMEWORK),
    "compliance-auditor": (compliance_auditor.run, compliance_auditor.FRAMEWORK),
}


def main() -> int:
    role = os.environ.get("AGENT_ROLE", "orchestrator")
    entry = ROLES.get(role)
    if entry is None:
        print(
            f"unknown AGENT_ROLE={role!r}; expected one of {list(ROLES)}",
            file=sys.stderr,
        )
        return 2

    run_fn, framework = entry
    ctx = common.build_context(role=role, framework=framework)
    run_fn(ctx)

    time.sleep(2)
    print(f"\n[{ctx.service_name}] done.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
