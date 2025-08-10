#!/bin/bash

echo "Testing caching functionality..."
echo ""

# Start the server in background
echo "Starting pllm server..."
PLLM_LITE_MODE=true CONFIG_FILE=config.lite.yaml ./pllm &
SERVER_PID=$!
sleep 2

# Test endpoint
URL="http://localhost:8080/v1/models"

echo "Making first request (should be cached)..."
START_TIME=$(date +%s%N)
response1=$(curl -s -D headers1.txt $URL)
END_TIME=$(date +%s%N)
DURATION1=$((($END_TIME - $START_TIME) / 1000000))
echo "First request took: ${DURATION1}ms"

# Check for cache headers
cache_status1=$(grep -i "X-Cache" headers1.txt || echo "X-Cache: MISS")
echo "Cache status: $cache_status1"

echo ""
echo "Making second request (should hit cache)..."
START_TIME=$(date +%s%N)
response2=$(curl -s -D headers2.txt $URL)
END_TIME=$(date +%s%N)
DURATION2=$((($END_TIME - $START_TIME) / 1000000))
echo "Second request took: ${DURATION2}ms"

# Check for cache headers
cache_status2=$(grep -i "X-Cache" headers2.txt || echo "X-Cache: MISS")
age=$(grep -i "Age" headers2.txt || echo "Age: 0")
echo "Cache status: $cache_status2"
echo "Cache age: $age"

echo ""
# Compare responses
if [ "$response1" = "$response2" ]; then
    echo "✓ Responses are identical"
else
    echo "✗ Responses differ (cache might not be working)"
fi

# Check if second request was faster
if [ $DURATION2 -lt $DURATION1 ]; then
    echo "✓ Second request was faster (${DURATION2}ms vs ${DURATION1}ms)"
    speedup=$((100 * ($DURATION1 - $DURATION2) / $DURATION1))
    echo "  Speedup: ${speedup}%"
else
    echo "⚠ Second request was not faster"
fi

# Check cache headers
if grep -q "X-Cache: HIT" headers2.txt 2>/dev/null; then
    echo "✓ Cache headers indicate a cache hit"
else
    echo "⚠ No cache hit detected in headers"
fi

# Cleanup
rm -f headers1.txt headers2.txt
kill $SERVER_PID 2>/dev/null

echo ""
echo "Cache test complete!"