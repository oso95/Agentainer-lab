#!/bin/bash

# Test script for request persistence feature

set -e

echo "Testing Agentainer Request Persistence..."
echo "========================================"

# Clean up any existing test agent
./agentainer remove test-persist 2>/dev/null || true

# Start the server in background
echo "1. Starting Agentainer server with request persistence..."
./agentainer server &
SERVER_PID=$!
sleep 5

# Deploy an agent but don't start it
echo "2. Deploying test agent (not starting yet)..."
./agentainer deploy --name test-persist --image nginx:alpine

# Get agent ID
AGENT_ID=$(./agentainer list | grep test-persist | awk '{print $1}')
echo "   Agent ID: $AGENT_ID"

# Send requests while agent is stopped
echo "3. Sending requests while agent is stopped..."
echo "   Request 1:"
curl -X GET http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -v 2>&1 | grep -E "(HTTP/|request_id)"

echo ""
echo "   Request 2:"
curl -X POST http://localhost:8081/agent/$AGENT_ID/test \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -d '{"message": "Hello from test"}' \
     -v 2>&1 | grep -E "(HTTP/|request_id)"

# Check pending requests
echo ""
echo "4. Checking pending requests..."
./agentainer requests $AGENT_ID

# Now start the agent
echo ""
echo "5. Starting agent (requests should replay automatically)..."
./agentainer start $AGENT_ID
sleep 5

# Check if requests were processed
echo ""
echo "6. Checking requests after agent start..."
./agentainer requests $AGENT_ID

# Check agent logs to see if requests were received
echo ""
echo "7. Agent logs (should show replayed requests):"
./agentainer logs $AGENT_ID | tail -20

# Cleanup
echo ""
echo "8. Cleaning up..."
./agentainer stop $AGENT_ID
./agentainer remove $AGENT_ID
kill $SERVER_PID

echo ""
echo "Test completed!"