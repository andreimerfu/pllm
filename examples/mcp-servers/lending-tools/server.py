"""Acme Bank lending-tools MCP server (demo).

Exposes the retail-lending toolset over streamable-HTTP so it can be
registered as a pllm MCP gateway backend (tools surface as
``lending-tools__<tool>``). Implementations are stubs returning plausible
responses — enough to exercise the registry → gateway → governance loop.
"""

import os

from mcp.server.fastmcp import FastMCP

mcp = FastMCP(
    "lending-tools",
    host="0.0.0.0",
    port=int(os.getenv("PORT", "8080")),
)


@mcp.tool()
def get_loan_products() -> str:
    """List Acme Bank loan products and indicative rates (read-only)."""
    return "Personal 9.9% APR | Auto 6.4% APR | Home Equity 7.8% APR"


@mcp.tool()
def pull_credit_report(applicant_id: str) -> str:
    """Fetch the applicant's credit report (PII + Financial)."""
    return f"credit_report(applicant={applicant_id}): score=712 utilization=28%"


@mcp.tool()
def verify_income(applicant_id: str, stated_income: float) -> str:
    """Verify stated income against payroll records (PII)."""
    return f"income_verified(applicant={applicant_id}): stated={stated_income} verified=true"


@mcp.tool()
def prequalify_applicant(applicant_id: str, amount: float) -> str:
    """Compute a pre-qualification estimate from credit + income."""
    return f"prequalified(applicant={applicant_id} amount={amount}): tier=A max=45000"


@mcp.tool()
def submit_to_underwriting(applicant_id: str, amount: float) -> str:
    """Hand the application to a human underwriter for a decision."""
    return f"submitted(applicant={applicant_id} amount={amount}): queued for underwriter"


@mcp.tool()
def approve_loan(applicant_id: str, amount: float) -> str:
    """Issue a final, binding credit decision. RESERVED FOR HUMANS."""
    return f"approved(applicant={applicant_id} amount={amount})"


@mcp.tool()
def issue_credit_decision(applicant_id: str, decision: str) -> str:
    """Record an autonomous credit decision. RESERVED FOR HUMANS."""
    return f"decision_recorded(applicant={applicant_id}): {decision}"


if __name__ == "__main__":
    mcp.run(transport="streamable-http")
