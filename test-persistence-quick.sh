#!/bin/bash

# Quick test script for request persistence feature

set -e

echo "Testing Agentainer Request Persistence..."
echo "========================================"

# Clean up any existing test agent
./agentainer remove test-persist 2>/dev/null || true

# Deploy an agent but don't start it
echo "1. Deploying test agent (not starting yet)..."
./agentainer deploy --name test-persist --image nginx:alpine

# Get agent ID - extract just the ID from the output
AGENT_ID=$(./agentainer list | grep test-persist | awk '{print $1}' | tail -1)
echo "   Agent ID: $AGENT_ID"

# Send requests while agent is stopped
echo ""
echo "2. Sending requests while agent is stopped..."
echo "   Request 1:"
curl -X GET http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -w "\n" 2>/dev/null

echo ""
echo "   Request 2:"
curl -X POST http://localhost:8081/agent/$AGENT_ID/test \
     -H "Authorization: Bearer agentainer-default-token" \
     -H "Content-Type: application/json" \
     -d '{"message": "Hello from test"}' \
     -w "\n" 2>/dev/null

# Check pending requests
echo ""
echo "3. Checking pending requests..."
./agentainer requests $AGENT_ID

# Now start the agent
echo ""
echo "4. Starting agent (requests should replay automatically)..."
./agentainer start $AGENT_ID
sleep 3

# Check if requests were processed
echo ""
echo "5. Checking requests after agent start..."
./agentainer requests $AGENT_ID

# Test live request
echo ""
echo "6. Sending live request to running agent..."
curl -X GET http://localhost:8081/agent/$AGENT_ID/ \
     -H "Authorization: Bearer agentainer-default-token" \
     -w "\n" 2>/dev/null | head -5

# Cleanup
echo ""
echo "7. Cleaning up..."
./agentainer stop $AGENT_ID
./agentainer remove $AGENT_ID

echo ""
echo "Test completed!"