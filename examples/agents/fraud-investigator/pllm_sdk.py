"""pllm SDK — client-side policy enforcement for Python agents.

Import + ``init()`` early. The SDK fetches the scope of the calling
key from ``GET /v1/sdk/policy`` and patches ``httpx.Client.send`` /
``httpx.AsyncClient.send`` so any HTTP call to a target outside the
scope raises ``PllmPolicyViolation`` *before* the network request is
made. Calls to the pllm gateway base URL are always allowed — the
gateway enforces its own rules.

Failure mode: **fail-closed**. If the policy fetch fails on init, all
non-pllm-gateway calls are blocked until a successful refresh. This
mirrors the regulated-control-plane pitch: if you can't reach the
control plane, you don't get to make calls.

Usage::

    import pllm_sdk

    pllm_sdk.init(
        base_url="http://localhost:8080",
        api_key="sk-api-...",
        # service_name reads OTEL_SERVICE_NAME if not provided
    )

    # any subsequent httpx call is checked
    from openai import OpenAI
    client = OpenAI(api_key="sk-api-...", base_url="http://localhost:8080/v1")
    client.chat.completions.create(...)  # ok if model in scope, else 403 from gateway

    from qdrant_client import QdrantClient
    QdrantClient(url="http://localhost:6333").get_collections()
    # raises PllmPolicyViolation locally — qdrant isn't in scope and isn't
    # the pllm gateway

The SDK also emits an OTel span on each blocked call (``pllm.policy.blocked=true``)
so the violation appears in pllm's compliance audit trail even though the
network call never happened.
"""
from __future__ import annotations

import os
import threading
import time
import urllib.parse
from typing import Any, Iterable, Optional, Set

try:
    import httpx
except ImportError as e:  # pragma: no cover
    raise ImportError(
        "pllm_sdk requires httpx — install it with `pip install httpx`."
    ) from e


__all__ = [
    "init",
    "PllmPolicyViolation",
    "current_policy",
    "get_prompt",
    "render_prompt",
    "call_mcp_tool",
]


class PllmPolicyViolation(Exception):
    """Raised when the SDK blocks a call because the target isn't in scope."""

    def __init__(self, target: str, reason: str):
        super().__init__(f"pllm policy: {reason} (target: {target})")
        self.target = target
        self.reason = reason


# --- module state ----------------------------------------------------------

_lock = threading.RLock()
_state: dict = {
    "initialized": False,
    "base_url": None,           # str
    "api_key": None,            # str
    "gateway_host": None,       # str  e.g. "localhost:8080"
    "policy": None,             # dict from /v1/sdk/policy, None until first success
    "policy_loaded_at": 0.0,    # epoch seconds
    "fail_closed": True,
    "extra_allowed_hosts": set(),
    "stop": threading.Event(),
    "patched": False,
    "_orig_send": None,
    "_orig_async_send": None,
    "tracer": None,             # opentelemetry tracer if available
}


def current_policy() -> Optional[dict]:
    """Returns the most recent policy snapshot, or None if not yet fetched."""
    with _lock:
        return None if _state["policy"] is None else dict(_state["policy"])


# --- policy fetching -------------------------------------------------------

def _fetch_policy() -> Optional[dict]:
    base = _state["base_url"]
    key = _state["api_key"]
    if not base or not key:
        return None
    try:
        # Use a vanilla unpatched httpx Client to avoid recursion.
        with httpx.Client(timeout=5.0) as c:
            resp = c.send(
                c.build_request(
                    "GET",
                    f"{base.rstrip('/')}/v1/sdk/policy",
                    headers={"Authorization": f"Bearer {key}"},
                )
            )
            if resp.status_code == 401:
                _emit_log(
                    "FATAL: api key rejected by pllm (HTTP 401). "
                    "The key was likely auto-revoked after flagged-call threshold "
                    "or has been deleted. Re-run scripts/setup-multiagent-demo.sh "
                    "and re-source the generated .env."
                )
                return None
            if resp.status_code != 200:
                body = resp.text[:200] if hasattr(resp, "text") else ""
                _emit_log(f"policy fetch returned {resp.status_code}: {body}")
                return None
            return resp.json()
    except Exception as e:  # pragma: no cover
        _emit_log(f"policy fetch failed: {type(e).__name__}: {e}")
        return None


def _refresher() -> None:
    while not _state["stop"].is_set():
        # Use TTL from the last policy or default to 30s.
        ttl = 30
        if _state["policy"]:
            ttl = max(5, int(_state["policy"].get("ttl_seconds", 30)))
        if _state["stop"].wait(ttl):
            return
        new_policy = _fetch_policy()
        if new_policy:
            with _lock:
                _state["policy"] = new_policy
                _state["policy_loaded_at"] = time.time()


