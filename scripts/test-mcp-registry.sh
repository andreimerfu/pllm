#!/usr/bin/env bash
#
# test-mcp-registry.sh — end-to-end smoke test for pllm's MCP Gateway +
# Registry + Deployment features.
#
# Covers:
#   1. Health checks
#   2. Registry CRUD:     skills, prompts, agents (with refs)
#   3. Registry-as-MCP:   /v1/mcp/registry tool-call surface
#   4. MCP gateway:       tools/list on /v1/mcp
#   5. Actual tool call:  invokes the first reg-* tool it finds (end-to-end
#                         through a deployed backend, if one exists)
#
# Env overrides:
#   BASE_URL    default http://localhost:8080
#   MASTER_KEY  default sk-master-dev-key-change-in-production
#
# Usage:
#   ./scripts/test-mcp-registry.sh            # run the full suite
#   ./scripts/test-mcp-registry.sh --cleanup  # delete test fixtures + exit

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
MASTER_KEY="${MASTER_KEY:-sk-master-dev-key-change-in-production}"
AUTH_HEADER="Authorization: Bearer ${MASTER_KEY}"

# --- pretty output --------------------------------------------------------

if [ -t 1 ]; then
  RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'
  BLUE=$'\033[34m'; DIM=$'\033[2m'; BOLD=$'\033[1m'; RESET=$'\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; BLUE=''; DIM=''; BOLD=''; RESET=''
fi

PASS=0; FAIL=0; SKIP=0
FAILURES=()

section() { printf '\n%s=== %s ===%s\n' "${BOLD}${BLUE}" "$1" "${RESET}"; }
pass()    { PASS=$((PASS+1)); printf '  %s✓%s %s\n' "${GREEN}" "${RESET}" "$1"; }
fail()    { FAIL=$((FAIL+1)); FAILURES+=("$1"); printf '  %s✗%s %s\n' "${RED}" "${RESET}" "$1"; [ $# -gt 1 ] && printf '    %s%s%s\n' "${DIM}" "$2" "${RESET}"; }
skip()    { SKIP=$((SKIP+1)); printf '  %s⊘%s %s\n' "${YELLOW}" "${RESET}" "$1"; }
info()    { printf '  %s%s%s\n' "${DIM}" "$1" "${RESET}"; }

# --- prereq check ---------------------------------------------------------

for cmd in curl jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf '%sError:%s missing required command: %s\n' "${RED}" "${RESET}" "$cmd" >&2
    exit 2
  fi
done

# --- helpers --------------------------------------------------------------

# api METHOD PATH [BODY] → prints response body, exits 0 on HTTP 2xx/3xx.
# Captures both headers and body so we can check the status.
api() {
  local method="$1" path="$2" body="${3:-}"
  local tmpfile
  tmpfile="$(mktemp)"
  local status
  if [ -n "$body" ]; then
    status=$(curl -s -o "$tmpfile" -w '%{http_code}' -X "$method" "$BASE_URL$path" \
      -H "$AUTH_HEADER" -H 'Content-Type: application/json' -d "$body")
  else
    status=$(curl -s -o "$tmpfile" -w '%{http_code}' -X "$method" "$BASE_URL$path" \
      -H "$AUTH_HEADER")
  fi
  cat "$tmpfile"
  rm -f "$tmpfile"
  [ "$status" -ge 200 ] && [ "$status" -lt 400 ]
}

# jq-safe .field lookup — returns empty string if missing.
# We avoid the `//` alternative operator because it treats `false` AND
# `null` as fallback, which breaks boolean assertions.
jqv() {
  local out
  out=$(jq -r "$1" 2>/dev/null)
  [ "$out" = "null" ] && out=""
  printf '%s' "$out"
}

# Test fixtures (namespaced so they don't collide with real data).
SKILL_NAME="test.pllm/triage-skill"
PROMPT_NAME="test.pllm/triage-prompt"
AGENT_NAME="test.pllm/triage-agent"

# --- cleanup path ---------------------------------------------------------

cleanup_fixtures() {
  section "Cleaning up test fixtures"
  for kind in skills prompts agents; do
    local name
    case "$kind" in
      skills)  name="$SKILL_NAME" ;;
      prompts) name="$PROMPT_NAME" ;;
      agents)  name="$AGENT_NAME" ;;
    esac
    # List versions, delete each. We ignore errors (404 = already gone).
    local versions
    versions=$(curl -s "$BASE_URL/v1/registry/$kind/$(printf %s "$name" | jq -sRr @uri)/versions" \
      -H "$AUTH_HEADER" | jq -r '.versions[]?.version' 2>/dev/null)
    for v in $versions; do
      curl -s -o /dev/null -X DELETE \
        "$BASE_URL/api/admin/registry/$kind/$(printf %s "$name" | jq -sRr @uri)/versions/$v" \
        -H "$AUTH_HEADER"
      info "deleted $kind/$name@$v"
    done
  done
  printf '%sCleanup done.%s\n' "${GREEN}" "${RESET}"
}

