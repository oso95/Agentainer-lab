# Gemini Agent Example

A Google Gemini agent with conversation memory using Redis state management.

## Features

- **Conversation Memory**: Remembers previous conversations
- **Redis State**: Persists conversations in Agentainer's Redis
- **Context-Aware**: Uses last 3 conversations for context
- **Metrics Tracking**: Tracks total conversations

## Quick Start

1. **Setup API Key**
   ```bash
   cp .env.example .env
   # Edit .env and add your Gemini API key
   ```

2. **Deploy**
   ```bash
   agentainer deploy --name gemini-agent --image ./Dockerfile
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
     -d '{"message": "Remember that my favorite color is blue"}'
   
   # Second chat - it remembers!
   curl -X POST http://localhost:8081/agent/<agent-id>/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "What is my favorite color?"}'
   
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
- `GEMINI_API_KEY`: Your Google AI API key (required)
- `GEMINI_MODEL`: Model to use (default: gemini-2.5-flash)

## How Memory Works

The agent uses Agentainer's provided Redis instance to store:
- Last 50 conversations in a Redis list
- Metrics in a Redis hash
- Includes last 3 conversations as context in API calls

This means conversations persist across:
- Agent restarts
- Container recreations  
- Server reboots (if Redis persists)