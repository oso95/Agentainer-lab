# Agentainer Examples

This directory contains examples demonstrating Agentainer's capabilities for orchestrating containerized agents and workflows.

## 📁 Directory Structure

```
examples/
├── gpt-agent/          # OpenAI GPT agent with conversation memory
├── gemini-agent/       # Google Gemini agent with conversation memory  
├── mapreduce-workflow/ # Complete MapReduce pattern implementation
├── workflow-demo/      # Multi-agent AI workflow demonstration
└── *.go               # Standalone Go examples
```

## 🚀 Quick Start

### Prerequisites
1. **Docker Desktop** - Must be running
2. **Agentainer Server** - Start with: `./agentainer server` (from repository root)
3. **Redis** - Start with: `docker run -d -p 6379:6379 redis:latest`
4. **API Keys** - Required for LLM agents (OpenAI or Google AI)

### Running Examples

```bash
# 1. Start Agentainer server (from repository root)
go build -o agentainer ./cmd/agentainer/
./agentainer server

# 2. Run an example
cd examples/mapreduce-workflow
./build.sh && python3 run_workflow_api.py
```

## 🤖 LLM Agent Examples

### GPT Agent (`gpt-agent/`)
**Description**: ChatGPT agent with persistent conversation memory using Redis.

**Key Features**:
- Stateful conversations across sessions
- Redis-based memory persistence
- Automatic context management
- Token usage tracking

**Setup**:
```bash
cd gpt-agent
cp .env.example .env
# Edit .env and add your OPENAI_API_KEY
./run.sh
```

### Gemini Agent (`gemini-agent/`)
**Description**: Google Gemini agent with persistent conversation memory.

**Key Features**:
- Stateful conversations across sessions
- Redis-based memory persistence
- Gemini Pro model integration
- Response streaming support

**Setup**:
```bash
cd gemini-agent
cp .env.example .env
# Edit .env and add your GOOGLE_AI_API_KEY
./run.sh
```

## 🔄 Workflow Examples

### MapReduce Workflow (`mapreduce-workflow/`) ⭐ Recommended
**Description**: Production-ready MapReduce implementation that counts words from multiple URLs in parallel.

**Architecture**:
- **List Phase**: Prepares URLs for processing
- **Map Phase**: Parallel processing of each URL
- **Reduce Phase**: Aggregates results from all mappers

**Key Features**:
- Dynamic parallel task execution
- Automatic retry with exponential backoff
- Comprehensive error handling
- Multiple export formats (JSON, CSV, Markdown)
- Performance metrics collection
- Resource limits and constraints

**Files**:
- `mapper.py` - Handles list and map phases
- `reducer.py` - Aggregates word counts
- `run_workflow_api.py` - Orchestrates the workflow
- `urls.txt` - Input URLs (one per line)

**Quick Start**:
```bash
cd mapreduce-workflow
./build.sh                    # Build Docker images
python3 run_workflow_api.py   # Run workflow
```

### Workflow Demo (`workflow-demo/`)
**Description**: Multi-agent AI workflow demonstrating integration of different LLM agents.

**Architecture**:
- **Doc Extractor**: Web scraping and content extraction
- **Gemini Agent**: Google Gemini AI for content analysis
- **GPT Agent**: OpenAI GPT for additional processing

**Key Features**:
- Multiple specialized agents
- Cross-agent communication
- Complex workflow orchestration
- AI-powered content analysis

**Setup**:
```bash
cd workflow-demo
./setup.sh                    # Build all images
python3 run_workflow.py       # Run workflow
```

## 📚 Standalone Go Examples

### Basic MapReduce Examples
- `mapreduce_simple.go` - Minimal MapReduce implementation
- `mapreduce_wordcount.go` - Word counting with MapReduce
- `mapreduce_pattern_demo.go` - Demonstrates execution patterns

**Run**: `go run mapreduce_simple.go`


## 🏗️ Architecture Concepts

### Agent Lifecycle
1. **Creation**: Agent containers are created on-demand
2. **Execution**: Tasks run in isolated environments
3. **State Management**: Redis stores agent state and results
4. **Cleanup**: Automatic cleanup based on policies

### Workflow Patterns
1. **Sequential**: Tasks run one after another
2. **Parallel**: Multiple tasks run simultaneously
3. **Map**: Dynamic parallel execution over arrays
4. **Reduce**: Aggregate results from parallel tasks

### State Management
- All state stored in Redis
- Key patterns: `workflow:{id}:state`, `task:{id}:result`
- Pub/sub for task completion notifications

## 🛠️ Development Tips

### For Coding Agents
1. **Always check prerequisites** before running examples
2. **Build Docker images** before running workflows: `./build.sh`
3. **Check logs** for debugging: `docker logs <container-id>`
4. **Monitor Redis** for state: `redis-cli HGETALL workflow:{id}:state`

### Common Issues
1. **"Cannot connect to Docker"** - Ensure Docker Desktop is running
2. **"Connection refused :8081"** - Start Agentainer server first
3. **"No such image"** - Run build script in example directory
4. **"Redis connection error"** - Start Redis container

### Best Practices
1. Use `.gitignore` to exclude result directories
2. Always cleanup resources after testing
3. Set resource limits for production use
4. Monitor agent performance metrics

## 📊 Monitoring

- **Web Dashboard**: http://localhost:8080
- **API Health**: http://localhost:8081/health
- **Metrics**: Available via Redis keys
- **Logs**: `agentainer logs <agent-id>`

## 🔗 Related Documentation

- [Agentainer Documentation](../docs/)
- [API Reference](../docs/API_ENDPOINTS.md)
- [Orchestrator Guide](../docs/ORCHESTRATOR.md)
- [Architecture Guide](../docs/WORKFLOW_ARCHITECTURE.md)