#!/bin/bash
# Deploy LLM agent using environment variables from .env file

set -e

# Check if .env exists
if [ ! -f .env ]; then
    echo "Error: .env file not found!"
    echo "Please copy .env.example to .env and add your API keys"
    exit 1
fi

# Load environment variables
set -a
source .env
set +a

# Validate required variables
if [ "$LLM_PROVIDER" == "openai" ] && [ -z "$OPENAI_API_KEY" ]; then
    echo "Error: OPENAI_API_KEY is required when using OpenAI provider"
    exit 1
fi

if [ "$LLM_PROVIDER" == "gemini" ] && [ -z "$GEMINI_API_KEY" ]; then
    echo "Error: GEMINI_API_KEY is required when using Gemini provider"
    exit 1
fi

# Deploy the agent
echo "Deploying LLM agent with $LLM_PROVIDER provider..."
agentainer deploy \
  --name ${AGENT_NAME:-llm-agent} \
  --image ./Dockerfile \
  --env LLM_PROVIDER=$LLM_PROVIDER \
  --env OPENAI_API_KEY=$OPENAI_API_KEY \
  --env OPENAI_MODEL=$OPENAI_MODEL \
  --env GEMINI_API_KEY=$GEMINI_API_KEY \
  --env GEMINI_MODEL=$GEMINI_MODEL \
  --env AGENT_NAME=${AGENT_NAME:-llm-agent} \
  --env STATE_DIR=${STATE_DIR:-/app/state} \
  --volume ./state:/app/state \
  --health-endpoint /health \
  --health-interval 30s \
  --auto-restart

echo "Deployment complete! Use 'agentainer list' to see your agent."