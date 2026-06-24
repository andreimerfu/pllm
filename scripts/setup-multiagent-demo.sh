#!/usr/bin/env bash
#
# setup-multiagent-demo.sh — bootstrap the 3-agent governance demo.
#
# Creates:
#   1. orchestrator-uc        UseCase: LLM(Z.ai GLM) + agent(risk-scorer)
#   2. risk-scorer-uc         UseCase: LLM(Z.ai GLM)
#   3. compliance-auditor-uc  UseCase: LLM(Z.ai GLM)
# Plus 3 API keys, one bound to each UseCase.
#
# Output: examples/agents/fraud-investigator/.env containing the env vars
# that docker-compose.yml expects. Source it before `docker compose up`.
#
# Idempotent: safe to re-run. If a use case with the same name already
# exists, the script reuses it. Keys are always recreated (the plaintext
# isn't retrievable after creation).
#
# Env: BASE_URL (default http://localhost:8080)
#      MASTER_KEY (default sk-master-dev-key-change-in-production)
#      ALLOWED_MODEL (default 'Z.ai: GLM 4.5 Air (free)')
#      DENIED_MODEL  (default 'Qwen: Qwen3.6 Plus (free)')

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
MASTER_KEY="${MASTER_KEY:-sk-master-dev-key-change-in-production}"
ALLOWED_MODEL="${ALLOWED_MODEL:-Z.ai: GLM 4.5 Air (free)}"
DENIED_MODEL="${DENIED_MODEL:-Qwen: Qwen3.6 Plus (free)}"
ALLOWED_MCP_SERVER="${ALLOWED_MCP_SERVER:-reg-server-everything-1-0-0}"
PROMPT_NAME="${PROMPT_NAME:-demo/loan-risk-eval}"
PROMPT_VERSION="${PROMPT_VERSION:-1.0.0}"

H_AUTH="Authorization: Bearer ${MASTER_KEY}"
H_JSON="Content-Type: application/json"

if [ -t 1 ]; then
  GREEN=$'\033[32m'; RED=$'\033[31m'; BOLD=$'\033[1m'; RESET=$'\033[0m'
else GREEN=''; RED=''; BOLD=''; RESET=''; fi

ok()   { printf "%s ✓ %s%s\n" "$GREEN" "$1" "$RESET"; }
fail() { printf "%s ✗ %s%s\n" "$RED" "$1" "$RESET"; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq is required"

# --- Health check ----------------------------------------------------------
curl -s "${BASE_URL}/health" >/dev/null 2>&1 || fail "pllm not reachable at ${BASE_URL}"
ok "pllm reachable"

# Resolve-or-create a UseCase by name and ALWAYS overwrite its refs to
# match `payload`. Echoes the ID. The full-overwrite behavior is what
# we want here — the demo's source-of-truth is this script, not what
# happened to be persisted from a previous run.
ensure_use_case() {
  local name="$1"
  local payload="$2"
  local existing
  existing=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/use-cases" |
    jq -r --arg n "$name" '.use_cases[]? | select(.name==$n) | .id' | head -1)
  if [ -n "$existing" ] && [ "$existing" != "null" ]; then
    # Overwrite refs / scope by PUTting the full payload on the existing ID.
    curl -s -o /dev/null -X PUT -H "$H_AUTH" -H "$H_JSON" -d "$payload" \
      "${BASE_URL}/api/admin/use-cases/${existing}"
    echo "$existing"
    return
  fi
  curl -s -X POST -H "$H_AUTH" -H "$H_JSON" -d "$payload" \
    "${BASE_URL}/api/admin/use-cases" |
    jq -r '.use_case.id // .id // empty'
}

# --- 1. Prompt registry entry --------------------------------------------
PROMPT_PAYLOAD=$(jq -n \
  --arg name "$PROMPT_NAME" \
  --arg version "$PROMPT_VERSION" \
  '{
    name: $name,
    version: $version,
    title: "Loan Risk Evaluation",
    description: "Centralized template for loan-risk scoring. Bound use cases get this; others get 403.",
    template: "Evaluate this transaction for fraud risk.\nCustomer: {customer}\nAmount: {amount}\nReply with one of: low | medium | high.",
    arguments: [
      { name: "customer", description: "Customer identifier", required: true },
      { name: "amount", description: "Transaction amount + currency", required: true }
    ]
  }')
