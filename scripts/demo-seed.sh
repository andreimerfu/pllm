#!/usr/bin/env bash
#
# demo-seed.sh — idempotent fixtures for the Phase 1+2 stakeholder demo.
#
# Creates a "customer-research-approved" UseCase with refs allowing the
# echo + research agents and test-model-allowed (but NOT test-model-
# restricted, which is the denial story the research agent exercises).
# Publishes two RegistryAgents pointing at the pre-built OCI images:
#   - pllm/agent-echo:dev       (built by `make agent-echo-image-minikube`)
#   - pllm/agent-research:dev   (built by `make agent-research-image-minikube`)
#
# By default the script STOPS BEFORE DEPLOYING so the presenter can click
# "Deploy agent" in the UI as part of the demo. Pass --deploy to also
# spin the pods up automatically (handy for a dry-run / skipping that step).
#
# Pre-reqs:
#   - pllm running at $PLLM_URL
#   - agent-echo and agent-research images available in the cluster
#     (via `make agent-echo-image-minikube` + `make agent-research-image-minikube`)
#   - $PLLM_MASTER_KEY exported (default: sk-master-test)
#
# Usage:
#   scripts/demo-seed.sh              # seed only
#   scripts/demo-seed.sh --deploy     # seed + auto-deploy
#   scripts/demo-seed.sh --cleanup    # tear everything down
#   scripts/demo-seed.sh --help

set -euo pipefail

PLLM_URL="${PLLM_URL:-http://localhost:8080}"
PLLM_MASTER_KEY="${PLLM_MASTER_KEY:-sk-master-test}"
AUTH=(-H "Authorization: Bearer ${PLLM_MASTER_KEY}")
CONTENT=(-H "Content-Type: application/json")

USE_CASE_SLUG="customer-research-approved"
AGENT_ECHO_NAME="pllm.io/agent-echo"
AGENT_RESEARCH_NAME="pllm.io/agent-research"
AGENT_VERSION="0.1.0"

# Flags.
MODE="seed"  # seed | deploy | cleanup
for arg in "$@"; do
  case "$arg" in
    --deploy)  MODE="deploy"   ;;
    --cleanup) MODE="cleanup"  ;;
    --help|-h)
      sed -n '2,25p' "$0"
      exit 0
      ;;
    *)
      echo "unknown flag: $arg" >&2
      exit 1
      ;;
  esac
done

step() { printf "\n\033[1;36m== %s ==\033[0m\n" "$1"; }
info() { printf "  \033[0;90m%s\033[0m\n" "$1"; }
ok()   { printf "  \033[0;32m✓\033[0m %s\n" "$1"; }

# --- helpers --------------------------------------------------------------

# URL-encode a string without requiring python3 — pure jq.
urlencode() {
  jq -rn --arg v "$1" '$v | @uri'
}

# Returns the UUID of the RegistryAgent for name+version, or empty string.
# GET /v1/registry/agents/{name}?version=X → AgentWithRefs → .agent.id
lookup_agent() {
  local name="$1"
  local encoded
  encoded=$(urlencode "$name")
  curl -fsS "${AUTH[@]}" \
    "${PLLM_URL}/v1/registry/agents/${encoded}?version=${AGENT_VERSION}" \
    2>/dev/null \
    | jq -r '.agent.id // empty' 2>/dev/null || true
}

# Returns the UUID of the UseCase matching the given slug, or empty string.
# GET /api/admin/use-cases → {use_cases: [...]} — items are flat UseCase rows.
lookup_use_case() {
  curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/use-cases" \
    | jq -r --arg slug "$1" '.use_cases[]? | select(.slug == $slug) | .id' \
    | head -n1
}

# --- seed -----------------------------------------------------------------

