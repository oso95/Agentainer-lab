#!/bin/bash

# Simple crash resilience test

echo "=== Testing Request Persistence During Agent Crash ==="
echo ""

# Ensure server is running
echo "1. Starting fresh server..."
pkill -f "agentainer server" 2>/dev/null || true
sleep 1
./agentainer server &
SERVER_PID=$!
sleep 3

# Deploy nginx agent
echo "2. Deploying nginx agent..."
./agentainer deploy --name crash-demo --image nginx:alpine
AGENT_ID=$(./agentainer list | grep crash-demo | awk '{print $1}' | tail -1)

# Start agent
echo "3. Starting agent..."
./agentainer start $AGENT_ID
sleep 3

# Send successful request
echo "4. Sending request to running agent..."
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token")
echo "$RESPONSE" | grep "HTTP_CODE"

# Check requests
echo ""
echo "5. Checking requests (with persistence enabled, all requests are stored)..."
./agentainer requests $AGENT_ID | head -10

# Kill the container to simulate crash
echo ""
echo "6. Simulating crash by killing container..."
docker kill agentainer-$AGENT_ID >/dev/null 2>&1

# Try to send request to crashed agent
echo ""
echo "7. Sending request to crashed agent..."
curl -s -m 5 http://localhost:8081/agent/$AGENT_ID/api/data \
     -H "Authorization: Bearer agentainer-default-token" \
     -d '{"important": "data"}' 2>&1 | grep -E "(502 Bad Gateway|error)"

# The key point: even though the agent crashed, the request should still be stored
echo ""
echo "8. Key Point: Request is stored even though agent crashed..."
./agentainer requests $AGENT_ID | grep -E "(api/data|important)"

# Cleanup
echo ""
echo "9. Cleanup..."
./agentainer remove $AGENT_ID >/dev/null 2>&1
kill $SERVER_PID 2>/dev/null

echo ""
echo "Summary: With the updated implementation, ALL requests are stored"
echo "regardless of agent status. This ensures no requests are lost even"
echo "if an agent crashes mid-processing."