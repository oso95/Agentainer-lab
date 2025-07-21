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
  <img width="800" alt="Agentainer Architecture" src="https://via.placeholder.com/800x400/1a1a2e/16213e?text=Agentainer+Architecture+Diagram">
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
- Auto-port assignment (9000-9999)
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

**📊 Observability**
- Real-time container logs
- Basic metrics collection
- Health check endpoints
- Agent state monitoring

</td>
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

## 🚀 Quick Start

### Prerequisites

- **Go** 1.21 or higher
- **Docker** and Docker Compose
- **Redis** (or use Docker Compose)
- **Git** (for cloning the repository)

> **Note**: The setup.sh script can install all prerequisites automatically on fresh Ubuntu/Debian VMs.

### Installation

<details>
<summary><b>Option 1: Quick Install (Recommended)</b></summary>

```bash
# For fresh VMs - installs all prerequisites (Git, Go, Docker, Docker Compose)
curl -fsSL https://raw.githubusercontent.com/oso95/Agentainer-lab/main/setup.sh | bash

# Or if you already have prerequisites:
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab
./install.sh

# Update your PATH
source ~/.bashrc

# Start services
docker-compose up -d redis
agentainer server
```

</details>

<details>
<summary><b>Option 2: Using Make</b></summary>

```bash
# Clone and install
git clone https://github.com/oso95/Agentainer-lab.git
cd agentainer-lab
make quick-install

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

### Your First Agent

```bash
# Deploy an agent with auto-assigned port
agentainer deploy --name my-first-agent --image nginx:latest

# Start the agent
agentainer start <agent-id>

# Access your agent
# Direct: http://localhost:9001
# Proxy:  http://localhost:8081/agent/<agent-id>/
```

---

## 🏗️ Architecture

### Agent-as-a-Service Model

Each agent runs as an isolated microservice with:

- **🔒 Container Isolation**: Dedicated Docker container per agent
- **💾 Persistent Identity**: Survives restarts and updates  
- **📁 Stateful Operation**: Volume mounts for data persistence
- **🌐 Network Isolation**: Unique port in 9000-9999 range
- **🔗 Unified Access**: Consistent proxy routing pattern

### Agent Lifecycle States

```mermaid
stateDiagram-v2
    [*] --> Created: Deploy
    Created --> Running: Start
    Running --> Stopped: Stop/Crash
    Running --> Paused: Pause
    Paused --> Running: Resume
    Stopped --> Running: Resume
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

</details>

<details>
<summary><b>Advanced Deployment</b></summary>

```bash
agentainer deploy \
  --name production-agent \
  --image my-agent:v1.0 \
  --port 8080:8000/tcp \              # Custom port mapping
  --volume ./data:/app/data \          # Persistent storage
  --volume ./config:/app/config:ro \   # Read-only config
  --env API_KEY=secret \               # Environment variables
  --env DEBUG=false \
  --cpu 1000000000 \                   # CPU limit (1 core)
  --memory 536870912 \                 # Memory limit (512MB)
  --auto-restart \                     # Restart on failure
  --token custom-auth-token            # Custom auth token
```

</details>

### 🔄 Resume vs Restart

| Feature | Resume | Restart |
|---------|--------|---------|
| **Works on** | Any non-running agent | Only running agents |
| **Behavior** | Smart recovery | Clean stop + start |
| **Use case** | Primary recovery method | Refresh running agent |
| **State** | Preserves all data | Preserves all data |

### 🌐 Access Methods

Agents can be accessed two ways:

1. **Direct Access**: `http://localhost:<port>`
   ```bash
   # Auto-assigned port shown after deployment
   curl http://localhost:9001
   ```

2. **Proxy Access** (Recommended): `http://localhost:8081/agent/<id>/`
   ```bash
   # Consistent URL pattern
   curl http://localhost:8081/agent/agent-123/api/status
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
    "ports": [{"host_port": 8080, "container_port": 80}],
    "volumes": [{"host_path": "./data", "container_path": "/data"}],
    "env_vars": {"NODE_ENV": "production"}
  }'

# Start the agent
curl -X POST http://localhost:8081/agents/{id}/start \
  -H "Authorization: Bearer agentainer-default-token"

# Stream logs
curl http://localhost:8081/agents/{id}/logs?follow=true \
  -H "Authorization: Bearer agentainer-default-token"
```

</details>

---

## 🎯 Examples

### State Persistence Pattern

```python
# In your agent code
import json
import os

class StatefulAgent:
    def __init__(self):
        self.state_file = '/app/data/state.json'
        self.load_state()
    
    def load_state(self):
        if os.path.exists(self.state_file):
            with open(self.state_file, 'r') as f:
                self.state = json.load(f)
        else:
            self.state = {"tasks": [], "config": {}}
    
    def save_state(self):
        os.makedirs(os.path.dirname(self.state_file), exist_ok=True)
        with open(self.state_file, 'w') as f:
            json.dump(self.state, f)
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
│   └── storage/        # Redis storage
├── pkg/                # Public packages
│   ├── docker/         # Docker client
│   └── metrics/        # Metrics collection
├── examples/           # Example agents
└── data/              # Runtime storage
```

### Building from Source

```bash
# Development build
go build -o agentainer ./cmd/agentainer

# Production build
go build -ldflags="-w -s" -o agentainer ./cmd/agentainer

# Run tests
make test
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
| Port conflicts | Check ports: `netstat -tulpn \| grep :8081` |
| Agent deployment fails | Check image exists: `docker images` |

</details>

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

### 🌟 Star us on GitHub if you find this project useful!

[**Report Bug**](https://github.com/oso95/Agentainer-lab/issues) • [**Request Feature**](https://github.com/oso95/Agentainer-lab/issues) • [**Join Discussion**](https://github.com/oso95/Agentainer-lab/discussions)

</div>