seed() {
  step "Health"
  curl -fsS "${PLLM_URL}/health" >/dev/null
  ok "pllm is reachable at ${PLLM_URL}"

  # ----- RegistryAgents: publish if not present -------------------------

  step "Publish RegistryAgent: echo"
  ECHO_ID=$(lookup_agent "$AGENT_ECHO_NAME")
  if [ -z "$ECHO_ID" ]; then
    # POST /api/admin/registry/agents → AgentWithRefs → .agent.id
    ECHO_ID=$(curl -fsS "${AUTH[@]}" "${CONTENT[@]}" \
      "${PLLM_URL}/api/admin/registry/agents" \
      -d "$(jq -n --arg n "$AGENT_ECHO_NAME" --arg v "$AGENT_VERSION" '{
        name: $n, version: $v, title: "Echo",
        description: "Minimal reference agent for the pllm Phase 2 demo",
        is_latest: true,
        packages: [{registry_type: "oci", identifier: "pllm/agent-echo", version: "dev"}]
      }')" | jq -r '.agent.id')
    ok "published ${AGENT_ECHO_NAME} v${AGENT_VERSION} (id=${ECHO_ID})"
  else
    info "${AGENT_ECHO_NAME} v${AGENT_VERSION} already exists (id=${ECHO_ID})"
  fi

  step "Publish RegistryAgent: research"
  RESEARCH_ID=$(lookup_agent "$AGENT_RESEARCH_NAME")
  if [ -z "$RESEARCH_ID" ]; then
    # POST /api/admin/registry/agents → AgentWithRefs → .agent.id
    RESEARCH_ID=$(curl -fsS "${AUTH[@]}" "${CONTENT[@]}" \
      "${PLLM_URL}/api/admin/registry/agents" \
      -d "$(jq -n --arg n "$AGENT_RESEARCH_NAME" --arg v "$AGENT_VERSION" '{
        name: $n, version: $v, title: "Research",
        description: "Reference agent that exercises scope enforcement",
        is_latest: true,
        packages: [{registry_type: "oci", identifier: "pllm/agent-research", version: "dev"}]
      }')" | jq -r '.agent.id')
    ok "published ${AGENT_RESEARCH_NAME} v${AGENT_VERSION} (id=${RESEARCH_ID})"
  else
    info "${AGENT_RESEARCH_NAME} v${AGENT_VERSION} already exists (id=${RESEARCH_ID})"
  fi

  # ----- UseCase: create or update with refs ----------------------------

  step "UseCase: ${USE_CASE_SLUG}"
  UC_BODY=$(jq -n \
    --arg slug "$USE_CASE_SLUG" \
    --arg echo_name "$AGENT_ECHO_NAME" \
    --arg research_name "$AGENT_RESEARCH_NAME" '{
      name: "customer-research-approved",
      slug: $slug,
      description: "Demo use case for the Phase 1+2 stakeholder walkthrough",
      enforce_scope: true,
      refs: [
        {target_kind: "agent", target_name: $echo_name},
        {target_kind: "agent", target_name: $research_name},
        {target_kind: "llm",   target_name: "test-model-allowed"}
      ]
    }')

  UC_ID=$(lookup_use_case "$USE_CASE_SLUG")
  if [ -z "$UC_ID" ]; then
    # POST /api/admin/use-cases → WithRefs → .use_case.id
    UC_ID=$(curl -fsS "${AUTH[@]}" "${CONTENT[@]}" \
      "${PLLM_URL}/api/admin/use-cases" \
      -d "$UC_BODY" | jq -r '.use_case.id')
    ok "created use case (id=${UC_ID})"
  else
    # PUT replaces refs wholesale.
    curl -fsS "${AUTH[@]}" "${CONTENT[@]}" -X PUT \
      "${PLLM_URL}/api/admin/use-cases/${UC_ID}" -d "$UC_BODY" >/dev/null
    ok "updated use case (id=${UC_ID}) with refs"
  fi

  # ----- Optional deploy ------------------------------------------------

  if [ "$MODE" = "deploy" ]; then
    step "Deploy echo agent"
    curl -fsS "${AUTH[@]}" "${CONTENT[@]}" "${PLLM_URL}/api/admin/agents" \
      -d "$(jq -n --arg a "$ECHO_ID" --arg u "$UC_ID" \
        '{agent_id: $a, use_case_id: $u, replicas: 1}')" \
      | jq -r '"  ✓ agent deployment id=" + .id'

    step "Deploy research agent"
    curl -fsS "${AUTH[@]}" "${CONTENT[@]}" "${PLLM_URL}/api/admin/agents" \
      -d "$(jq -n --arg a "$RESEARCH_ID" --arg u "$UC_ID" \
        '{agent_id: $a, use_case_id: $u, replicas: 1}')" \
      | jq -r '"  ✓ agent deployment id=" + .id'
  fi

  # ----- Summary --------------------------------------------------------

  step "Done"
  cat <<EOF

  use_case_id   = ${UC_ID}
  echo_id       = ${ECHO_ID}
  research_id   = ${RESEARCH_ID}

  Next:
    1. Open ${PLLM_URL}/ui
    2. Go to the Agents page
    3. Click "Deploy agent" → pick ${AGENT_ECHO_NAME} + ${USE_CASE_SLUG} → Deploy
    4. Repeat for ${AGENT_RESEARCH_NAME}
    5. Once the pods are running, click the row dropdown → Invoke
    6. Visit Compliance → Scope denials to see the research agent's
       restricted-model probe land as an audit event
EOF
}

# --- cleanup --------------------------------------------------------------

cleanup() {
  step "Cleanup: agent deployments"
  AGENTS_JSON=$(curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/agents")
  echo "$AGENTS_JSON" | jq -r '.agents[]?.id' | while read -r id; do
    curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/agents/${id}" >/dev/null || true
    ok "undeployed ${id}"
  done

  step "Cleanup: RegistryAgents"
  for n in "$AGENT_ECHO_NAME" "$AGENT_RESEARCH_NAME"; do
    encoded=$(urlencode "$n")
    curl -fsS "${AUTH[@]}" -X DELETE \
      "${PLLM_URL}/api/admin/registry/agents/${encoded}/versions/${AGENT_VERSION}" \
      >/dev/null 2>&1 || true
    ok "deleted ${n} v${AGENT_VERSION}"
  done

  step "Cleanup: UseCase"
  UC_ID=$(lookup_use_case "$USE_CASE_SLUG")
  if [ -n "$UC_ID" ]; then
    curl -fsS "${AUTH[@]}" -X DELETE \
      "${PLLM_URL}/api/admin/use-cases/${UC_ID}" >/dev/null
    ok "deleted ${USE_CASE_SLUG}"
  else
    info "${USE_CASE_SLUG} not found; nothing to delete"
  fi

  step "Done"
}

# --- main -----------------------------------------------------------------

case "$MODE" in
  seed|deploy) seed ;;
  cleanup)     cleanup ;;
  *)           echo "unknown mode: $MODE" >&2; exit 1 ;;
esac
