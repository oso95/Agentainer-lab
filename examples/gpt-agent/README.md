# GPT Agent Example

A ChatGPT agent with conversation memory using Redis state management.

## Features

- **Conversation Memory**: Remembers previous conversations
- **Redis State**: Persists conversations in Agentainer's Redis
- **Context-Aware**: Uses last 3 conversations for context
- **Metrics Tracking**: Tracks total conversations

## Quick Start

1. **Setup API Key**
   ```bash
   cp .env.example .env
   # Edit .env and add your OpenAI API key
   ```

2. **Deploy**
   ```bash
   agentainer deploy --name gpt-agent --image ./Dockerfile
   ```

3. **Start**
   ```bash
   agentainer start <agent-id>
   ```

4. **Test Conversation Memory**
   ```bash
   # First chat
   curl -X POST http://localhost:8081/agent/<agent-id>/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "My name is Alice"}'
   
   # Second chat - it remembers!
   curl -X POST http://localhost:8081/agent/<agent-id>/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "What is my name?"}'
   
   # View conversation history
   curl http://localhost:8081/agent/<agent-id>/history
   
   # View metrics
   curl http://localhost:8081/agent/<agent-id>/metrics
   ```

## Endpoints

- `/` - Help and endpoint list
- `/health` - Health check
- `/chat` - Chat with memory (POST)
- `/history` - View conversation history (GET)
- `/clear` - Clear conversation history (POST)
- `/metrics` - View agent metrics (GET)

## Configuration

Edit `.env` to change:
- `OPENAI_API_KEY`: Your OpenAI API key (required)
- `OPENAI_MODEL`: Model to use (default: gpt-3.5-turbo)

## How Memory Works

The agent uses Agentainer's provided Redis instance to store:
- Last 50 conversations in a Redis list
- Metrics in a Redis hash
- Includes last 3 conversations as context in API calls

This means conversations persist across:
- Agent restarts
- Container recreations  
- Server reboots (if Redis persists)