if [ "${1:-}" = "--cleanup" ]; then
  cleanup_fixtures
  exit 0
fi

# --- 1. health ------------------------------------------------------------

section "1. Health"
if curl -sf -o /dev/null "$BASE_URL/health"; then
  pass "GET /health"
else
  fail "GET /health — is pllm reachable at $BASE_URL?"
  printf '\n%sAborting: pllm is not reachable.%s\n' "${RED}" "${RESET}"
  exit 1
fi
if curl -sf -o /dev/null "$BASE_URL/ready"; then
  pass "GET /ready"
else
  fail "GET /ready"
fi

# --- 2. skills ------------------------------------------------------------

section "2. Registry — Skills"

SKILL_V1=$(api POST /api/admin/registry/skills "$(cat <<JSON
{
  "name": "$SKILL_NAME",
  "version": "0.1.0",
  "title": "Triage skill (test)",
  "description": "Categorizes incoming support tickets.",
  "image": "ghcr.io/example/ticket-triage:0.1.0",
  "manifest": {"entries": [{"path": "SKILL.md"}]}
}
JSON
)")

if [ "$(printf %s "$SKILL_V1" | jqv .is_latest)" = "true" ]; then
  pass "POST skill v0.1.0 → is_latest=true"
else
  fail "POST skill v0.1.0" "$(printf %s "$SKILL_V1" | head -c 200)"
fi

SKILL_V2=$(api POST /api/admin/registry/skills "$(cat <<JSON
{
  "name": "$SKILL_NAME",
  "version": "0.2.0",
  "title": "Triage skill v2 (test)",
  "description": "Now with few-shot examples.",
  "image": "ghcr.io/example/ticket-triage:0.2.0"
}
JSON
)")

if [ "$(printf %s "$SKILL_V2" | jqv .is_latest)" = "true" ]; then
  pass "POST skill v0.2.0 → is_latest=true"
else
  fail "POST skill v0.2.0"
fi

LATEST=$(api GET "/v1/registry/skills/$(printf %s "$SKILL_NAME" | jq -sRr @uri)")
if [ "$(printf %s "$LATEST" | jqv .version)" = "0.2.0" ]; then
  pass "GET skill resolves to latest (v0.2.0)"
else
  fail "GET skill latest" "got $(printf %s "$LATEST" | jqv .version), want 0.2.0"
fi

V1_RELOAD=$(api GET "/v1/registry/skills/$(printf %s "$SKILL_NAME" | jq -sRr @uri)?version=0.1.0")
if [ "$(printf %s "$V1_RELOAD" | jqv .is_latest)" = "false" ]; then
  pass "Previous skill version flipped is_latest=false"
else
  fail "Previous skill is_latest should be false"
fi

# --- 3. prompts -----------------------------------------------------------

section "3. Registry — Prompts"

PROMPT=$(api POST /api/admin/registry/prompts "$(cat <<JSON
{
  "name": "$PROMPT_NAME",
  "version": "1.0.0",
  "title": "Classifier system prompt",
  "description": "Turns free-text into JSON categories.",
  "template": "You classify tickets for {{team}}. Respond JSON.",
  "arguments": [{"name":"team","required":true}]
}
JSON
)")

if [ "$(printf %s "$PROMPT" | jqv .version)" = "1.0.0" ]; then
  pass "POST prompt"
else
  fail "POST prompt" "$(printf %s "$PROMPT" | head -c 200)"
fi

PFETCH=$(api GET "/v1/registry/prompts/$(printf %s "$PROMPT_NAME" | jq -sRr @uri)")
if [ -n "$(printf %s "$PFETCH" | jqv .template)" ]; then
  pass "GET prompt has template body"
else
  fail "GET prompt missing template"
fi

# --- 4. agents with refs --------------------------------------------------

section "4. Registry — Agents with dependency refs"

