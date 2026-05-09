#!/usr/bin/env bash
#
# test-otel-discovery.sh — end-to-end smoke for OTel-driven agent discovery.
#
#  1. Health
#  2. Create UseCase that allows openai/gpt-4o only
#  3. Create a key bound to that UseCase
#  4. POST a synthetic OTLP batch (LLM gpt-4o + Qdrant search) at /v1/traces
#  5. Verify a DiscoveredAgent appears via /api/admin/discovery/agents
#  6. Verify the Qdrant call is flagged
#  7. Promote the agent
#  8. POST 2 more OTLP batches with Qdrant -> 3 flags total -> key revoked
#
# Env: BASE_URL (default http://localhost:8080)
#      MASTER_KEY (default sk-master-dev-key-change-in-production)
#
# Usage:
#   ./scripts/test-otel-discovery.sh

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
MASTER_KEY="${MASTER_KEY:-sk-master-dev-key-change-in-production}"
H_AUTH="Authorization: Bearer ${MASTER_KEY}"
H_JSON="Content-Type: application/json"

if [ -t 1 ]; then
  GREEN=$'\033[32m'; RED=$'\033[31m'; YELLOW=$'\033[33m'; BOLD=$'\033[1m'; RESET=$'\033[0m'
else
  GREEN=''; RED=''; YELLOW=''; BOLD=''; RESET=''
fi

PASS=0; FAIL=0
FAILURES=()

ok()   { PASS=$((PASS+1)); printf "%s ✓ %s%s\n" "$GREEN" "$1" "$RESET"; }
fail() { FAIL=$((FAIL+1)); FAILURES+=("$1"); printf "%s ✗ %s%s\n" "$RED" "$1" "$RESET"; }
note() { printf "%s· %s%s\n" "$YELLOW" "$1" "$RESET"; }

# ---- prereq ---------------------------------------------------------------
command -v jq >/dev/null 2>&1 || { echo "jq is required"; exit 1; }

# ---- 1. Health ------------------------------------------------------------
if curl -s "${BASE_URL}/health" | grep -q '"status"'; then
  ok "health"
else
  fail "health (is pllm running at ${BASE_URL}?)"
  exit 1
fi

# ---- 2. Create UseCase ---------------------------------------------------
UC_NAME="otel-demo-$(date +%s)"
UC_PAYLOAD=$(cat <<JSON
{
  "name": "${UC_NAME}",
  "description": "auto-created by test-otel-discovery.sh",
  "enforce_scope": true,
  "refs": [
    {"target_kind":"llm","target_name":"openai/gpt-4o"}
  ]
}
JSON
)
UC_RESP=$(curl -s -X POST -H "$H_AUTH" -H "$H_JSON" -d "$UC_PAYLOAD" "${BASE_URL}/api/admin/use-cases")
UC_ID=$(echo "$UC_RESP" | jq -r '.use_case.id // .id // empty')
if [ -n "$UC_ID" ]; then
  ok "use case created (${UC_ID})"
else
  fail "use case create — response: $UC_RESP"
  exit 1
fi

# ---- 3. Create key bound to UseCase --------------------------------------
KEY_PAYLOAD=$(cat <<JSON
{"name":"otel-demo-key-$$","key_type":"api","use_case_id":"${UC_ID}"}
JSON
)
KEY_RESP=$(curl -s -X POST -H "$H_AUTH" -H "$H_JSON" -d "$KEY_PAYLOAD" "${BASE_URL}/api/admin/keys")
KEY_ID=$(echo "$KEY_RESP" | jq -r '.id // empty')
if [ -n "$KEY_ID" ]; then
  ok "key created (${KEY_ID})"
else
  fail "key create — response: $KEY_RESP"
  exit 1
fi