curl -s -o /dev/null -X POST -H "$H_AUTH" -H "$H_JSON" \
  -d "$PROMPT_PAYLOAD" "${BASE_URL}/api/admin/registry/prompts" || true
ok "prompt registered: ${PROMPT_NAME}@${PROMPT_VERSION}"

# --- 2. UseCases -----------------------------------------------------------
ORCH_PAYLOAD=$(jq -n \
  --arg llm "$ALLOWED_MODEL" \
  --arg peer "risk-scorer" \
  --arg mcp "$ALLOWED_MCP_SERVER" \
  '{
    name: "otel-multi-orchestrator",
    description: "Multi-agent demo: orchestrator allowed to call risk-scorer + one MCP server; LLM scope = single allowed model",
    enforce_scope: true,
    refs: [
      { target_kind: "llm",    target_name: $llm },
      { target_kind: "agent",  target_name: $peer },
      { target_kind: "server", target_name: $mcp }
    ]
  }')
ORCH_UC_ID=$(ensure_use_case "otel-multi-orchestrator" "$ORCH_PAYLOAD")
[ -n "$ORCH_UC_ID" ] || fail "create orchestrator UC"
ok "orchestrator UC: ${ORCH_UC_ID}"

RISK_PAYLOAD=$(jq -n \
  --arg llm "$ALLOWED_MODEL" \
  --arg pname "$PROMPT_NAME" \
  '{
    name: "otel-multi-risk-scorer",
    description: "Multi-agent demo: risk-scorer LLM + registered prompt template only",
    enforce_scope: true,
    refs: [
      { target_kind: "llm",    target_name: $llm },
      { target_kind: "prompt", target_name: $pname }
    ]
  }')
RISK_UC_ID=$(ensure_use_case "otel-multi-risk-scorer" "$RISK_PAYLOAD")
[ -n "$RISK_UC_ID" ] || fail "create risk-scorer UC"
ok "risk-scorer UC: ${RISK_UC_ID}"

COMP_PAYLOAD=$(jq -n --arg llm "$ALLOWED_MODEL" '{
  name: "otel-multi-compliance-auditor",
  description: "Multi-agent demo: compliance-auditor LLM scope only",
  enforce_scope: true,
  refs: [
    { target_kind: "llm", target_name: $llm }
  ]
}')
COMP_UC_ID=$(ensure_use_case "otel-multi-compliance-auditor" "$COMP_PAYLOAD")
[ -n "$COMP_UC_ID" ] || fail "create compliance-auditor UC"
ok "compliance-auditor UC: ${COMP_UC_ID}"

# --- 2. Keys ---------------------------------------------------------------
# Key handling: the discovery auto-revoker fires during a normal demo
# run and revokes the orchestrator + compliance-auditor keys (sets
# is_active=false AND revoked_at=now). To make this script idempotent
# across docker-compose iterations, we DELETE any existing keys with the
# demo names and mint fresh ones. PUT/{id} alone wouldn't be enough —
# the admin handler historically didn't clear revoked_at, so reactivated
# keys still failed Key.CanUse(). Delete + create avoids that quirk and
# guarantees a clean slate on every run.
mint_key() {
  local name="$1"
  local uc="$2"

  # Delete any pre-existing keys with this exact name. The list endpoint
  # is master-key-protected; we filter client-side via jq.
  local stale_ids
  stale_ids=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/keys?limit=200" |
    jq -r --arg n "$name" '.keys[]? | select(.name==$n) | .id')
  if [ -n "$stale_ids" ]; then
    while IFS= read -r kid; do
      [ -z "$kid" ] && continue
      curl -s -o /dev/null -X DELETE -H "$H_AUTH" \
        "${BASE_URL}/api/admin/keys/${kid}"
    done <<<"$stale_ids"
  fi

  local resp
  resp=$(curl -s -X POST -H "$H_AUTH" -H "$H_JSON" \
    -d "$(jq -n --arg n "$name" --arg uc "$uc" '{name:$n,key_type:"api",use_case_id:$uc}')" \
    "${BASE_URL}/api/admin/keys")
  local id key
  id=$(echo "$resp" | jq -r '.id // empty')
  key=$(echo "$resp" | jq -r '.plaintext_key // .key // empty')
  if [ -z "$id" ] || [ -z "$key" ]; then
    echo "$resp" >&2
    fail "create key $name"
  fi
  echo "${id}|${key}"
}

