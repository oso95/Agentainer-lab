# Agentainer Examples

This directory contains various examples demonstrating Agentainer's capabilities.

## 🤖 LLM Agent Examples

### GPT Agent (`gpt-agent/`)
A ChatGPT agent with conversation memory using Redis state management.

**Quick Start:**
```bash
cd gpt-agent
./run.sh
```

**Features:**
- Conversation memory (remembers previous chats)
- Redis state persistence
- Context-aware responses
- Metrics tracking

### Gemini Agent (`gemini-agent/`)
A Google Gemini agent with conversation memory using Redis state management.

**Quick Start:**
```bash
cd gemini-agent
./run.sh
```

**Features:**
- Conversation memory (remembers previous chats)
- Redis state persistence
- Context-aware responses
- Metrics tracking

## 🔄 Workflow Examples

### MapReduce Examples
- `mapreduce_simple.go` - Basic MapReduce workflow implementation
- `mapreduce_wordcount.go` - Word counting using MapReduce pattern
- `mapreduce_pattern_demo.go` - Demonstrates MapReduce execution pattern
- `mapreduce-workflow/` - Complete MapReduce with custom Docker images

### SDK Examples (Python)
- `sdk_data_pipeline.py` - Data processing pipeline using Agentainer Flow SDK
- `sdk_mapreduce_example.py` - MapReduce pattern using Python SDK
- `sdk_monitoring_dashboard.py` - Monitoring dashboard workflow

## 🚀 Getting Started

1. **Start Agentainer Server**
   ```bash
   make run
   ```

2. **Choose an Example**
   - For LLM agents: Use the `run.sh` scripts in agent folders
   - For workflows: Run the Go examples with `go run <example>.go`
   - For SDK examples: Run with `python3 <example>.py`

## 📋 Prerequisites

- Docker Desktop running
- Redis running (via `docker-compose up -d`)
- API keys for LLM agents (OpenAI or Google AI)
- Go 1.21+ for Go examples
- Python 3.11+ with agentainer-flow SDK for Python examples

## 💡 Tips

- All agent examples include `.env.example` files - copy to `.env` and add your API keys
- Use `agentainer logs <agent-id>` to debug issues
- Check http://localhost:8080 for the web dashboard
- Agent state persists in Redis across restarts