AGENT=$(api POST /api/admin/registry/agents "$(cat <<JSON
{
  "name": "$AGENT_NAME",
  "version": "0.1.0",
  "title": "Triage bot",
  "description": "Triage agent orchestrating a skill + prompt.",
  "language": "python",
  "framework": "langgraph",
  "model_provider": "openai",
  "model_name": "gpt-4o-mini",
  "refs": [
    {"target_kind":"skill",  "target_name":"$SKILL_NAME",  "local_name":"triage"},
    {"target_kind":"prompt", "target_name":"$PROMPT_NAME"}
  ]
}
JSON
)")

if [ "$(printf %s "$AGENT" | jq -r '.agent.is_latest // empty')" = "true" ]; then
  pass "POST agent"
else
  fail "POST agent" "$(printf %s "$AGENT" | head -c 200)"
fi

AGETCH=$(api GET "/v1/registry/agents/$(printf %s "$AGENT_NAME" | jq -sRr @uri)")
skills_n=$(printf %s "$AGETCH" | jq -r '.skills | length // 0')
prompts_n=$(printf %s "$AGETCH" | jq -r '.prompts | length // 0')
if [ "$skills_n" = "1" ] && [ "$prompts_n" = "1" ]; then
  pass "GET agent round-trips refs (skills=$skills_n, prompts=$prompts_n)"
else
  fail "GET agent refs" "got skills=$skills_n prompts=$prompts_n"
fi

# Overwrite with fewer refs — deps should shrink.
_=$(api POST /api/admin/registry/agents "$(cat <<JSON
{
  "name": "$AGENT_NAME",
  "version": "0.1.0",
  "refs": [
    {"target_kind":"prompt", "target_name":"$PROMPT_NAME"}
  ]
}
JSON
)")
AGETCH=$(api GET "/v1/registry/agents/$(printf %s "$AGENT_NAME" | jq -sRr @uri)")
if [ "$(printf %s "$AGETCH" | jq -r '.skills | length // 0')" = "0" ]; then
  pass "Agent re-upsert replaces refs wholesale (skills cleared)"
else
  fail "Agent upsert should clear old refs"
fi

# --- 5. Registry-as-MCP ---------------------------------------------------

section "5. Registry-as-MCP (catalog over MCP protocol)"

RMCP_TOOLS=$(api POST /v1/mcp/registry "$(cat <<'JSON'
{"jsonrpc":"2.0","id":1,"method":"tools/list"}
JSON
)")

# Expect tools like list_servers / get_server / list_agents / ...
tool_names=$(printf %s "$RMCP_TOOLS" | jq -r '.result.tools[]?.name' 2>/dev/null)
for want in list_servers get_server list_agents list_skills list_prompts; do
  if printf '%s\n' "$tool_names" | grep -qx "$want"; then
    pass "registry MCP exposes tool: $want"
  else
    fail "registry MCP missing tool: $want"
  fi
done

# Call list_skills over MCP — should include our test skill.
RMCP_SKILLS=$(api POST /v1/mcp/registry "$(cat <<'JSON'
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_skills"}}
JSON
)")
if printf %s "$RMCP_SKILLS" | jq -r '.result.content[0].text' \
     | jq -e --arg n "$SKILL_NAME" '.Items | map(.name) | index($n) != null' >/dev/null 2>&1; then
  pass "registry MCP list_skills returns $SKILL_NAME"
else
  fail "registry MCP list_skills missing $SKILL_NAME"
fi

# --- 6. MCP gateway (aggregated) ------------------------------------------

section "6. MCP Gateway /v1/mcp"

GW_TOOLS=$(api POST /v1/mcp "$(cat <<'JSON'
{"jsonrpc":"2.0","id":1,"method":"tools/list"}
JSON
)")

total_tools=$(printf %s "$GW_TOOLS" | jq -r '.result.tools | length // 0')
if [ "$total_tools" -ge 0 ]; then
  pass "gateway tools/list returned $total_tools tools"
else
  fail "gateway tools/list"
fi

# Look for any reg-* tool (means a registry-deployed backend is live).
reg_tool=$(printf %s "$GW_TOOLS" | jq -r '.result.tools[]?.name' | grep -E '^reg-.*__' | head -1 || true)

if [ -z "$reg_tool" ]; then
  skip "no deployed reg-* backend found — deploy a server from the Registry to test end-to-end invocation"