# ---- 4. POST synthetic OTLP ----------------------------------------------
SVC_NAME="fraud-investigator-test-$$"
otlp_payload() {
  cat <<JSON
{
  "resourceSpans":[{
    "resource":{"attributes":[
      {"key":"service.name","value":{"stringValue":"${SVC_NAME}"}},
      {"key":"service.version","value":{"stringValue":"0.1.0"}},
      {"key":"pllm.key_id","value":{"stringValue":"${KEY_ID}"}}
    ]},
    "scopeSpans":[{
      "scope":{"name":"test"},
      "spans":[
        {"name":"openai.chat","kind":3,"startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[
          {"key":"gen_ai.system","value":{"stringValue":"openai"}},
          {"key":"gen_ai.request.model","value":{"stringValue":"openai/gpt-4o"}}
        ]},
        {"name":"qdrant.search","kind":3,"startTimeUnixNano":"3","endTimeUnixNano":"4","attributes":[
          {"key":"db.system","value":{"stringValue":"qdrant"}},
          {"key":"db.collection.name","value":{"stringValue":"customers"}}
        ]}
      ]
    }]
  }]
}
JSON
}
HTTP1=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces")
if [ "$HTTP1" = "200" ]; then
  ok "otlp batch 1 posted (200)"
else
  fail "otlp batch 1 returned ${HTTP1}"
fi

# ---- 5. Verify DiscoveredAgent appears -----------------------------------
sleep 1
AGENT_LIST=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/discovery/agents?status=pending")
AGENT_ID=$(echo "$AGENT_LIST" | jq -r --arg svc "$SVC_NAME" \
  '.items[] | select(.service_name==$svc) | .id' | head -1)
if [ -n "$AGENT_ID" ] && [ "$AGENT_ID" != "null" ]; then
  ok "discovered agent listed (${AGENT_ID})"
else
  fail "discovered agent missing — list: $AGENT_LIST"
  exit 1
fi

# ---- 6. Verify Qdrant call is flagged ------------------------------------
DETAIL=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/discovery/agents/${AGENT_ID}")
QDRANT_OUTCOME=$(echo "$DETAIL" | jq -r '.calls[] | select(.kind=="vector_db") | .last_outcome' | head -1)
if [ "$QDRANT_OUTCOME" = "flagged" ]; then
  ok "qdrant call flagged"
else
  fail "qdrant call not flagged (got: ${QDRANT_OUTCOME}) — detail: $DETAIL"
fi

# ---- 7. Promote ----------------------------------------------------------
PROM=$(curl -s -X POST -H "$H_AUTH" -H "$H_JSON" -d '{"version":"0.1.0"}' \
  "${BASE_URL}/api/admin/discovery/agents/${AGENT_ID}/promote")
REG_ID=$(echo "$PROM" | jq -r '.registry_agent_id // empty')
if [ -n "$REG_ID" ]; then
  ok "promoted to registry agent (${REG_ID})"
else
  fail "promote failed — resp: $PROM"
fi

# ---- 8. Two more batches → 3 flags total → key revoked ------------------
note "posting two more OTLP batches to push flagged_count to threshold"
curl -s -o /dev/null -X POST -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces"
sleep 1
curl -s -o /dev/null -X POST -H "$H_JSON" -d "$(otlp_payload)" "${BASE_URL}/v1/traces"
sleep 2

KEY_AFTER=$(curl -s -H "$H_AUTH" "${BASE_URL}/api/admin/keys/${KEY_ID}")
ACTIVE=$(echo "$KEY_AFTER" | jq -r '.is_active')
REVOKED=$(echo "$KEY_AFTER" | jq -r '.revoked_at // empty')
if [ "$ACTIVE" = "false" ] && [ -n "$REVOKED" ]; then
  ok "key auto-revoked after threshold (revoked_at=${REVOKED})"
else
  fail "key still active — is_active=${ACTIVE}, revoked_at=${REVOKED}"
fi

# ---- summary --------------------------------------------------------------
echo
printf "%sSummary: %s%d passed%s, %s%d failed%s\n" \
  "$BOLD" "$GREEN" "$PASS" "$RESET" "$RED" "$FAIL" "$RESET"
if [ "$FAIL" -gt 0 ]; then
  echo
  echo "Failures:"
  for f in "${FAILURES[@]}"; do echo "  - $f"; done
fi
exit "$FAIL"