# --- decision --------------------------------------------------------------

def _host_port(url: str) -> str:
    """Returns 'host:port' (port omitted only when scheme-default)."""
    p = urllib.parse.urlparse(url)
    host = p.hostname or ""
    if p.port is not None:
        return f"{host}:{p.port}"
    if p.scheme == "https":
        return f"{host}:443"
    return f"{host}:80"


def _is_pllm_gateway(url: str) -> bool:
    return _host_port(url) == _state["gateway_host"]


def _allowed_external(host_port: str) -> bool:
    if host_port in _state["extra_allowed_hosts"]:
        return True
    pol = _state["policy"]
    if not pol:
        return False
    for h in pol.get("allowed_external", []) or []:
        if host_port == h or host_port.endswith(h):
            return True
    return False


def _decide(method: str, url: str) -> tuple[bool, str]:
    """Returns (allowed, reason)."""
    if not _state["initialized"]:
        return True, "sdk not initialized"
    if _is_pllm_gateway(url):
        return True, "pllm gateway (gateway enforces)"
    pol = _state["policy"]
    if pol is None:
        if _state["fail_closed"]:
            return False, "policy not loaded yet (fail-closed)"
        return True, "policy not loaded (fail-open)"
    if _allowed_external(_host_port(url)):
        return True, "host on allowed_external list"
    return False, f"{_host_port(url)} not in allowed_external list"


# --- OTel emission ---------------------------------------------------------

def _emit_blocked_span(method: str, url: str, reason: str) -> None:
    tracer = _state.get("tracer")
    if tracer is None:
        return
    try:
        from opentelemetry.trace import SpanKind  # local import
        with tracer.start_as_current_span(f"pllm.policy.blocked.{method}", kind=SpanKind.CLIENT) as sp:
            sp.set_attribute("http.method", method)
            sp.set_attribute("http.url", url)
            sp.set_attribute("pllm.policy.blocked", True)
            sp.set_attribute("pllm.policy.reason", reason)
    except Exception:
        pass


def _emit_log(msg: str) -> None:
    print(f"[pllm_sdk] {msg}", flush=True)


# --- httpx patches ---------------------------------------------------------

def _patched_send(self, request: "httpx.Request", *args, **kwargs):
    url = str(request.url)
    method = request.method.upper()
    allowed, reason = _decide(method, url)
    if not allowed:
        _emit_blocked_span(method, url, reason)
        _emit_log(f"BLOCKED {method} {url} — {reason}")
        raise PllmPolicyViolation(url, reason)
    return _state["_orig_send"](self, request, *args, **kwargs)


async def _patched_async_send(self, request: "httpx.Request", *args, **kwargs):
    url = str(request.url)
    method = request.method.upper()
    allowed, reason = _decide(method, url)
    if not allowed:
        _emit_blocked_span(method, url, reason)
        _emit_log(f"BLOCKED async {method} {url} — {reason}")
        raise PllmPolicyViolation(url, reason)
    return await _state["_orig_async_send"](self, request, *args, **kwargs)


def _install_patches() -> None:
    if _state["patched"]:
        return
    _state["_orig_send"] = httpx.Client.send
    _state["_orig_async_send"] = httpx.AsyncClient.send
    httpx.Client.send = _patched_send  # type: ignore[assignment]
    httpx.AsyncClient.send = _patched_async_send  # type: ignore[assignment]
    _state["patched"] = True


# --- public API ------------------------------------------------------------

