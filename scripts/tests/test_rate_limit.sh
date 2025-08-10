#!/bin/bash

echo "Testing rate limiting..."
echo "Rate limit is set to 60 requests per minute (1 per second)"
echo ""

# Test endpoint
URL="http://localhost:8080/v1/models"

echo "Making rapid requests to test rate limiting..."
for i in {1..10}; do
  echo -n "Request $i: "
  response=$(curl -s -o /dev/null -w "%{http_code}" -D - $URL 2>/dev/null | head -n1)
  
  # Extract rate limit headers
  headers=$(curl -s -D - $URL 2>/dev/null)
  limit=$(echo "$headers" | grep -i "X-RateLimit-Limit" | cut -d: -f2 | tr -d ' \r')
  remaining=$(echo "$headers" | grep -i "X-RateLimit-Remaining" | cut -d: -f2 | tr -d ' \r')
  
  echo "Status: $response | Limit: $limit | Remaining: $remaining"
  
  # Small delay to make output readable
  sleep 0.1
done

echo ""
echo "Testing rate limit exceeded..."
echo "Making 70 rapid requests (exceeding the 60/min limit)..."

success=0
rate_limited=0

for i in {1..70}; do
  response=$(curl -s -o /dev/null -w "%{http_code}" $URL)
  if [ "$response" = "200" ]; then
    ((success++))
  elif [ "$response" = "429" ]; then
    ((rate_limited++))
    echo "Rate limit hit at request $i!"
    break
  fi
done

echo ""
echo "Summary:"
echo "  Successful requests: $success"
echo "  Rate limited requests: $rate_limited"

if [ $rate_limited -gt 0 ]; then
  echo "✓ Rate limiting is working correctly!"
else
  echo "⚠ Rate limiting may not be working as expected"
fi