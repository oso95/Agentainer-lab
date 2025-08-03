<div align="center">

# 🚀 Agentainer

## **The Missing Infrastructure Layer for LLM Agents**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-required-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/oso95/Agentainer-lab/pulls)
[![GitHub Stars](https://img.shields.io/github/stars/oso95/Agentainer-lab?style=social)](https://github.com/oso95/Agentainer-lab/stargazers)

<p align="center">
  <img src="https://img.shields.io/badge/Status-Proof%20of%20Concept-orange" alt="Status">
  <img src="https://img.shields.io/badge/Platform-Linux%20|%20macOS%20|%20WSL-blue" alt="Platform">
  <img src="https://img.shields.io/badge/Architecture-Microservices-purple" alt="Architecture">
</p>

### **Deploy, manage, and scale LLM agents as containerized microservices with built-in resilience**

[**🚀 Quick Start**](#-quick-start) • [**📖 Documentation**](#-documentation) • [**💡 Examples**](#-examples) • [**🔧 CLI Reference**](#-cli-commands) • [**🔌 API**](#-api-reference) • [**🏗️ Architecture**](#-architecture)

</div>

---

## 🎯 What is Agentainer?


**Agentainer** is a container runtime specifically designed for LLM agents. Just as Docker revolutionized application deployment, Agentainer makes it dead simple to deploy, manage, and scale AI agents with production-grade reliability.


### The Problem
🔴 **Building LLM agents is easy. Running them reliably in production is hard.**
- Agents crash unexpectedly
- Lost requests during downtime
- Complex state management
- No standard deployment patterns
- Manual orchestration overhead

### The Solution
✅ **Agentainer provides the missing infrastructure layer:**
- **One command deployment**: `agentainer deploy --name my-agent --image ./Dockerfile`
- **Automatic crash recovery** with request replay
- **Built-in state persistence** via Redis
- **Network isolation** with unified proxy access
- **Production patterns** out of the box

### 🚀 NEW: Agentainer Flow - Workflow Orchestration
- **MapReduce in one line**: `agentainer workflow mapreduce --mapper scraper:latest --reducer analyzer:latest`
- **20-50x faster parallel execution** with agent pooling
- **Fan-out/fan-in orchestration** for complex workflows
- **Shared workflow state** across all steps
- **70% less code** than traditional orchestrators

---

## 🔍 How It Compares

<table>
<tr>
<th>Feature</th>
<th>Agentainer</th>
<th>Raw Docker</th>
<th>Kubernetes</th>
<th>Serverless</th>
</tr>
<tr>
<td><b>Deployment Speed</b></td>
<td>✅ < 30 seconds</td>
<td>⚠️ Manual setup</td>
<td>❌ Complex YAML</td>
<td>✅ Fast</td>
</tr>
<tr>
<td><b>State Management</b></td>
<td>✅ Built-in Redis</td>
<td>❌ DIY</td>
<td>⚠️ External</td>
<td>❌ Stateless</td>
</tr>
<tr>
<td><b>Request Persistence</b></td>
<td>✅ Automatic</td>
<td>❌ Not included</td>
<td>❌ Not included</td>
<td>❌ Lost on timeout</td>
</tr>
<tr>
<td><b>Crash Recovery</b></td>
<td>✅ With replay</td>
<td>⚠️ Restart only</td>
<td>⚠️ Restart only</td>
<td>✅ Auto-retry</td>
</tr>
<tr>
<td><b>Local Development</b></td>
<td>✅ Optimized</td>
<td>✅ Native</td>
<td>❌ Heavy</td>
<td>❌ Cloud only</td>
</tr>
<tr>
<td><b>LLM-Specific</b></td>
<td>✅ Purpose-built</td>
<td>❌ Generic</td>
<td>❌ Generic</td>
<td>❌ Generic</td>
</tr>
</table>

---

## 🏗️ Architecture

Agentainer provides a complete infrastructure layer between your agent code and container runtime.
<img width="400" height="600" alt="image" src="https://github.com/user-attachments/assets/7c8a3b72-bf6f-4663-a620-ddf5e9d8c181" />


### 🎯 Why Choose Agentainer?

<table>
<tr>
<th width="25%">🚀 Deploy in Seconds</th>
<th width="25%">💪 Never Lose Data</th>
<th width="25%">🔒 Secure by Default</th>
<th width="25%">🎯 Purpose-Built</th>
</tr>
<tr>
<td>From code to running agent with one command</td>
<td>Built-in Redis + request queuing + auto-recovery</td>
<td>Network isolation, no direct port exposure</td>
<td>Designed specifically for LLM agent workloads</td>
</tr>
</table>

---

## ⚠️ Important Notice

> **PROOF-OF-CONCEPT SOFTWARE - LOCAL TESTING ONLY**
>
> This is experimental software designed for local development and concept validation.  
> **🚨 DO NOT USE IN PRODUCTION OR EXPOSE TO EXTERNAL NETWORKS 🚨**
>
> - Demo authentication (default tokens)
> - Minimal security controls
> - Not suitable for multi-user environments
> - Requires Docker socket access

---

---

## 📸 Perfect For

<table>
<tr>
<td width="33%">

**💬 Customer Support Bots**

Stateful agents that remember conversation history and customer context across sessions.

</td>
<td width="33%">

**🔄 Data Processing Pipelines**

Multi-agent workflows with automatic retries and state checkpointing.

</td>
<td width="33%">

**🤖 Personal Assistants**

Long-running agents that handle tasks asynchronously without losing progress.

</td>
</tr>
<tr>
<td width="33%">

**📋 Research Agents**

Agents that collect data over time and need persistent storage.

</td>
<td width="33%">

**🎯 API Gateways**

Intelligent routers that adapt based on traffic patterns and errors.

</td>
<td width="33%">

**📊 Analytics Agents**

Agents that process metrics and maintain rolling aggregations.

</td>
</tr>
</table>

---

## 🚀 Quick Start

### Prerequisites
- **Docker** (required)
- **Go 1.23+** (for building from source)
- **Git** (for cloning)

> **Note for macOS users**: When deploying from Dockerfiles, build the image first using `docker build`, then deploy the built image. This avoids Docker socket compatibility issues.

### Installation (< 2 minutes)

```bash
# Clone and install
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab
make setup    # Installs everything including prerequisites

# Start Agentainer (unified approach)
make run      # Uses docker-compose internally
```

> **Note**: Agentainer now uses a unified startup approach with docker-compose. This ensures consistent Redis connectivity for both standalone agents and workflows.

### Your First Agent (< 30 seconds)

```bash
# 1. Deploy a simple agent
agentainer deploy --name hello-world --image nginx:latest

# 2. Start it
agentainer start <agent-id>

# 3. Access it (no auth needed for proxy)
curl http://localhost:8081/agent/hello-world/
```

### Deploy an LLM Agent (< 1 minute)

```bash
# 1. Use the GPT example
cd examples/gpt-agent
cp .env.example .env
# Add your OpenAI API key to .env

# 2. Deploy from Dockerfile
# For macOS users: Build the image first, then deploy
docker build -t gpt-bot-image .
agentainer deploy --name gpt-bot --image gpt-bot-image

# For Linux users: Direct Dockerfile deployment works
# agentainer deploy --name gpt-bot --image ./Dockerfile

# 3. Start and test
agentainer start <agent-id>

# 4. Chat with your agent
curl -X POST http://localhost:8081/agent/gpt-bot/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello! What is Agentainer?"}'
```

### Workflow Orchestration (NEW! 🚀)

```bash
# 1. Simple MapReduce workflow
agentainer workflow mapreduce \
  --name web-scraper \
  --mapper scraper:latest \
  --reducer analyzer:latest \
  --parallel 10

# 2. Monitor workflow progress
agentainer workflow get <workflow-id>
agentainer workflow jobs <workflow-id>

# 3. With agent pooling (20-50x faster!)
agentainer workflow mapreduce \
  --name fast-processor \
  --mapper processor:v2 \
  --reducer aggregator:v2 \
  --parallel 20 \
  --pool-size 5  # 5 agents handle 20 tasks
```

See [Agentainer Flow documentation](docs/AGENTAINER_FLOW.md) for more workflow patterns.

---

## 💡 Examples

### Example 1: Stateful Chat Agent with Memory

<details>
<summary><b>View Code</b></summary>

```python
# app.py - A GPT agent that remembers conversations
import os
import redis
from flask import Flask, request, jsonify

app = Flask(__name__)

# Connect to Agentainer's Redis
redis_client = redis.Redis(
    host='host.docker.internal', 
    port=6379
)

@app.route('/chat', methods=['POST'])
def chat():
    user_msg = request.json['message']
    
    # Get conversation history from Redis
    history = redis_client.lrange('conversations', 0, 5)
    
    # Call OpenAI with context
    response = openai_chat_with_history(user_msg, history)
    
    # Save to Redis for next time
    redis_client.lpush('conversations', f"User: {user_msg}")
    redis_client.lpush('conversations', f"AI: {response}")
    
    return jsonify({'response': response})
```

```dockerfile
# Dockerfile
FROM python:3.11-slim
WORKDIR /app
RUN pip install flask redis openai gunicorn
COPY app.py .
COPY .env .
EXPOSE 8000
CMD ["gunicorn", "-b", "0.0.0.0:8000", "app:app"]
```

```bash
# Deploy and use
agentainer deploy --name memory-bot --image ./Dockerfile
agentainer start <agent-id>

# First conversation
curl -X POST http://localhost:8081/agent/memory-bot/chat \
  -d '{"message": "My name is Alice"}'
# Response: "Nice to meet you, Alice!"

# Later conversation - it remembers!
curl -X POST http://localhost:8081/agent/memory-bot/chat \
  -d '{"message": "What is my name?"}'
# Response: "Your name is Alice."
```

</details>

### Example 2: Multi-Agent Pipeline

<details>
<summary><b>View YAML Deployment</b></summary>

```yaml
# agents.yaml - Deploy a complete LLM pipeline
apiVersion: v1
kind: AgentDeployment
metadata:
  name: llm-pipeline
spec:
  agents:
    # Agent 1: Data Collector
    - name: collector
      image: ./collector/Dockerfile
      env:
        COLLECT_INTERVAL: "60"
      volumes:
        - host: ./data
          container: /app/data
      
    # Agent 2: Processor with GPU
    - name: processor  
      image: ./processor/Dockerfile
      resources:
        memory: 4G
        cpu: 2
      env:
        MODEL: "llama2"
        
    # Agent 3: API Gateway
    - name: gateway
      image: ./gateway/Dockerfile
      healthCheck:
        endpoint: /health
        interval: 30s
      autoRestart: true
```

```bash
# Deploy entire pipeline
agentainer deploy --config agents.yaml

# All agents start with crash recovery
# and request persistence enabled
```

</details>

### Example 3: Production-Ready Agent

<details>
<summary><b>View Production Pattern</b></summary>

```python
# Resilient agent with state checkpointing
import signal
import json
import os

class ResilientAgent:
    def __init__(self):
        # Handle graceful shutdown
        signal.signal(signal.SIGTERM, self.shutdown)
        
        # Load previous state if exists
        self.checkpoint = self.load_checkpoint()
        
    def process_batch(self, items):
        for i, item in enumerate(items):
            try:
                # Process item
                result = self.process_item(item)
                
                # Save progress after each item
                self.checkpoint['last_processed'] = i
                self.checkpoint['results'].append(result)
                self.save_checkpoint()
                
            except Exception as e:
                # On error, we can resume from checkpoint
                self.handle_error(e, item)
                
    def shutdown(self, signum, frame):
        """Save state before container stops"""
        self.save_checkpoint()
        sys.exit(0)
```

```bash
# Deploy with persistent volume
agentainer deploy \
  --name resilient-processor \
  --image ./Dockerfile \
  --volume /data/checkpoints:/app/checkpoints \
  --auto-restart

# Even if it crashes, it resumes from checkpoint
# Agentainer replays any missed requests
```

</details>


---

## 📖 Documentation

### Quick Reference

| Command | Description | Example |
|---------|-------------|---------|
| `deploy` | Deploy a new agent | `agentainer deploy --name my-agent --image nginx` |
| `start` | Start an agent | `agentainer start <agent-id>` |
| `stop` | Stop an agent | `agentainer stop <agent-id>` |
| `resume` | Resume crashed agent | `agentainer resume <agent-id>` |
| `list` | List all agents | `agentainer list` |
| `logs` | View agent logs | `agentainer logs <agent-id>` |

**[📖 Full Documentation →](docs/)** including:
- [CLI Reference](docs/CLI_REFERENCE.md) - All commands and options
- [Deployment Guide](docs/DEPLOYMENT_GUIDE.md) - Advanced deployment patterns  
- [Building Resilient Agents](docs/RESILIENT_AGENTS.md) - Production patterns
- [API Endpoints](docs/API_ENDPOINTS.md) - REST API reference
- [Network Architecture](docs/NETWORK_ARCHITECTURE.md) - Networking details




### 📬 Request Persistence

When request persistence is enabled (default), Agentainer automatically:

1. **Queues requests** sent to stopped/crashed agents
2. **Replays requests** when agents become available
3. **Tracks status** of all requests (pending/completed/failed)
4. **Preserves requests** even if agents crash mid-processing

```bash
# View pending requests for an agent
agentainer requests agent-123

# Requests are automatically replayed when you start the agent
agentainer start <agent-id>
```

### 🏥 Health Checks

Agentainer monitors agent health and automatically restarts unhealthy agents:

1. **Configurable Endpoints**: Define custom health check paths
2. **Auto-Restart**: Restart agents that fail health checks
3. **Failure Tracking**: Monitor consecutive failures before restart
4. **Status Monitoring**: View health status via CLI or API

```bash
# View health status for all agents
agentainer health

# View health status for a specific agent
agentainer health agent-123

# Deploy with health checks
agentainer deploy --name my-agent --image my-app:latest \
  --health-endpoint /health \
  --health-interval 30s \
  --health-retries 3 \
  --auto-restart
```

### 📊 Resource Monitoring (Coming Soon)

Real-time resource monitoring for all agents with historical data:

1. **CPU & Memory**: Track usage and limits
2. **Network I/O**: Monitor bandwidth and packet counts
3. **Disk I/O**: Track read/write operations
4. **History**: View up to 24 hours of metrics data

```bash
# View current resource metrics
agentainer metrics agent-123

# View metrics history (last hour)
agentainer metrics agent-123 --history

# View metrics for specific duration
agentainer metrics agent-123 --history --duration 6h

# Get metrics via API
curl http://localhost:8081/agents/agent-123/metrics \
  -H "Authorization: Bearer agentainer-default-token"
```

### 💾 Backup & Restore (Coming Soon)

Complete backup solution for agent configurations and persistent data:

1. **Configuration Backup**: Save agent settings, environment, and volumes
2. **Volume Data**: Backup persistent volume data  
3. **Selective Restore**: Restore all or specific agents
4. **Export/Import**: Share backups as tar.gz files

```bash
# Create a backup of all agents
agentainer backup create --name "production-backup" --description "Weekly backup"

# Backup specific agents
agentainer backup create --name "critical-agents" --agents agent-123,agent-456

# List available backups
agentainer backup list

# Restore all agents from backup
agentainer backup restore backup-1234567890

# Restore specific agents
agentainer backup restore backup-1234567890 --agents agent-123

# Export backup for archival
agentainer backup export backup-1234567890 production-backup.tar.gz

# Delete old backup
agentainer backup delete backup-1234567890
```

### 📝 Logging & Audit Trail (Coming Soon)

Comprehensive logging system with structured logs and audit trails:

1. **Structured Logs**: JSON-formatted logs with metadata
2. **Audit Trail**: Track all administrative actions
3. **Log Rotation**: Automatic rotation and cleanup
4. **Real-time Access**: Stream logs via Redis
5. **Filtering**: Query logs by component, level, or time

```bash
# View audit logs for all actions
agentainer audit

# Filter audit logs
agentainer audit --user admin --action deploy_agent --duration 24h

# View audit logs for specific resource
agentainer audit --resource agent --duration 1h

# Export audit logs (limit results)
agentainer audit --limit 1000 > audit-export.log
```

**Audit Events Tracked:**
- Agent deployment, start, stop, restart, removal
- Configuration changes
- Authentication attempts
- API access with IP tracking
- Resource modifications

---

## 🔌 API Reference

### Two Endpoints, Two Purposes

<table>
<tr>
<td width="50%">

**🔧 API Endpoints** (`/agents/*`)
- Manage agent lifecycle
- Requires authentication
- Deploy, start, stop agents

```bash
# Deploy agent
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer token" \
  -d '{"name": "my-agent", "image": "nginx"}'
```

</td>
<td width="50%">

**🌐 Proxy Endpoints** (`/agent/*`)
- Access your agents directly
- No authentication needed
- Call your agent's APIs

```bash
# Chat with agent
curl -X POST http://localhost:8081/agent/my-agent/chat \
  -d '{"message": "Hello!"}'
```

</td>
</tr>
</table>

**Quick tip**: "agents" (plural) = API, "agent" (singular) = Proxy

**[📖 Full API Documentation →](docs/API_ENDPOINTS.md)**


---

## 🛠️ Development

### Quick Start Development

```bash
# Clone the repo
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab

# Build and run
make build
make run      # Unified startup with docker-compose

# Run tests
make test
```

> **Migration Note**: If you were using the old startup method, see the [Unified Startup Migration Guide](docs/UNIFIED_STARTUP_MIGRATION.md).

### Key Commands

```bash
make help        # Show all available commands
make setup       # Complete setup for fresh VMs
make verify      # Verify installation
make test-all    # Run all tests including integration
```

---

## 🐛 Troubleshooting

<details>
<summary><b>Common Issues</b></summary>

| Issue | Solution |
|-------|----------|
| Docker daemon not running | Ensure Docker is running: `docker ps` |
| Redis connection failed | Verify Redis: `redis-cli ping` |
| Permission denied | Add user to docker group: `sudo usermod -aG docker $USER` |
| Agent not accessible | Check proxy endpoint: `http://localhost:8081/agent/<id>/` |
| Requests not replaying | Check persistence is enabled in config.yaml |
| Installation fails | Run `make verify` to check prerequisites |
| "Image not found" error | Build the Docker image first or use a Dockerfile path |
| Agent states out of sync | Wait 10 seconds for auto-sync or restart server |

</details>

---

## 🤝 Contributing

We welcome contributions! Agentainer is in active development and we'd love your help making it better.

### How to Contribute

1. **🐛 Report Bugs**: [Open an issue](https://github.com/oso95/Agentainer-lab/issues) with reproduction steps
2. **💡 Suggest Features**: [Start a discussion](https://github.com/oso95/Agentainer-lab/discussions) about your idea
3. **📦 Submit PRs**: Fork, branch, code, test, and submit!
4. **📖 Improve Docs**: Help us make the docs clearer
5. **🧪 Share Examples**: Add your agent examples to inspire others

### Development Setup

```bash
# Fork and clone
git clone https://github.com/YOUR-USERNAME/Agentainer-lab.git
cd agentainer-lab

# Create feature branch  
git checkout -b feature/amazing-feature

# Make changes and test
make test
make test-integration

# Submit PR
git push origin feature/amazing-feature
```

---

## 👥 Community & Support

- **💬 Discord**: [Join our community](https://discord.gg/8KzmtXKAcH)
- **📧 Email**: cyw@cywang.me

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

### 🌟 Star us on GitHub if you find this project useful!

<a href="https://github.com/oso95/Agentainer-lab/stargazers">
  <img src="https://img.shields.io/github/stars/oso95/Agentainer-lab?style=social" alt="GitHub stars">
</a>

<br/>
<br/>

[**Report Bug**](https://github.com/oso95/Agentainer-lab/issues) • [**Request Feature**](https://github.com/oso95/Agentainer-lab/issues) • [**Join Discussion**](https://github.com/oso95/Agentainer-lab/discussions)

</div>