def init(
    base_url: str,
    api_key: str,
    *,
    fail_closed: bool = True,
    extra_allowed_hosts: Optional[Iterable[str]] = None,
    refresh: bool = True,
) -> None:
    """Initialize the SDK and patch httpx for policy enforcement.

    :param base_url: pllm gateway URL, e.g. ``http://localhost:8080``.
        Calls to this host are always allowed (gateway enforces).
    :param api_key: bearer key the SDK uses to fetch the policy.
    :param fail_closed: when True (default), block all non-gateway calls
        if the policy can't be fetched. When False, allow them through.
    :param extra_allowed_hosts: optional ``host:port`` strings to allow
        unconditionally — e.g. an OTel collector that lives outside the
        pllm gateway. The OTel exporter usually points at the pllm
        gateway already, so this is rarely needed.
    :param refresh: spawn a background thread to refresh the policy
        every ``ttl_seconds``. Disable for short-lived scripts.
    """
    with _lock:
        _state["base_url"] = base_url
        _state["api_key"] = api_key
        _state["gateway_host"] = _host_port(base_url)
        _state["fail_closed"] = fail_closed
        if extra_allowed_hosts:
            _state["extra_allowed_hosts"] = set(extra_allowed_hosts)
        # Try to obtain an OTel tracer for blocked-call audit spans.
        try:
            from opentelemetry import trace
            _state["tracer"] = trace.get_tracer("pllm_sdk")
        except Exception:
            _state["tracer"] = None

        # Initial fetch — best-effort. Leaves _state["policy"] = None on
        # failure, which (with fail_closed=True) blocks all non-gateway
        # traffic until the refresher succeeds.
        pol = _fetch_policy()
        if pol:
            _state["policy"] = pol
            _state["policy_loaded_at"] = time.time()
            _emit_log(
                f"policy loaded: enforce={pol.get('enforce')} "
                f"llms={len(pol.get('allowed_llms', []))} "
                f"agents={len(pol.get('allowed_agents', []))} "
                f"mcp={len(pol.get('allowed_mcp_servers', []))}"
            )
        else:
            _emit_log("WARNING: policy not loaded — fail-closed mode active" if fail_closed else "policy not loaded — fail-open")

        _install_patches()
        _state["initialized"] = True

    if refresh:
        t = threading.Thread(target=_refresher, name="pllm_sdk_refresher", daemon=True)
        t.start()


# --- gateway helpers (prompts + MCP) --------------------------------------

def _gateway_request(method: str, path: str, **kwargs) -> "httpx.Response":
    """Issue a bearer-authed request to the pllm gateway via the
    *patched* httpx client — this means the SDK's own policy applies to
    its own helpers (a misconfigured agent that tries to grab a prompt
    or call a tool not in scope sees the same `PllmPolicyViolation`)."""
    base = _state["base_url"]
    key = _state["api_key"]
    if not base or not key:
        raise RuntimeError("pllm_sdk.init() must be called before gateway helpers")
    headers = kwargs.pop("headers", {}) or {}
    headers["Authorization"] = f"Bearer {key}"
    with httpx.Client(timeout=15.0) as c:
        return c.request(method, f"{base.rstrip('/')}{path}", headers=headers, **kwargs)


def get_prompt(name: str, version: str = "latest") -> dict:
    """Fetch a registered prompt template from the pllm gateway.

    Returns ``{"name", "version", "title", "template", "arguments"}``.
    Raises:
      - PllmPolicyViolation: if the bound use case doesn't declare the prompt.
      - httpx.HTTPStatusError: for 4xx/5xx other than 403.
    """
    resp = _gateway_request("GET", f"/v1/prompts/{name}/versions/{version}")
    if resp.status_code == 403:
        raise PllmPolicyViolation(
            f"prompt:{name}@{version}",
            f"prompt fetch denied by gateway (403): {resp.text[:200]}",
        )
    resp.raise_for_status()
    return resp.json()


def render_prompt(template: str, **vars) -> str:
    """Format a fetched prompt template with the provided keyword args.

    Uses Python ``str.format_map`` so missing variables surface clearly.
    """
    return template.format_map(_SafeMap(vars))


class _SafeMap(dict):
    def __missing__(self, key: str) -> str:
        return "{" + key + "}"


def call_mcp_tool(tool: str, arguments: Optional[dict] = None) -> dict:
    """Invoke an MCP tool through the pllm gateway via JSON-RPC.

    ``tool`` is the gateway-aggregated name like
    ``server-everything__echo`` (slug + ``__`` + tool name). Returns
    the JSON-RPC ``result`` payload. Raises ``PllmPolicyViolation`` on
    403 (gateway-side scope deny).
    """
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {"name": tool, "arguments": arguments or {}},
    }
    resp = _gateway_request("POST", "/v1/mcp", json=payload)
    if resp.status_code == 403:
        raise PllmPolicyViolation(
            f"mcp:{tool}",
            f"mcp call denied by gateway (403): {resp.text[:200]}",
        )
    resp.raise_for_status()
    body = resp.json()
    if isinstance(body, dict) and "error" in body:
        # JSON-RPC error envelope (gateway returned 200 but with error)
        err = body["error"]
        msg = err.get("message", "mcp error") if isinstance(err, dict) else str(err)
        raise RuntimeError(f"mcp tool '{tool}' returned error: {msg}")
    return body.get("result", body) if isinstance(body, dict) else body


def shutdown() -> None:
    """Stop the refresh thread and undo httpx patches. Mostly for tests."""
    _state["stop"].set()
    if _state["patched"] and _state["_orig_send"]:
        httpx.Client.send = _state["_orig_send"]  # type: ignore[assignment]
        httpx.AsyncClient.send = _state["_orig_async_send"]  # type: ignore[assignment]
        _state["patched"] = False
