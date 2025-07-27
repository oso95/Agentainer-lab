#!/bin/bash

# Final test script for request persistence and replay

echo "=== Agentainer Request Persistence & Replay Test ==="
echo ""

# Start server
echo "1. Starting server with request persistence..."
./agentainer server &
SERVER_PID=$!
sleep 3

# Deploy agent
echo "2. Deploying nginx agent (not started)..."
./agentainer deploy --name persist-test --image nginx:alpine
AGENT_ID=$(./agentainer list | grep persist-test | awk '{print $1}' | tail -1)
echo "   Agent ID: $AGENT_ID"

# Send requests while stopped
echo ""
echo "3. Sending 2 requests while agent is stopped..."
echo "   Request 1 (GET /):"
curl -s -X GET http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" | python3 -m json.tool | grep -E "(success|message|request_id)"

echo ""
echo "   Request 2 (POST /api/test):"
curl -s -X POST http://localhost:8081/agent/$AGENT_ID/api/test \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -d '{"test": "data"}' | python3 -m json.tool | grep -E "(success|message|request_id)"

# Show pending requests
echo ""
echo "4. Checking pending requests (should show 2)..."
./agentainer requests $AGENT_ID | grep -E "(Pending requests|ID:|Method:|Status:|No pending)"

# Start agent
echo ""
echo "5. Starting agent to trigger replay..."
./agentainer start $AGENT_ID
echo "   Waiting for replay worker (15 seconds)..."
sleep 15

# Check requests again
echo ""
echo "6. Checking requests after replay (should be processed)..."
./agentainer requests $AGENT_ID | grep -E "(Pending requests|ID:|Method:|Status:|No pending|completed)"

# Send live request to verify agent is working
echo ""
echo "7. Sending live request to verify agent is responding..."
curl -s -I http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" | head -3

# Cleanup
echo ""
echo "8. Cleaning up..."
./agentainer stop $AGENT_ID
./agentainer remove $AGENT_ID
kill $SERVER_PID 2>/dev/null

echo ""
echo "Test complete!"