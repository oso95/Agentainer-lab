<div align="center">

# 🚀 Agentainer Lab

### **Deploy and Manage LLM Agents as Containerized Microservices**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-required-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/oso95/Agentainer-lab/pulls)
[![GitHub Stars](https://img.shields.io/github/stars/oso95/Agentainer-lab?style=social)](https://github.com/oso95/Agentainer-lab/stargazers)

<p align="center">
  <img src="https://img.shields.io/badge/Status-Proof%20of%20Concept-orange" alt="Status">
  <img src="https://img.shields.io/badge/Platform-Linux%20|%20macOS%20|%20WSL-blue" alt="Platform">
  <img src="https://img.shields.io/badge/Architecture-Microservices-purple" alt="Architecture">
</p>

---

### **Orchestrate AI agents with the simplicity of containers and the power of microservices**

[**Getting Started**](#-quick-start) • [**Documentation**](#-documentation) • [**Examples**](#-examples) • [**CLI Reference**](#-cli-commands) • [**API**](#-api-reference)

</div>

---

## 🎯 Overview

**Agentainer Lab** is a lightweight runtime that transforms LLM-based agents into containerized microservices. Deploy, manage, and scale your AI agents with the same ease as traditional web services—no frontend required.

<div align="center">
  <img width="1272" height="1174" alt="image" src="https://github.com/user-attachments/assets/2d6f894c-7321-417b-ad8d-053b92669658" />
</div>

### ✨ Key Features

<table>
<tr>
<td width="50%">

**🔧 Developer-First Design**
- CLI-driven workflow
- RESTful API with token auth
- No frontend dependencies
- Redis-backed state management
- Automatic Dockerfile building

</td>
<td width="50%">

**🐳 Container-Native**
- Docker-based isolation
- Internal network architecture
- Persistent volume mounting
- Smart proxy routing
- Auto-sync with Docker state

</td>
</tr>
<tr>
<td width="50%">

**🔄 Lifecycle Management**
- Deploy, start, stop, pause, resume
- Universal recovery system
- Automatic restart policies
- State persistence across restarts
- Real-time state synchronization

</td>
<td width="50%">

**📊 Advanced Features**
- Request persistence & replay
- Crash resilience
- Real-time container logs
- Health check endpoints
- Image validation & auto-build

</td>
</tr>
</table>

### 🆕 What's New

- **Automatic Dockerfile Building**: Deploy directly from Dockerfiles - Agentainer builds images automatically
- **Real-time State Sync**: Agent states automatically sync with Docker containers every 10 seconds
- **Image Validation**: Prevents deployment errors by validating Docker images before creating agents
- **Network Isolation**: Agents run in isolated internal networks with no direct port exposure
- **Request Persistence**: Automatic queuing and replay of requests to unavailable agents
- **YAML Deployments**: Deploy multiple agents at once using YAML configuration files

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

## 🚀 Quick Start

### Prerequisites

- **Go** 1.21 or higher
- **Docker** (for running agents)
- **Redis** (install locally or run via Docker)
- **Git** (for cloning the repository)

> **Note**: Use `make setup` to install all prerequisites automatically on fresh VMs.

### Installation

<details>
<summary><b>Option 1: Quick Setup (Recommended for Fresh VMs)</b></summary>

```bash
# Clone the repository
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab

# Complete setup (installs prerequisites + Agentainer)
make setup

# Update your PATH
source ~/.bashrc

# Start Agentainer server (containerized for proper networking)
./scripts/start-server.sh

# Or manually:
docker network create agentainer-network
docker run -d -p 6379:6379 --name agentainer-redis redis:7-alpine
docker build -t agentainer:latest .
docker run -d --name agentainer-server \
  --network agentainer-network \
  -p 8081:8081 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENTAINER_REDIS_HOST=host.docker.internal \
  --add-host host.docker.internal:host-gateway \
  agentainer:latest
```

</details>

<details>
<summary><b>Option 2: Standard Installation</b></summary>

```bash
# Clone the repository
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab

# Install Agentainer (assumes prerequisites are installed)
make install-user

# Update your PATH
source ~/.bashrc

# Start Agentainer server (containerized for proper networking)
./scripts/start-server.sh

# Or manually:
docker network create agentainer-network
docker run -d -p 6379:6379 --name agentainer-redis redis:7-alpine
docker build -t agentainer:latest .
docker run -d --name agentainer-server \
  --network agentainer-network \
  -p 8081:8081 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENTAINER_REDIS_HOST=host.docker.internal \
  --add-host host.docker.internal:host-gateway \
  agentainer:latest
```

</details>


### Verify Installation

```bash
# Check that everything is installed correctly
make verify
```

### Your First Agent

```bash
# Deploy from a Docker image
agentainer deploy --name my-first-agent --image nginx:latest

# Deploy from a Dockerfile (auto-builds the image!)
agentainer deploy --name my-custom-agent --image ./Dockerfile

# Or deploy multiple agents from YAML
agentainer deploy --config examples/deployments/basic-agents.yaml

# Start the agent
agentainer start <agent-id>

# Access your agent through the proxy (no auth needed):
curl http://localhost:8081/agent/<agent-id>/

# Check agent status via API (auth required):
curl http://localhost:8081/agents/<agent-id> \
  -H "Authorization: Bearer agentainer-default-token"
```

---

## 🏗️ Architecture

### Network-Isolated Agent Model

Each agent runs in complete isolation with:

- **🔒 Container Isolation**: Dedicated Docker container per agent
- **🌐 Internal Network**: Agents communicate only through the proxy
- **🚫 No Direct Ports**: No external port exposure for security
- **💾 Persistent Storage**: Volume mounts for data persistence
- **🔗 Unified Access**: All access through proxy at port 8081

### Automatic State Synchronization

Agentainer automatically keeps agent states synchronized with Docker:

- **🔄 Real-time Sync**: States update every 10 seconds automatically
- **🐳 Docker Events**: Monitors Docker events for immediate updates
- **🎯 Consistency**: CLI and API always show accurate container states
- **🔧 No Manual Intervention**: Sync happens transparently in background
- **📊 Unified State Store**: Single Redis instance for all components

### Request Persistence & Replay

Agentainer ensures reliable message delivery with:

- **📬 Request Queuing**: Stores requests when agents are unavailable
- **🔄 Automatic Replay**: Replays queued requests when agents start
- **💪 Crash Resilience**: Preserves requests even during agent crashes
- **📊 Status Tracking**: Monitor pending/completed/failed requests

### Agent Lifecycle States

```mermaid
stateDiagram-v2
    [*] --> Created: Deploy
    Created --> Running: Start
    Running --> Stopped: Stop/Crash
    Running --> Paused: Pause
    Paused --> Running: Resume
    Stopped --> Running: Resume/Start
    Created --> Running: Resume
    Running --> [*]: Remove
    Stopped --> [*]: Remove
    Paused --> [*]: Remove
```

---

## 📖 Documentation

### 🛠️ CLI Commands

<details>
<summary><b>Core Commands</b></summary>

| Command | Description | Example |
|---------|-------------|---------|
| `deploy` | Deploy a new agent | `agentainer deploy --name web-agent --image nginx:latest` |
| `start` | Start a deployed agent | `agentainer start agent-123` |
| `stop` | Stop a running agent | `agentainer stop agent-123` |
| `restart` | Restart a running agent | `agentainer restart agent-123` |
| `pause` | Pause agent execution | `agentainer pause agent-123` |
| `resume` | Resume any non-running agent | `agentainer resume agent-123` |
| `remove` | Remove agent completely | `agentainer remove agent-123` |
| `list` | List all agents | `agentainer list` |
| `logs` | View agent logs | `agentainer logs agent-123 --follow` |
| `requests` | View pending requests | `agentainer requests agent-123` |
| `health` | View health status | `agentainer health` or `agentainer health agent-123` |
| `metrics` | View resource metrics | `agentainer metrics agent-123` or `agentainer metrics agent-123 --history` |
| `backup` | Backup/restore agents | `agentainer backup create --name daily` or `agentainer backup restore backup-123` |
| `audit` | View audit logs | `agentainer audit` or `agentainer audit --action deploy_agent --duration 1h` |

</details>

<details>
<summary><b>Deploy from Dockerfile</b></summary>

Agentainer can automatically build Docker images from Dockerfiles during deployment:

```bash
# Deploy directly from a Dockerfile
agentainer deploy --name my-agent --image ./Dockerfile

# Deploy from a Dockerfile in another directory
agentainer deploy --name web-app --image ./my-app/Dockerfile.production

# The system will:
# 1. Detect that you're providing a Dockerfile
# 2. Build the image automatically with progress display
# 3. Generate a unique image name (e.g., agentainer-my-agent:20241227-143052)
# 4. Deploy the agent using the built image
```

**Features:**
- **Automatic Detection**: Recognizes Dockerfile vs image name
- **Progress Display**: Shows build progress with spinner
- **Smart Naming**: Generates unique image names to prevent conflicts
- **Build Context**: Uses the Dockerfile's directory as build context
- **Error Handling**: Validates images before deployment

</details>

<details>
<summary><b>Advanced Deployment Options</b></summary>

```bash
agentainer deploy \
  --name production-agent \
  --image my-agent:v1.0 \               # Can also be a Dockerfile path!
  --volume ./data:/app/data \          # Persistent storage
  --volume ./config:/app/config:ro \   # Read-only config
  --env API_KEY=secret \               # Environment variables
  --env DEBUG=false \
  --cpu 1 \                            # CPU limit (1 core)
  --memory 512M \                      # Memory limit (512MB)
  --auto-restart \                     # Restart on failure
  --token custom-auth-token \          # Custom auth token
  --health-endpoint /health \          # Health check endpoint
  --health-interval 30s \              # Check interval
  --health-timeout 5s \                # Request timeout
  --health-retries 3                   # Retries before restart
```

> **Note**: Direct port mappings (`--port`) are deprecated for security. All agent access is through the proxy.

</details>

<details>
<summary><b>YAML Batch Deployment</b></summary>

Deploy multiple agents at once using YAML configuration files:

```yaml
# agents.yaml
apiVersion: v1
kind: AgentDeployment
metadata:
  name: my-deployment
  description: Deploy multiple agents at once
spec:
  agents:
    - name: web-agent
      image: nginx:alpine
      replicas: 2
      resources:
        memory: 256M
        cpu: 0.5
      volumes:
        - host: ./web-data
          container: /usr/share/nginx/html
      autoRestart: true
      healthCheck:
        endpoint: /
        interval: 30s
        retries: 3
    
    - name: api-agent
      image: node:18-alpine
      env:
        NODE_ENV: production
      resources:
        memory: 512M
        cpu: 1
```

Deploy with:
```bash
agentainer deploy --config agents.yaml
```

See `examples/deployments/` for more YAML examples including:
- `basic-agents.yaml` - Simple multi-agent deployment
- `llm-pipeline.yaml` - Complex LLM processing pipeline

</details>

### 🔄 Resume vs Restart

| Feature | Resume | Restart |
|---------|--------|---------|
| **Works on** | Any non-running agent | Only running agents |
| **Behavior** | Smart recovery | Clean stop + start |
| **Use case** | Primary recovery method | Refresh running agent |
| **State** | Preserves all data | Preserves all data |

### 🌐 Access Methods

Agentainer provides two different endpoints for interacting with agents:

#### 1. **Proxy Endpoint** (Access agent's service): `/agent/<id>/`
Used to access the agent's actual service endpoints (no authentication required):
```bash
# Routes requests to the agent's internal web service
curl http://localhost:8081/agent/agent-123/
curl http://localhost:8081/agent/agent-123/chat
curl http://localhost:8081/agent/agent-123/api/v1/completions
```

#### 2. **API Endpoint** (Manage agent): `/agents/<id>`
Used to get agent information and control the agent via REST API (requires authentication):
```bash
# Get agent details, status, configuration
curl http://localhost:8081/agents/agent-123 \
  -H "Authorization: Bearer agentainer-default-token"

# Control agent (start, stop, restart)
curl -X POST http://localhost:8081/agents/agent-123/start \
  -H "Authorization: Bearer agentainer-default-token"
```

**Key Differences:**
- **Proxy endpoint** (`/agent/<id>/`) - Public access, no auth required, routes to agent's service
- **API endpoint** (`/agents/<id>`) - Requires auth token, manages agent lifecycle

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
agentainer start agent-123
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

### 📊 Resource Monitoring

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

### 💾 Backup & Restore

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

### 📝 Logging & Audit Trail

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

### Understanding Proxy vs API Endpoints

Agentainer provides two distinct types of endpoints for different purposes:

#### 🔧 API Endpoints (`/agents/*`) - Management Operations
- **Purpose**: Control and manage agent lifecycle
- **Authentication**: Required (Bearer token)
- **Use when you want to**:
  - Deploy, start, stop, or remove agents
  - View agent status and configuration
  - Monitor logs and metrics
  - Manage agent resources

**Example**: Managing an agent
```bash
# Deploy a new agent
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer your-token" \
  -d '{"name": "my-agent", "image": "my-agent:latest"}'

# Check agent status
curl http://localhost:8081/agents/agent-123 \
  -H "Authorization: Bearer your-token"
```

#### 🌐 Proxy Endpoint (`/agent/*`) - Direct Agent Access
- **Purpose**: Communicate directly with your agent's application
- **Authentication**: Not required (public access)
- **Use when you want to**:
  - Call any HTTP endpoint your agent exposes
  - Send requests to your agent's application
  - Integrate your agent with external services
  - Test your agent's API endpoints

**Example**: Accessing your agent's application
```bash
# If your agent exposes a root endpoint
curl http://localhost:8081/agent/agent-123/

# If your agent has an API endpoint
curl -X POST http://localhost:8081/agent/agent-123/api/chat \
  -d '{"message": "Hello, agent!"}'

# Access any path your agent exposes
curl http://localhost:8081/agent/agent-123/health
```

#### 🤔 Which Should You Use?

| Task | Use | Example |
|------|-----|---------|
| Deploy a new agent | API | `POST /agents` |
| Stop a running agent | API | `POST /agents/{id}/stop` |
| Check if agent exists | API | `GET /agents/{id}` |
| Call your agent's REST API | Proxy | `POST /agent/{id}/api/endpoint` |
| Access your agent's endpoints | Proxy | `GET /agent/{id}/` |
| Send data to your agent | Proxy | `POST /agent/{id}/process` |

**Quick tip**: Remember "agents" (plural) = API, "agent" (singular) = Proxy

See [API Endpoints Documentation](docs/API_ENDPOINTS.md) for complete reference.

<details>
<summary><b>REST Endpoints</b></summary>

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| **Server Endpoints** | | | |
| GET | `/health` | Agentainer server health check | No |
| **API Endpoints** | | | |
| POST | `/agents` | Deploy new agent | Yes |
| GET | `/agents` | List all agents | Yes |
| GET | `/agents/{id}` | Get agent details | Yes |
| POST | `/agents/{id}/start` | Start agent | Yes |
| POST | `/agents/{id}/stop` | Stop agent | Yes |
| POST | `/agents/{id}/restart` | Restart running agent | Yes |
| POST | `/agents/{id}/pause` | Pause agent | Yes |
| POST | `/agents/{id}/resume` | Resume agent | Yes |
| DELETE | `/agents/{id}` | Remove agent | Yes |
| GET | `/agents/{id}/logs` | Get agent logs | Yes |
| GET | `/agents/{id}/metrics` | Get current agent metrics | Yes |
| GET | `/agents/{id}/metrics/history` | Get agent metrics history | Yes |
| GET | `/agents/{id}/requests` | Get pending requests | Yes |
| GET | `/agents/{id}/requests/{reqId}` | Get specific request | Yes |
| POST | `/agents/{id}/requests/{reqId}/replay` | Manually replay request | Yes |
| GET | `/agents/{id}/health` | Get agent health monitoring status | Yes |
| GET | `/health/agents` | Get all agents health status | Yes |
| **Proxy Endpoints** | | | |
| ANY | `/agent/{id}/*` | Direct proxy to any agent endpoint | No |
| GET | `/agent/{id}/health` | Example: Access agent's health endpoint | No |
| POST | `/agent/{id}/api/*` | Example: Access agent's API endpoints | No |

</details>

<details>
<summary><b>Example API Usage</b></summary>

```bash
# Deploy an agent
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer agentainer-default-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-agent",
    "image": "my-api:latest",
    "volumes": [{"host_path": "./data", "container_path": "/data"}],
    "env_vars": {"NODE_ENV": "production"}
  }'

# Start the agent
curl -X POST http://localhost:8081/agents/{id}/start \
  -H "Authorization: Bearer agentainer-default-token"

# Stream logs
curl http://localhost:8081/agents/{id}/logs?follow=true \
  -H "Authorization: Bearer agentainer-default-token"

# Check pending requests
curl http://localhost:8081/agents/{id}/requests \
  -H "Authorization: Bearer agentainer-default-token"
```

</details>

---

## 🎯 Examples

### Deploy from Dockerfile

```bash
# Example 1: Deploy from a simple Dockerfile
cat > Dockerfile.hello <<EOF
FROM python:3.11-slim
RUN echo 'print("Hello from Agentainer!")' > app.py
CMD ["python", "app.py"]
EOF

agentainer deploy --name hello-world --image Dockerfile.hello
agentainer start <agent-id>

# Example 2: Deploy from existing project Dockerfile
agentainer deploy \
  --name my-api \
  --image ./my-project/Dockerfile \
  --env NODE_ENV=production \
  --volume ./config:/app/config:ro

# Example 3: Deploy LLM agent from example
agentainer deploy \
  --name llm-agent \
  --image examples/llm-agent/Dockerfile \
  --env OPENAI_API_KEY=$OPENAI_API_KEY \
  --volume ./llm-data:/app/state
```

### Real-World Example: API vs Proxy Usage

Here's a practical example deploying and using a chatbot agent:

```bash
# 1. Deploy the chatbot agent using the API
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer agentainer-default-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "chatbot",
    "image": "examples/llm-agent/Dockerfile",
    "env_vars": {"OPENAI_API_KEY": "your-key"}
  }'

# 2. Start the agent using the API
curl -X POST http://localhost:8081/agents/agent-123/start \
  -H "Authorization: Bearer agentainer-default-token"

# 3. Check agent status using the API
curl http://localhost:8081/agents/agent-123 \
  -H "Authorization: Bearer agentainer-default-token"
# Returns: {"name": "chatbot", "status": "running", ...}

# 4. Now interact with your chatbot using the PROXY (no auth!)
curl -X POST http://localhost:8081/agent/agent-123/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, how are you?"}'
# Returns: {"response": "I'm doing well, thank you! How can I help you today?"}

# 5. Access the agent's health endpoint using the PROXY
curl http://localhost:8081/agent/agent-123/health

# 6. Monitor agent logs using the API
curl http://localhost:8081/agents/agent-123/logs \
  -H "Authorization: Bearer agentainer-default-token"

# 7. When done, stop the agent using the API
curl -X POST http://localhost:8081/agents/agent-123/stop \
  -H "Authorization: Bearer agentainer-default-token"
```

**Key Takeaway**: 
- Use **API** (`/agents/*`) for management tasks (requires auth)
- Use **Proxy** (`/agent/*`) to interact with your agent's actual application (no auth)

### Building Resilient Agents

Agentainer provides infrastructure-level resilience features:
- **Automatic Request Replay**: Failed requests are queued and replayed when agents recover
- **Crash Recovery**: Agents can be restarted with `agentainer resume` after any failure
- **Persistent Volumes**: Mount volumes to preserve agent data across restarts

However, your agent code needs to handle its own application-level state. Here are proven patterns for implementing resilient agent logic:

#### Why These Patterns?

While Agentainer handles:
- ✅ Restarting crashed containers
- ✅ Replaying failed HTTP requests
- ✅ Preserving volume data

Your agent code should handle:
- ❌ Saving processing state between requests
- ❌ Resuming interrupted batch operations
- ❌ Graceful shutdown on SIGTERM
- ❌ Checkpoint/restore for long-running tasks

#### State Persistence Pattern

**Use this when:** Your agent processes data in batches or maintains session state

```python
# StatefulAgent: Maintains state across restarts
import json
import os
from datetime import datetime

class StatefulAgent:
    def __init__(self):
        self.state_dir = '/app/data'
        self.state_file = os.path.join(self.state_dir, 'state.json')
        self.load_state()

    def load_state(self):
        """Load state from persistent storage"""
        if os.path.exists(self.state_file):
            with open(self.state_file, 'r') as f:
                self.state = json.load(f)
        else:
            self.state = {
                "processed_count": 0,
                "last_run": None,
                "config": {},
                "history": []
            }

    def save_state(self):
        """Save state to persistent storage"""
        os.makedirs(self.state_dir, exist_ok=True)
        with open(self.state_file, 'w') as f:
            json.dump(self.state, f, indent=2)

    def process(self, data):
        # Update state
        self.state["processed_count"] += 1
        self.state["last_run"] = datetime.now().isoformat()
        self.state["history"].append({
            "timestamp": datetime.now().isoformat(),
            "data": data
        })

        # Save immediately for persistence
        self.save_state()
```

#### Auto-Recovery Pattern

**Use this when:** Your agent performs long-running operations that shouldn't restart from scratch

```python
# SelfHealingAgent: Recovers from interruptions gracefully
import signal
import sys
import json
import os

class SelfHealingAgent:
    def __init__(self):
        # Set up signal handlers for graceful shutdown
        signal.signal(signal.SIGTERM, self.handle_shutdown)
        signal.signal(signal.SIGINT, self.handle_shutdown)

        # Initialize state
        self.last_processed = None
        self.pending_tasks = []
        self.current_state = {}

        # Load previous state
        self.load_checkpoint()

    def handle_shutdown(self, signum, frame):
        """Save state before shutdown"""
        print(f"Received signal {signum}, saving checkpoint...")
        self.save_checkpoint()
        sys.exit(0)

    def save_checkpoint(self):
        """Save current progress"""
        checkpoint = {
            "last_processed": self.last_processed,
            "queue": self.pending_tasks,
            "state": self.current_state
        }
        os.makedirs('/app/data', exist_ok=True)
        with open('/app/data/checkpoint.json', 'w') as f:
            json.dump(checkpoint, f)

    def load_checkpoint(self):
        """Resume from last checkpoint"""
        if os.path.exists('/app/data/checkpoint.json'):
            with open('/app/data/checkpoint.json', 'r') as f:
                checkpoint = json.load(f)
                self.last_processed = checkpoint.get('last_processed')
                self.pending_tasks = checkpoint.get('queue', [])
                self.current_state = checkpoint.get('state', {})
                print(f"Resumed from checkpoint: {len(self.pending_tasks)} tasks pending")
```

#### Deployment with Persistence

```bash
# Deploy an agent with persistent volume for state
agentainer deploy \
  --name resilient-agent \
  --image my-agent:latest \
  --volume /host/agent-data:/app/data \
  --env AGENT_ID=agent-001 \
  --auto-restart

# The agent will:
# - Save state to /app/data (persisted on host)
# - Automatically restart on failure
# - Resume from last checkpoint on startup
# - Have requests queued if it crashes
```

#### Summary: Division of Responsibilities

| Feature | Agentainer Provides | Your Agent Code Handles |
|---------|-------------------|------------------------|
| Container restart | ✅ Auto-restart on crash | Save state before crash |
| HTTP requests | ✅ Queue & replay failed requests | Process requests idempotently |
| Storage | ✅ Persistent volume mounts | Read/write state files |
| Networking | ✅ Proxy & internal network | Handle connection errors |
| Lifecycle | ✅ Start/stop/pause/resume | Graceful shutdown logic |

### Multi-Agent Deployment

```bash
# Deploy a pipeline of agents
agentainer deploy --name data-collector --image collector:latest
agentainer deploy --name data-processor --image processor:latest  
agentainer deploy --name data-storage --image storage:latest

# Start all agents
agentainer start data-collector
agentainer start data-processor
agentainer start data-storage

# All agents are isolated and communicate through the proxy
```

---

## 🛠️ Development

### Project Structure

```
agentainer-lab/
├── cmd/agentainer/      # CLI entry point
├── internal/            # Private packages
│   ├── agent/          # Agent lifecycle
│   ├── api/            # REST API server
│   ├── config/         # Configuration
│   ├── requests/       # Request persistence
│   └── storage/        # Redis storage
├── pkg/                # Public packages
│   ├── docker/         # Docker client
│   └── metrics/        # Metrics collection
├── scripts/            # Helper scripts
│   ├── tests/          # Test scripts
│   └── deprecated/     # Legacy scripts
├── examples/           # Example agents
│   ├── simple-agent/   # Basic Flask agent
│   ├── llm-agent/      # Multi-provider LLM agent
│   └── deployments/    # YAML deployment examples
└── docs/              # Documentation
```

### Building from Source

```bash
# Development build
make build

# Production build
make build-prod

# Run tests
make test

# Run integration tests
make test-all
```

### Deployment Options

#### Option 1: Containerized Server (Recommended for Development)
Run Agentainer server as a container to ensure proper network connectivity:

```bash
# Use the provided script
./scripts/start-server.sh

# Or manually:
docker network create agentainer-network
docker run -d -p 6379:6379 --name agentainer-redis redis:7-alpine
docker build -t agentainer:latest .
docker run -d --name agentainer-server \
  --network agentainer-network \
  -p 8081:8081 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENTAINER_REDIS_HOST=host.docker.internal \
  --add-host host.docker.internal:host-gateway \
  agentainer:latest
```

This setup:
- Allows server to reach agent containers by hostname
- CLI commands work normally
- Agents can still communicate with each other

#### Option 2: Docker Compose
For a fully containerized setup:

```bash
docker-compose up -d
```

This will start both Redis (on port 6379) and Agentainer server (on port 8081). You can use both CLI commands and the API!

### Available Make Commands

```bash
make help              # Show all available commands
make setup            # Complete setup for fresh VMs
make verify           # Verify installation
make test-network     # Test network isolation
make test-persistence # Test request persistence
make test-crash       # Test crash resilience
```

---

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

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

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

### 🌟 Star us on GitHub if you find this project useful!

[**Report Bug**](https://github.com/oso95/Agentainer-lab/issues) • [**Request Feature**](https://github.com/oso95/Agentainer-lab/issues) • [**Join Discussion**](https://github.com/oso95/Agentainer-lab/discussions)

</div>