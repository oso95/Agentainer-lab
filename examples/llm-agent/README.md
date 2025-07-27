# LLM Agent Example

A flexible LLM agent that supports multiple providers (OpenAI GPT and Google Gemini) with conversation history and state persistence.

## Features

- **Multi-Provider Support**: Switch between OpenAI and Google Gemini
- **Conversation Memory**: Maintains conversation history across restarts
- **State Persistence**: Saves conversation state to disk
- **Health Monitoring**: Built-in health check endpoint
- **Error Handling**: Graceful error handling and logging

## Quick Start

### Setup Environment Variables

Copy the example environment file and add your API keys:

```bash
cp .env.example .env
# Edit .env with your API keys
```

### Option 1: Quick deployment with environment file (Recommended)

```bash
# After setting up your .env file
./deploy-with-env.sh

# The script will:
# - Load your API keys from .env
# - Validate required variables
# - Deploy with automatic Dockerfile building
# - Configure health checks and auto-restart
```

### Option 2: Using OpenAI (GPT) manually

```bash
# Build the image
cd examples/llm-agent
docker build -t llm-agent:latest .

# Deploy with OpenAI
agentainer deploy \
  --name my-gpt-agent \
  --image llm-agent:latest \
  --env LLM_PROVIDER=openai \
  --env OPENAI_API_KEY=your-openai-api-key \
  --env OPENAI_MODEL=gpt-3.5-turbo \
  --volume ./agent-data:/app/state \
  --auto-restart

# Start the agent
agentainer start <agent-id>

# Chat with the agent
curl -X POST http://localhost:8081/agent/<agent-id>/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Hello! Tell me about yourself.",
    "system_prompt": "You are a helpful AI assistant running in a container."
  }'
```

### Option 3: Using Google Gemini manually

```bash
# Deploy with Gemini
agentainer deploy \
  --name my-gemini-agent \
  --image llm-agent:latest \
  --env LLM_PROVIDER=gemini \
  --env GEMINI_API_KEY=your-gemini-api-key \
  --env GEMINI_MODEL=gemini-pro \
  --volume ./agent-data:/app/state \
  --auto-restart

# Start and use the same way as above
```

### Option 3: Deploy from YAML

```bash
# Edit deploy-llm-agent.yaml with your API keys
agentainer deploy --config deploy-llm-agent.yaml
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENT_NAME` | Name of the agent | `llm-agent` |
| `LLM_PROVIDER` | Provider to use (`openai` or `gemini`) | `openai` |
| `OPENAI_API_KEY` | OpenAI API key (required for OpenAI) | - |
| `OPENAI_MODEL` | OpenAI model to use | `gpt-3.5-turbo` |
| `GEMINI_API_KEY` | Google Gemini API key (required for Gemini) | - |
| `GEMINI_MODEL` | Gemini model to use | `gemini-pro` |
| `PORT` | Port to run the service on | `8000` |

## API Endpoints

### Chat with the Agent
```bash
POST /chat
{
  "message": "Your message here",
  "system_prompt": "Optional system prompt"
}
```

### Get Agent Status
```bash
GET /status
```

### View Conversation History
```bash
GET /history
```

### Clear Conversation History
```bash
POST /clear
```

### Health Check
```bash
GET /health
```

## Example Usage

### Basic Chat
```bash
curl -X POST http://localhost:8081/agent/<agent-id>/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What is the capital of France?"}'
```

### Chat with Custom System Prompt
```bash
curl -X POST http://localhost:8081/agent/<agent-id>/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Write a haiku about containers",
    "system_prompt": "You are a poetic AI that responds only in haiku format."
  }'
```

### Get Conversation History
```bash
curl http://localhost:8081/agent/<agent-id>/history
```

### Monitor Agent Health
```bash
# Via Agentainer CLI
agentainer health <agent-id>

# Via API
curl http://localhost:8081/agent/<agent-id>/health
```

## State Persistence

The agent automatically saves conversation history to `/app/state/agent_state.json`. Mount a volume to persist conversations across container restarts:

```bash
--volume ./my-agent-data:/app/state
```

## Advanced Configuration

### High-Availability Setup
Deploy multiple instances with shared state:

```yaml
apiVersion: v1
kind: AgentDeployment
spec:
  agents:
    - name: llm-agent-pool
      image: llm-agent:latest
      replicas: 3
      env:
        LLM_PROVIDER: openai
        OPENAI_API_KEY: ${OPENAI_API_KEY}
      volumes:
        - host: ./shared-state
          container: /app/state
      autoRestart: true
      healthCheck:
        endpoint: /health
        interval: 30s
        retries: 3
```

## Troubleshooting

### API Key Issues
If you see "unhealthy" status:
1. Check that the API key environment variable is set correctly
2. Verify the API key is valid and has proper permissions
3. Check logs: `agentainer logs <agent-id>`

### Rate Limiting
Both OpenAI and Gemini have rate limits. The agent will return errors if limits are exceeded. Consider:
- Using multiple API keys
- Implementing request queuing
- Adding retry logic with exponential backoff

### Memory Usage
Long conversations can consume memory. The agent automatically:
- Keeps only the last 10 messages for context
- Persists only the last 50 messages to disk
- Clears old history on `/clear` endpoint