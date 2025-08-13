#!/bin/bash

# Test script for PLLM authentication and budget system

API_URL="http://localhost:8080"
ADMIN_URL="http://localhost:8081"

echo "==================================="
echo "PLLM Authentication Test Script"
echo "==================================="

# Test 1: Health check (no auth required)
echo -e "\n[1] Testing health endpoint..."
curl -s ${API_URL}/health | jq '.' || echo "Health check failed"

# Test 2: Try accessing API without auth (should fail)
echo -e "\n[2] Testing API without authentication (should fail)..."
curl -s -w "\nHTTP Status: %{http_code}\n" ${API_URL}/api/models | jq '.' 2>/dev/null || echo "Correctly rejected"

# Test 3: Test with Master Key
echo -e "\n[3] Testing with Master Key..."
MASTER_KEY="sk-master-change-this-in-production"
curl -s -H "Authorization: Bearer ${MASTER_KEY}" ${API_URL}/api/models | jq '.' || echo "Master key auth failed"

# Test 4: Test with Virtual Key (from seed data)
echo -e "\n[4] Testing with Virtual Key..."
VIRTUAL_KEY="sk-admin-full-access-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
curl -s -H "Authorization: Bearer ${VIRTUAL_KEY}" ${API_URL}/api/models | jq '.' || echo "Virtual key auth failed"

# Test 5: Create a new team
echo -e "\n[5] Creating a new team..."
curl -s -X POST ${ADMIN_URL}/admin/teams \
  -H "Authorization: Bearer ${MASTER_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Team",
    "description": "Test team for authentication",
    "max_budget": 1000,
    "budget_duration": "monthly",
    "tpm": 100000,
    "rpm": 100,
    "allowed_models": ["gpt-3.5-turbo", "gpt-4"]
  }' | jq '.'

# Test 6: Create a new virtual key
echo -e "\n[6] Creating a new virtual key..."
curl -s -X POST ${ADMIN_URL}/admin/keys \
  -H "Authorization: Bearer ${MASTER_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test API Key",
    "user_id": "11111111-1111-1111-1111-111111111111",
    "max_budget": 100,
    "budget_duration": "daily",
    "tpm": 10000,
    "rpm": 10,
    "allowed_models": ["gpt-3.5-turbo"]
  }' | jq '.'

# Test 7: Test rate limiting
echo -e "\n[7] Testing rate limiting..."
echo "Sending 5 rapid requests..."
for i in {1..5}; do
  echo -n "Request $i: "
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer ${VIRTUAL_KEY}" \
    ${API_URL}/api/models
  sleep 0.1
done

# Test 8: Test budget tracking
echo -e "\n[8] Testing budget tracking..."
curl -s -X GET ${ADMIN_URL}/admin/budgets \
  -H "Authorization: Bearer ${MASTER_KEY}" | jq '.'

# Test 9: Test SSO login URL
echo -e "\n[9] SSO Login URLs:"
echo "  GitHub: ${API_URL}/auth/login?connector=github"
echo "  Google: ${API_URL}/auth/login?connector=google"
echo "  Mock: ${API_URL}/auth/login?connector=mock"

echo -e "\n==================================="
echo "Test complete!"
echo "==================================="
echo ""
echo "Next steps:"
echo "1. Start services: docker-compose -f deploy/docker-compose.auth.yml up"
echo "2. Access Web UI: http://localhost:3000"
echo "3. Login with: admin@pllm.local / admin123"
echo "4. Or use SSO providers configured in Dex"