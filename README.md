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

</td>
<td width="50%">

**🐳 Container-Native**
- Docker-based isolation
- Internal network architecture
- Persistent volume mounting
- Smart proxy routing

</td>
</tr>
<tr>
<td width="50%">

**🔄 Lifecycle Management**
- Deploy, start, stop, pause, resume
- Universal recovery system
- Automatic restart policies
- State persistence across restarts

</td>
<td width="50%">

**📊 Advanced Features**
- Request persistence & replay
- Crash resilience
- Real-time container logs
- Health check endpoints

</td>
</tr>
</table>

### 🆕 What's New

- **Network Isolation**: Agents now run in isolated internal networks with no direct port exposure
- **Request Persistence**: Automatic queuing and replay of requests to unavailable agents
- **Crash Resilience**: Requests are preserved even if agents crash mid-processing
- **Simplified Installation**: All operations now through unified `make` commands

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
- **Docker** and Docker Compose
- **Redis** (or use Docker Compose)
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

# Start services
docker-compose up -d redis
agentainer server
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

# Start services
docker-compose up -d redis
agentainer server
```

</details>

<details>
<summary><b>Option 3: Docker Compose</b></summary>

```bash
# Clone the repository
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab

# Start everything with Docker Compose
docker-compose up -d
```

</details>

### Verify Installation

```bash
# Check that everything is installed correctly
make verify
```

### Your First Agent

```bash
# Deploy an agent
agentainer deploy --name my-first-agent --image nginx:latest

# Start the agent
agentainer start <agent-id>

# Access your agent through the proxy:
curl http://localhost:8081/agent/<agent-id>/

# Check agent status via API:
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

</details>

<details>
<summary><b>Advanced Deployment</b></summary>

```bash
agentainer deploy \
  --name production-agent \
  --image my-agent:v1.0 \
  --volume ./data:/app/data \          # Persistent storage
  --volume ./config:/app/config:ro \   # Read-only config
  --env API_KEY=secret \               # Environment variables
  --env DEBUG=false \
  --cpu 1000000000 \                   # CPU limit (1 core)
  --memory 536870912 \                 # Memory limit (512MB)
  --auto-restart \                     # Restart on failure
  --token custom-auth-token            # Custom auth token
```

> **Note**: Direct port mappings (`--port`) are deprecated for security. All agent access is through the proxy.

</details>

### 🔄 Resume vs Restart

| Feature | Resume | Restart |
|---------|--------|---------|
| **Works on** | Any non-running agent | Only running agents |
| **Behavior** | Smart recovery | Clean stop + start |
| **Use case** | Primary recovery method | Refresh running agent |
| **State** | Preserves all data | Preserves all data |

### 🌐 Access Methods

All agents are accessed through the secure proxy:

**Proxy Access**: `http://localhost:8081/agent/<id>/`
```bash
# Routes to agent's internal endpoints
curl http://localhost:8081/agent/agent-123/
curl http://localhost:8081/agent/agent-123/api/status
```

**API Access**: `http://localhost:8081/agents/<id>`
```bash
# Get agent information via REST API
curl http://localhost:8081/agents/agent-123 \
  -H "Authorization: Bearer agentainer-default-token"
```

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

---

## 🔌 API Reference

<details>
<summary><b>REST Endpoints</b></summary>

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/agents` | Deploy new agent |
| GET | `/agents` | List all agents |
| GET | `/agents/{id}` | Get agent details |
| POST | `/agents/{id}/start` | Start agent |
| POST | `/agents/{id}/stop` | Stop agent |
| POST | `/agents/{id}/restart` | Restart running agent |
| POST | `/agents/{id}/pause` | Pause agent |
| POST | `/agents/{id}/resume` | Resume agent |
| DELETE | `/agents/{id}` | Remove agent |
| GET | `/agents/{id}/logs` | Get agent logs |
| GET | `/agents/{id}/metrics` | Get agent metrics |
| GET | `/agents/{id}/requests` | Get pending requests |
| GET | `/agents/{id}/requests/{reqId}` | Get specific request |
| POST | `/agents/{id}/requests/{reqId}/replay` | Manually replay request |
| ANY | `/agent/{id}/*` | Proxy to agent |

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

### Building Resilient Agents

Agentainer supports building production-ready agents with automatic recovery and state persistence. Here are proven patterns for implementing resilient agent logic:

#### State Persistence Pattern

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

</details>

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

### 🌟 Star us on GitHub if you find this project useful!

[**Report Bug**](https://github.com/oso95/Agentainer-lab/issues) • [**Request Feature**](https://github.com/oso95/Agentainer-lab/issues) • [**Join Discussion**](https://github.com/oso95/Agentainer-lab/discussions)

</div>