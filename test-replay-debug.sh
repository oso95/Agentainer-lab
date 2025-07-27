#!/bin/bash

# Debug script for request replay

echo "Request Replay Debug Test"
echo "========================"

# Start server with request persistence
echo "1. Starting server..."
./agentainer server &
SERVER_PID=$!
sleep 3

# Deploy and get agent
echo "2. Deploying agent..."
./agentainer deploy --name test-replay --image nginx:alpine
AGENT_ID=$(./agentainer list | grep test-replay | awk '{print $1}' | tail -1)
echo "   Agent ID: $AGENT_ID"

# Send request while stopped
echo "3. Sending request while agent is stopped..."
curl -X GET http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" \
     -s | python3 -m json.tool

# Check pending
echo ""
echo "4. Checking pending requests..."
./agentainer requests $AGENT_ID | grep -E "(ID:|Status:|Method:)"

# Start agent and wait for replay
echo ""
echo "5. Starting agent..."
./agentainer start $AGENT_ID
echo "   Waiting 10 seconds for replay worker..."
sleep 10

# Check again
echo ""
echo "6. Checking requests after wait..."
./agentainer requests $AGENT_ID | grep -E "(ID:|Status:|Method:|No pending)"

# Cleanup
echo ""
echo "7. Cleaning up..."
./agentainer stop $AGENT_ID
./agentainer remove $AGENT_ID
kill $SERVER_PID 2>/dev/null

echo ""
echo "Debug test completed!"