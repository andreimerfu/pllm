#!/bin/bash

echo "Simple cache test for /v1/models endpoint"
echo ""

# Start server
PLLM_LITE_MODE=true CONFIG_FILE=config.lite.yaml ./pllm > /dev/null 2>&1 &
PID=$!
sleep 2

# Make multiple requests
echo "Request 1:"
response=$(curl -s -D - http://localhost:8080/v1/models)
echo "$response" | head -n1
echo "$response" | grep -i "X-Cache" || echo "X-Cache: (not set)"
echo "$response" | grep -i "Age" || echo "Age: (not set)"

echo ""
echo "Request 2 (should be cached):"
response=$(curl -s -D - http://localhost:8080/v1/models)
echo "$response" | head -n1
echo "$response" | grep -i "X-Cache" || echo "X-Cache: (not set)"
echo "$response" | grep -i "Age" || echo "Age: (not set)"

echo ""
echo "Request 3 (should be cached):"
response=$(curl -s -D - http://localhost:8080/v1/models)
echo "$response" | head -n1
echo "$response" | grep -i "X-Cache" || echo "X-Cache: (not set)"
echo "$response" | grep -i "Age" || echo "Age: (not set)"

# Cleanup
kill $PID 2>/dev/null
wait $PID 2>/dev/null

echo ""
echo "Test complete"