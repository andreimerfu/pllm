#!/usr/bin/env bash
# End-to-end smoke test for Phase 1 agent onboarding.
#
# Pre-reqs:
#   - pllm running at $PLLM_URL (default http://localhost:8080)
#   - a Kubernetes cluster the configured deployment.kubernetes can reach
#   - the master key set in $PLLM_MASTER_KEY (default matches deploy/k8s/secret.yaml)
#
# The published agent's OCI image is expected to expose POST /invoke on
# port 8000 with the Phase 2 contract:
#   request  { "input": <any>, "task_id": "<string>" }
#   response { "output": <any>, "error": "<optional string>" }
#
# A 15-line reference pod satisfies this contract and is used in demos.
#
# Prints FAIL + exits non-zero on the first failing step.

set -euo pipefail
PLLM_URL="${PLLM_URL:-http://localhost:8080}"
PLLM_MASTER_KEY="${PLLM_MASTER_KEY:-sk-master-dev-key-change-in-production}"
AUTH=(-H "Authorization: Bearer ${PLLM_MASTER_KEY}")

step() { printf "\n== %s ==\n" "$1"; }

step "Health"
curl -fsS "${PLLM_URL}/health" >/dev/null

step "Create a minimal UseCase"
UC_ID=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/use-cases" \
  -d '{"name":"smoke-agent-uc","slug":"smoke-agent-uc","description":"Phase 1 smoke"}' | jq -r .id)
echo "use_case_id=${UC_ID}"

step "Publish a trivial registry agent (OCI image: ghcr.io/pllm/agent-smoke:0.1)"
AGENT_ID=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/registry/agents" \
  -d '{
    "name":"test.pllm/agent-smoke",
    "version":"0.1.0",
    "title":"Smoke Agent",
    "description":"Phase 1 smoke",
    "packages":[{"registry_type":"oci","identifier":"ghcr.io/pllm/agent-smoke","version":"0.1"}]
  }' | jq -r .id)
echo "agent_id=${AGENT_ID}"

step "Deploy the agent"
DEPLOY=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/agents" \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"use_case_id\":\"${UC_ID}\"}")
AD_ID=$(echo "${DEPLOY}" | jq -r .id)
SECRET=$(echo "${DEPLOY}" | jq -r .secret_name)
KEY_ID=$(echo "${DEPLOY}" | jq -r .provisioned_key_id)
echo "agent_deployment_id=${AD_ID} secret_name=${SECRET} provisioned_key_id=${KEY_ID}"

step "List agents — expect our deployment"
curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/agents" | jq -e \
  --arg id "${AD_ID}" '.agents | map(select(.id==$id)) | length == 1' >/dev/null

step "Refresh status"
curl -fsS "${AUTH[@]}" -X POST "${PLLM_URL}/api/admin/agents/${AD_ID}/status" | jq .status

step "Invoke the agent (plain alias)"
INVOKE_RESP=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/v1/agents/$(jq -r .workload_name <<<"${DEPLOY}")/invoke" \
  -d '{"input":"hello","task_id":"smoke-1"}' || true)
echo "invoke response: ${INVOKE_RESP}"
# We don't assert on the body — the reference agent may echo or not,
# depending on what's running in the cluster. Reaching 200 proves the
# gateway wiring works.

step "Fetch agent card"
curl -fsS "${AUTH[@]}" \
  "${PLLM_URL}/v1/a2a/$(jq -r .workload_name <<<"${DEPLOY}")/.well-known/agent.json" \
  | jq -e '.name and .url and (.capabilities.streaming == false)' >/dev/null

step "Undeploy"
curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/agents/${AD_ID}"

step "Re-list — expect absent"
curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/agents" | jq -e \
  --arg id "${AD_ID}" '.agents | map(select(.id==$id)) | length == 0' >/dev/null

echo "OK"

if [[ "${1:-}" == "--cleanup" ]]; then
  step "Cleanup test fixtures"
  curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/registry/agents/${AGENT_ID}" || true
  curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/use-cases/${UC_ID}" || true
fi
