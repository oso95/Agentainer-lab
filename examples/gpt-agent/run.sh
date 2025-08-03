#!/bin/bash

# Quick start script for GPT Agent
set -e

echo "🚀 GPT Agent Quick Start"
echo "======================="
echo

# Check if .env exists
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        echo "📝 Creating .env from .env.example..."
        cp .env.example .env
        echo "⚠️  Please edit .env and add your OpenAI API key!"
        echo "   Run: nano .env"
        echo
        exit 1
    else
        echo "❌ No .env or .env.example found!"
        exit 1
    fi
fi

# Check if API key is set
if grep -q "your_openai_api_key_here" .env; then
    echo "⚠️  Please update your OpenAI API key in .env!"
    echo "   Run: nano .env"
    echo
    exit 1
fi

# Build the Docker image
echo "🔨 Building Docker image..."
docker build -t gpt-agent:latest .

# Check if agentainer is running
if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "⚠️  Agentainer doesn't seem to be running!"
    echo "   Start it with: make run"
    echo
    exit 1
fi

# Deploy the agent
echo "📦 Deploying agent..."
RESPONSE=$(curl -s -X POST http://localhost:8081/agents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer agentainer-default-token" \
  -d '{
    "name": "gpt-agent",
    "image": "gpt-agent:latest",
    "port": 8000
  }')

AGENT_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$AGENT_ID" ]; then
    echo "❌ Failed to deploy agent. Response:"
    echo $RESPONSE
    exit 1
fi

echo "✅ Agent deployed with ID: $AGENT_ID"

# Start the agent
echo "▶️  Starting agent..."
curl -s -X POST http://localhost:8081/agents/$AGENT_ID/start \
  -H "Authorization: Bearer agentainer-default-token" > /dev/null

sleep 2

# Test the agent
echo "🧪 Testing agent..."
echo

# Show endpoints
echo "📋 Available endpoints:"
echo "   http://localhost:8081/agent/$AGENT_ID/"
echo "   http://localhost:8081/agent/$AGENT_ID/health"
echo "   http://localhost:8081/agent/$AGENT_ID/chat"
echo "   http://localhost:8081/agent/$AGENT_ID/history"
echo "   http://localhost:8081/agent/$AGENT_ID/metrics"
echo

# Test chat
echo "💬 Testing chat..."
CHAT_RESPONSE=$(curl -s -X POST http://localhost:8081/agent/$AGENT_ID/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello! What can you help me with?"}')

if echo $CHAT_RESPONSE | grep -q "response"; then
    echo "✅ Chat is working!"
    echo
    echo "Response:"
    echo $CHAT_RESPONSE | python3 -m json.tool 2>/dev/null || echo $CHAT_RESPONSE
else
    echo "❌ Chat test failed. Response:"
    echo $CHAT_RESPONSE
fi

echo
echo "🎉 GPT Agent is ready to use!"
echo
echo "Try these commands:"
echo "  # Chat with the agent"
echo "  curl -X POST http://localhost:8081/agent/$AGENT_ID/chat \\"
echo "    -H \"Content-Type: application/json\" \\"
echo "    -d '{\"message\": \"Tell me a joke\"}'"
echo
echo "  # View conversation history"
echo "  curl http://localhost:8081/agent/$AGENT_ID/history"
echo
echo "  # View metrics"
echo "  curl http://localhost:8081/agent/$AGENT_ID/metrics"
echo
echo "To stop the agent:"
echo "  agentainer stop $AGENT_ID"
echo