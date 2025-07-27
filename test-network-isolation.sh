#!/bin/bash

# Test script for network isolation feature

set -e

echo "Testing Agentainer Network Isolation..."
echo "======================================="

# Start the server
echo "1. Starting Agentainer server..."
./agentainer server &
SERVER_PID=$!
sleep 5

# Deploy a simple nginx agent
echo "2. Deploying test agent..."
./agentainer deploy --name test-nginx --image nginx:alpine

# Get agent ID
AGENT_ID=$(./agentainer list | grep test-nginx | awk '{print $1}')
echo "   Agent ID: $AGENT_ID"

# Start the agent
echo "3. Starting agent..."
./agentainer start $AGENT_ID
sleep 3

# Test proxy access
echo "4. Testing proxy access..."
curl -s http://localhost:8081/agent/$AGENT_ID/ | head -5

# Check Docker network
echo "5. Verifying network isolation..."
docker network ls | grep agentainer
docker ps --filter "label=agentainer.id=$AGENT_ID" --format "table {{.Names}}\t{{.Networks}}"

# Cleanup
echo "6. Cleaning up..."
./agentainer stop $AGENT_ID
./agentainer remove $AGENT_ID
kill $SERVER_PID

echo "Test completed!"