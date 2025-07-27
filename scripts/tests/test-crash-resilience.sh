#!/bin/bash

# Test script for crash resilience - what happens when agent crashes mid-request

echo "=== Agentainer Crash Resilience Test ==="
echo ""

# Start server
echo "1. Starting server with request persistence..."
./agentainer server &
SERVER_PID=$!
sleep 3

# Deploy a simple HTTP server that can be killed
echo "2. Deploying test agent..."
./agentainer deploy --name crash-test --image python:3.9-slim
AGENT_ID=$(./agentainer list | grep crash-test | awk '{print $1}' | tail -1)
echo "   Agent ID: $AGENT_ID"

# Start the agent with a simple HTTP server
echo ""
echo "3. Starting agent with simple HTTP server..."
./agentainer start $AGENT_ID
sleep 2

# Run a simple HTTP server in the container
docker exec agentainer-$AGENT_ID python -m http.server 8000 &
PYTHON_PID=$!
sleep 3

# Send a request that will succeed
echo ""
echo "4. Sending test request (should succeed)..."
curl -s http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" | head -5

# Check requests - should be marked as completed
echo ""
echo "5. Checking requests (should show completed)..."
./agentainer requests $AGENT_ID | head -10

# Now simulate a crash by killing the container
echo ""
echo "6. Simulating agent crash..."
docker kill agentainer-$AGENT_ID

# Send request while crashed
echo ""
echo "7. Sending request to crashed agent..."
curl -s -m 5 http://localhost:8081/agent/$AGENT_ID/test \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -d '{"data": "test"}' || echo "Request failed (expected)"

# Check pending requests
echo ""
echo "8. Checking pending requests..."
./agentainer requests $AGENT_ID | grep -E "(Pending|ID:|Method:|Status:)"

# Restart the agent
echo ""
echo "9. Restarting agent..."
./agentainer start $AGENT_ID
sleep 3

# Start HTTP server again
docker exec agentainer-$AGENT_ID python -m http.server 8000 &
sleep 3

# Check if request was replayed
echo ""
echo "10. Checking if request was replayed..."
sleep 10
./agentainer requests $AGENT_ID | grep -E "(Pending|ID:|Method:|Status:|No pending)"

# Cleanup
echo ""
echo "11. Cleaning up..."
./agentainer stop $AGENT_ID 2>/dev/null || true
./agentainer remove $AGENT_ID
kill $SERVER_PID 2>/dev/null

echo ""
echo "Test complete!"