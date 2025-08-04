# Agentainer Documentation

Welcome to the Agentainer documentation! This directory contains detailed guides and references for using Agentainer effectively.

## 📚 Documentation Index

### Getting Started
- **[Deployment Guide](./DEPLOYMENT_GUIDE.md)** - Learn how to deploy agents using various methods
- **[Building Resilient Agents](./RESILIENT_AGENTS.md)** - Patterns for building production-ready agents

### Core Components
- **[Agentainer Flow](./AGENTAINER_FLOW.md)** - Workflow orchestration overview
- **[Orchestrator](./ORCHESTRATOR.md)** - Detailed orchestration engine documentation
- **[Workflow Architecture](./WORKFLOW_ARCHITECTURE.md)** - How workflows manage agent containers

### Reference
- **[CLI Reference](./CLI_REFERENCE.md)** - Complete command-line interface documentation
- **[API Endpoints](./API_ENDPOINTS.md)** - REST API reference for programmatic access
- **[Network Architecture](./NETWORK_ARCHITECTURE.md)** - Understanding Agentainer's networking model

### Advanced Topics
- **[Configuration Guide](./CONFIGURATION.md)** - Detailed configuration options (coming soon)
- **[Security Guide](./SECURITY.md)** - Best practices for securing your agents (coming soon)
- **[Performance Tuning](./PERFORMANCE.md)** - Optimization tips and tricks (coming soon)

## 🔍 Quick Links

### Common Tasks

**Deploy an agent:**
```bash
agentainer deploy --name my-agent --image nginx:latest
```

**Deploy from Dockerfile:**
```bash
agentainer deploy --name my-app --image ./Dockerfile
```

**View logs:**
```bash
agentainer logs my-agent --follow
```

**Check health:**
```bash
agentainer health my-agent
```

### API vs Proxy

Remember the key difference:
- **API** (`/agents/*`) - For managing agents (requires auth)
- **Proxy** (`/agent/*`) - For accessing agents (no auth)

### Need Help?

1. Check the relevant guide in this documentation
2. Look at the [examples](../examples/) directory
3. Open an [issue](https://github.com/oso95/Agentainer-lab/issues)
4. Join our [discussions](https://github.com/oso95/Agentainer-lab/discussions)

## 📝 Contributing to Docs

We welcome documentation improvements! If you find something unclear or missing:

1. Fork the repository
2. Make your changes in the `docs/` directory
3. Submit a pull request

Documentation should be:
- Clear and concise
- Include practical examples
- Follow the existing format
- Be technically accurate