#!/bin/bash

# Test script for LLM agent
# Usage: ./test-agent.sh <agent-id>

if [ -z "$1" ]; then
    echo "Usage: $0 <agent-id>"
    echo "Example: $0 agent-123456"
    exit 1
fi

AGENT_ID=$1
BASE_URL="http://localhost:8081/agent/$AGENT_ID"

echo "Testing LLM Agent: $AGENT_ID"
echo "================================"

# Health check
echo -e "\n1. Health Check:"
curl -s "$BASE_URL/health" | jq .

# Status check
echo -e "\n2. Status Check:"
curl -s "$BASE_URL/status" | jq .

# Basic chat
echo -e "\n3. Basic Chat Test:"
curl -s -X POST "$BASE_URL/chat" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello! What is your name and what can you do?"}' | jq .

# Chat with system prompt
echo -e "\n4. Chat with System Prompt:"
curl -s -X POST "$BASE_URL/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Explain containerization",
    "system_prompt": "You are a Docker expert. Keep responses concise and technical."
  }' | jq .

# Get history
echo -e "\n5. Conversation History:"
curl -s "$BASE_URL/history" | jq .

echo -e "\nTest completed!"