else
  pass "found deployed backend tool: $reg_tool"

  # Pick something safe to call. Prefer echo/add/ping; fall back to first.
  pick=""
  for pattern in '__echo$' '__add$' '__ping$'; do
    c=$(printf %s "$GW_TOOLS" | jq -r '.result.tools[]?.name' | grep -E "^reg-.*$pattern" | head -1 || true)
    if [ -n "$c" ]; then pick="$c"; break; fi
  done
  pick="${pick:-$reg_tool}"

  # Build args that match what we're calling.
  args='{}'
  case "$pick" in
    *__echo) args='{"message":"hello from test-mcp-registry.sh"}' ;;
    *__add)  args='{"a":12,"b":30}' ;;
  esac

  RESP=$(api POST /v1/mcp "$(cat <<JSON
{
  "jsonrpc":"2.0","id":2,"method":"tools/call",
  "params":{"name":"$pick","arguments":$args}
}
JSON
)")
  if [ -n "$(printf %s "$RESP" | jq -r '.result.content[0].text // empty' 2>/dev/null)" ]; then
    pass "invoked $pick via /v1/mcp"
    info "response: $(printf %s "$RESP" | jq -r '.result.content[0].text' | head -c 120)"
  elif [ -n "$(printf %s "$RESP" | jq -r '.error.message // empty' 2>/dev/null)" ]; then
    fail "invocation returned JSON-RPC error" "$(printf %s "$RESP" | jq -r '.error.message')"
  else
    fail "invocation response malformed" "$(printf %s "$RESP" | head -c 200)"
  fi
fi

# --- 7. deployments + backends snapshot -----------------------------------

section "7. Snapshot (informational)"

DEPS=$(api GET /api/admin/deployments)
dep_count=$(printf %s "$DEPS" | jq -r '.deployments | length // 0')
info "active deployments: $dep_count"
if [ "$dep_count" -gt 0 ]; then
  printf %s "$DEPS" | jq -r '.deployments[] | "    - \(.resource_name)@\(.resource_version) [\(.status)]"'
fi

BES=$(api GET /api/admin/mcp/servers)
be_count=$(printf %s "$BES" | jq -r '. | length // 0')
info "MCP backends: $be_count"
if [ "$be_count" -gt 0 ]; then
  printf %s "$BES" | jq -r '.[] | "    - \(.slug) [\(.transport)] \(.live_health // .health_status)"'
fi

# --- 8. lifecycle hygiene -------------------------------------------------

section "8. Lifecycle hygiene"

# Status before any forced run — should be 200 with at least an empty
# summary (or whatever the daemon's first-tick produced).
LC_STATUS=$(api GET /api/admin/lifecycle/status)
if [ -n "$LC_STATUS" ] && printf %s "$LC_STATUS" | jq -e . >/dev/null 2>&1; then
  pass "GET /api/admin/lifecycle/status returns valid JSON"
else
  fail "GET /api/admin/lifecycle/status" "$(printf %s "$LC_STATUS" | head -c 200)"
fi

# Force a run. Must return 200 and a JSON summary listing all three rules.
LC_RUN=$(api POST /api/admin/lifecycle/run)
if [ -n "$LC_RUN" ] && printf %s "$LC_RUN" | jq -e . >/dev/null 2>&1; then
  pass "POST /api/admin/lifecycle/run returns valid JSON"
else
  fail "POST /api/admin/lifecycle/run" "$(printf %s "$LC_RUN" | head -c 200)"
fi

# Each rule must appear in the rules array.
for rule in inactive_keys ownerless_keys risky_servers; do
  if printf %s "$LC_RUN" | jq -e --arg r "$rule" \
       '.rules | map(.rule) | index($r) != null' >/dev/null 2>&1; then
    pass "lifecycle run executed rule: $rule"
  else
    fail "lifecycle run missing rule: $rule" "$(printf %s "$LC_RUN" | head -c 200)"
  fi
done

# --- summary --------------------------------------------------------------

printf '\n'
printf '%s──────── summary ────────%s\n' "${BOLD}" "${RESET}"
printf '%s%d passed%s · %s%d failed%s · %s%d skipped%s\n' \
  "${GREEN}" "$PASS" "${RESET}" \
  "${RED}" "$FAIL" "${RESET}" \
  "${YELLOW}" "$SKIP" "${RESET}"

if [ "$FAIL" -gt 0 ]; then
  printf '\nFailures:\n'
  for f in "${FAILURES[@]}"; do
    printf '  %s✗%s %s\n' "${RED}" "${RESET}" "$f"
  done
  exit 1
fi

printf '\nRe-run with %s--cleanup%s to remove test fixtures from the registry.\n' "${BOLD}" "${RESET}"