ORCH_PAIR=$(mint_key "otel-multi-orchestrator-key" "$ORCH_UC_ID")
RISK_PAIR=$(mint_key "otel-multi-risk-scorer-key" "$RISK_UC_ID")
COMP_PAIR=$(mint_key "otel-multi-compliance-auditor-key" "$COMP_UC_ID")

ORCH_KEY_ID="${ORCH_PAIR%%|*}"; ORCH_API_KEY="${ORCH_PAIR##*|}"
RISK_KEY_ID="${RISK_PAIR%%|*}"; RISK_API_KEY="${RISK_PAIR##*|}"
COMP_KEY_ID="${COMP_PAIR%%|*}"; COMP_API_KEY="${COMP_PAIR##*|}"

ok "orchestrator key  ${ORCH_KEY_ID}"
ok "risk-scorer key   ${RISK_KEY_ID}"
ok "compliance key    ${COMP_KEY_ID}"

# Probe each new key against /v1/sdk/policy. The SDK fails-closed when
# this endpoint returns non-200, which surfaces in the agent log as
# "policy not loaded — fail-closed mode active" and 401s on every
# downstream call. Catch the bad key here instead of debugging in the
# container output.
verify_key() {
  local label="$1"
  local key="$2"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${key}" "${BASE_URL}/v1/sdk/policy")
  if [ "$code" != "200" ]; then
    fail "key ${label} rejected by /v1/sdk/policy (HTTP ${code}). Stop containers, then re-run."
  fi
}
verify_key "orchestrator"       "$ORCH_API_KEY"
verify_key "risk-scorer"        "$RISK_API_KEY"
verify_key "compliance-auditor" "$COMP_API_KEY"
ok "all 3 keys validated against /v1/sdk/policy"

# --- 2b. Reset existing DiscoveredAgent rows so flagged_count starts ---
# at 0 and the revoker's per-agent latch is cleared. Without this, the
# orchestrator gets auto-revoked seconds into the next run because
# flagged_count from previous demo iterations persists.
reset_discovered() {
  local svc="$1"
  local id
  id=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/discovery/agents" |
    jq -r --arg s "$svc" '.items[]? | select(.service_name==$s) | .id' | head -1)
  if [ -n "$id" ] && [ "$id" != "null" ]; then
    curl -s -o /dev/null -X POST -H "$H_AUTH" \
      "${BASE_URL}/api/admin/discovery/agents/${id}/reset"
    ok "reset discovered agent: ${svc}"
  fi
}
reset_discovered "orchestrator"
reset_discovered "risk-scorer"
reset_discovered "compliance-auditor"

# --- 3. Write .env ---------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/../examples/agents/fraud-investigator/.env"

cat > "$ENV_FILE" <<EOF
# Auto-generated by setup-multiagent-demo.sh on $(date -u +%Y-%m-%dT%H:%M:%SZ)
# Source this before \`docker compose up\`:
#   set -a && source .env && set +a && docker compose up --build

PLLM_BASE_URL=${BASE_URL}
ALLOWED_MODEL=${ALLOWED_MODEL}
DENIED_MODEL=${DENIED_MODEL}
ALLOWED_PEER=risk-scorer
DENIED_PEER=compliance-auditor

# orchestrator
ORCH_API_KEY=${ORCH_API_KEY}
ORCH_KEY_ID=${ORCH_KEY_ID}

# risk-scorer
RISK_API_KEY=${RISK_API_KEY}
RISK_KEY_ID=${RISK_KEY_ID}

# compliance-auditor
COMP_API_KEY=${COMP_API_KEY}
COMP_KEY_ID=${COMP_KEY_ID}
EOF
ok "wrote env file: ${ENV_FILE}"

cat <<EOF

${BOLD}Next${RESET}

  cd examples/agents/fraud-investigator
  docker compose down       # clear any prior container's cached env
  set -a && source .env && set +a
  docker compose up --build

After the run:
  - /compliance → 3 discovered agents (orchestrator, risk-scorer, compliance-auditor)
  - orchestrator drawer shows: 1 LLM allowed, 1 vector_db flagged, 1 agent (risk-scorer) allowed,
    1 agent (compliance-auditor) flagged
  - Promote the orchestrator → /registry → fraud-investigator → Dependencies
    shows the full graph
EOF
