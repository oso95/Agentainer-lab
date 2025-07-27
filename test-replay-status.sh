#!/bin/bash
# Test script to debug ReplayWorker status check

echo "=== Testing ReplayWorker Status Check ==="
echo

# Get agent data from Redis
AGENT_ID="agent-1753657574541799931"
echo "1. Getting agent data from Redis:"
AGENT_DATA=$(redis-cli get "agent:$AGENT_ID")
echo "$AGENT_DATA" | head -c 200
echo "..."
echo

# Check if it contains "status":"running"
echo "2. Checking for 'status\":\"running' pattern:"
if echo "$AGENT_DATA" | grep -q '"status":"running"'; then
    echo "✓ Pattern found - agent should be considered running"
else
    echo "✗ Pattern NOT found"
fi
echo

# Extract actual status
echo "3. Extracting actual status:"
STATUS=$(echo "$AGENT_DATA" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: '$STATUS'"
echo

# Check pending requests
echo "4. Checking pending requests:"
PENDING_COUNT=$(redis-cli scard "agent:$AGENT_ID:requests:pending")
echo "Pending requests: $PENDING_COUNT"

echo
echo "5. List of pending request IDs:"
redis-cli smembers "agent:$AGENT_ID:requests:pending"