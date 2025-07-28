# Deployment Guide

This guide covers various ways to deploy agents with Agentainer, from simple single-agent deployments to complex multi-agent systems.

## Table of Contents

- [Quick Start](#quick-start)
- [Deployment Methods](#deployment-methods)
- [Advanced Options](#advanced-options)
- [YAML Deployments](#yaml-deployments)
- [Network Configuration](#network-configuration)
- [Resource Management](#resource-management)
- [Health Checks](#health-checks)
- [Environment Variables](#environment-variables)
- [Volume Mounts](#volume-mounts)
- [Best Practices](#best-practices)

## Quick Start

### Deploy from Docker Image

```bash
# Basic deployment
agentainer deploy --name my-agent --image nginx:latest

# With environment variables
agentainer deploy --name api-agent --image my-api:v1.0 \
  --env API_KEY=secret \
  --env NODE_ENV=production

# With volume mount
agentainer deploy --name data-agent --image processor:latest \
  --volume /host/data:/container/data
```

### Deploy from Dockerfile

Agentainer can build images automatically from Dockerfiles:

```bash
# Deploy from Dockerfile in current directory
agentainer deploy --name my-agent --image ./Dockerfile

# Deploy from Dockerfile in another directory
agentainer deploy --name web-app --image ./my-app/Dockerfile.production

# With build context
agentainer deploy --name service --image ./services/api/Dockerfile
```

## Deployment Methods

### 1. CLI Deployment

The most straightforward method for single agents:

```bash
agentainer deploy \
  --name production-agent \
  --image my-agent:v1.0 \
  --volume ./data:/app/data \
  --volume ./config:/app/config:ro \
  --env API_KEY=secret \
  --env DEBUG=false \
  --cpu 1 \
  --memory 512M \
  --auto-restart \
  --token custom-auth-token \
  --health-endpoint /health \
  --health-interval 30s \
  --health-timeout 5s \
  --health-retries 3
```

### 2. YAML Deployment

For deploying multiple agents or complex configurations:

```yaml
# deployment.yaml
apiVersion: v1
kind: AgentDeployment
metadata:
  name: my-deployment
  description: Production deployment
spec:
  agents:
    - name: frontend-agent
      image: frontend:v2.0
      replicas: 2
      env:
        NODE_ENV: production
        API_URL: http://localhost:8081/agent/backend-agent
      resources:
        memory: 256M
        cpu: 0.5
      healthCheck:
        endpoint: /health
        interval: 30s
        
    - name: backend-agent
      image: ./backend/Dockerfile
      env:
        DATABASE_URL: postgres://db:5432/myapp
        REDIS_URL: redis://host.docker.internal:6379
      volumes:
        - host: ./data
          container: /app/data
          mode: rw
      resources:
        memory: 1G
        cpu: 2
      autoRestart: true
      
    - name: worker-agent
      image: worker:latest
      replicas: 3
      env:
        QUEUE_URL: redis://host.docker.internal:6379
        WORKER_TYPE: processor
      resources:
        memory: 512M
        cpu: 1
```

Deploy with:
```bash
agentainer deploy --config deployment.yaml
```

### 3. Programmatic Deployment

Using the REST API:

```bash
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-agent",
    "image": "my-api:latest",
    "env_vars": {
      "NODE_ENV": "production",
      "PORT": "8000"
    },
    "volumes": [
      {
        "host_path": "./data",
        "container_path": "/app/data",
        "mode": "rw"
      }
    ],
    "resources": {
      "memory": "512M",
      "cpu": 1
    },
    "health_check": {
      "endpoint": "/health",
      "interval": "30s",
      "retries": 3
    },
    "auto_restart": true
  }'
```

## Advanced Options

### Resource Limits

Control CPU and memory usage:

```bash
# CPU limits (number of cores)
--cpu 0.5       # Half a core
--cpu 2         # Two cores

# Memory limits
--memory 256M   # 256 megabytes
--memory 1G     # 1 gigabyte
--memory 2048M  # 2048 megabytes
```

### Auto-Restart Policies

```bash
# Always restart on failure
--auto-restart

# Restart with max retry count
--auto-restart --restart-max-retries 5

# Restart with delay
--auto-restart --restart-delay 10s
```

### Health Checks

Configure health monitoring:

```bash
agentainer deploy --name monitored-agent \
  --image my-app:latest \
  --health-endpoint /health \      # HTTP endpoint to check
  --health-interval 30s \          # Check every 30 seconds
  --health-timeout 5s \            # Timeout for each check
  --health-retries 3 \             # Failures before unhealthy
  --health-start-period 60s        # Grace period on startup
```

### Custom Authentication

Deploy with custom tokens:

```bash
# Set custom token for this agent
agentainer deploy --name secure-agent \
  --image my-agent:latest \
  --token my-secret-token-123

# Access requires the custom token
curl http://localhost:8081/agents/secure-agent \
  -H "Authorization: Bearer my-secret-token-123"
```

## Network Configuration

### Internal Communication

Agents can communicate with each other using agent IDs:

```yaml
agents:
  - name: api-gateway
    image: gateway:latest
    env:
      # Reference other agents by ID
      USER_SERVICE: http://user-service:8000
      ORDER_SERVICE: http://order-service:8000
      
  - name: user-service
    image: services/user:latest
    
  - name: order-service
    image: services/order:latest
```

### External Services

Access host services using `host.docker.internal`:

```yaml
env:
  # Access Redis on host
  REDIS_URL: redis://host.docker.internal:6379
  # Access PostgreSQL on host
  DATABASE_URL: postgres://host.docker.internal:5432/db
  # Access another service on host
  API_URL: http://host.docker.internal:3000
```

## Environment Variables

### From Command Line

```bash
# Single variable
--env KEY=value

# Multiple variables
--env DATABASE_URL=postgres://localhost/db \
--env REDIS_URL=redis://localhost:6379 \
--env LOG_LEVEL=debug
```

### From .env File

```bash
# Load from .env file (must be copied in Dockerfile)
# Dockerfile:
# COPY .env .

# Then in your app:
from dotenv import load_dotenv
load_dotenv()
```

### From YAML

```yaml
env:
  NODE_ENV: production
  API_KEY: ${API_KEY}  # From environment
  DATABASE_URL: postgres://db:5432/myapp
  FEATURE_FLAGS:
    - ENABLE_CACHE
    - ENABLE_METRICS
```

## Volume Mounts

### Basic Mounts

```bash
# Read-write mount
--volume /host/path:/container/path

# Read-only mount
--volume /host/path:/container/path:ro

# Multiple mounts
--volume ./data:/app/data \
--volume ./config:/app/config:ro \
--volume /var/log/agent:/app/logs
```

### Mount Patterns

```yaml
volumes:
  # Data persistence
  - host: ./data
    container: /app/data
    mode: rw
    
  # Configuration
  - host: ./config
    container: /app/config
    mode: ro
    
  # Shared cache
  - host: /tmp/cache
    container: /app/cache
    mode: rw
    
  # Logs
  - host: ./logs
    container: /app/logs
    mode: rw
```

## Production Deployment Patterns

### 1. Single Agent with Full Options

```bash
agentainer deploy \
  --name prod-api \
  --image api:v1.2.3 \
  --env NODE_ENV=production \
  --env API_KEY="${API_KEY}" \
  --env DATABASE_URL="${DATABASE_URL}" \
  --volume /data/api:/app/data \
  --volume /config/api:/app/config:ro \
  --cpu 2 \
  --memory 2G \
  --health-endpoint /health \
  --health-interval 30s \
  --health-retries 3 \
  --auto-restart \
  --token "${AGENT_TOKEN}"
```

### 2. Microservices Architecture

```yaml
# microservices.yaml
apiVersion: v1
kind: AgentDeployment
spec:
  agents:
    # API Gateway
    - name: gateway
      image: gateway:v1.0
      env:
        SERVICES:
          - http://auth-service:8000
          - http://user-service:8000
          - http://order-service:8000
      resources:
        memory: 512M
        cpu: 1
      healthCheck:
        endpoint: /health
        
    # Auth Service
    - name: auth-service
      image: services/auth:v1.0
      env:
        JWT_SECRET: ${JWT_SECRET}
        REDIS_URL: redis://host.docker.internal:6379
      resources:
        memory: 256M
        
    # User Service  
    - name: user-service
      image: services/user:v1.0
      env:
        DATABASE_URL: ${USER_DB_URL}
      volumes:
        - host: ./user-data
          container: /app/data
      resources:
        memory: 512M
        
    # Order Service
    - name: order-service
      image: services/order:v1.0
      env:
        DATABASE_URL: ${ORDER_DB_URL}
        KAFKA_URL: ${KAFKA_URL}
      resources:
        memory: 1G
        cpu: 2
```

### 3. Data Pipeline

```yaml
# pipeline.yaml
apiVersion: v1
kind: AgentDeployment
spec:
  agents:
    # Data Collector
    - name: collector
      image: pipeline/collector:latest
      env:
        SOURCES:
          - https://api.example.com
          - https://data.example.org
        SCHEDULE: "*/5 * * * *"
      volumes:
        - host: ./raw-data
          container: /data/raw
          
    # Data Processor
    - name: processor
      image: pipeline/processor:latest
      env:
        INPUT_DIR: /data/raw
        OUTPUT_DIR: /data/processed
        MODEL_PATH: /models/latest
      volumes:
        - host: ./raw-data
          container: /data/raw
          mode: ro
        - host: ./processed-data
          container: /data/processed
        - host: ./models
          container: /models
          mode: ro
      resources:
        memory: 4G
        cpu: 4
        
    # Data Publisher
    - name: publisher
      image: pipeline/publisher:latest
      env:
        INPUT_DIR: /data/processed
        S3_BUCKET: my-data-bucket
        NOTIFICATION_URL: ${WEBHOOK_URL}
      volumes:
        - host: ./processed-data
          container: /data/processed
          mode: ro
```

## Best Practices

### 1. Use Specific Image Tags

```bash
# Good - specific version
--image my-agent:v1.2.3
--image my-agent:stable
--image my-agent:2024-01-15

# Avoid - ambiguous
--image my-agent:latest
--image my-agent
```

### 2. Set Resource Limits

Always set resource limits to prevent runaway agents:

```bash
--memory 512M   # Prevent memory leaks
--cpu 1         # Prevent CPU hogging
```

### 3. Configure Health Checks

Health checks enable automatic recovery:

```bash
--health-endpoint /health
--health-interval 30s
--auto-restart
```

### 4. Use Volumes for State

Persist important data outside containers:

```bash
--volume /data/agent-123:/app/data
```

### 5. Secure Sensitive Data

```bash
# Use environment variables for secrets
--env API_KEY="${API_KEY}"

# Use read-only mounts for configs
--volume ./config:/app/config:ro

# Use custom tokens
--token "${AGENT_TOKEN}"
```

### 6. Plan for Failures

```yaml
# Enable auto-restart
autoRestart: true

# Set health checks
healthCheck:
  endpoint: /health
  retries: 3
  
# Mount persistent storage
volumes:
  - host: ./data
    container: /app/data
```

## Troubleshooting Deployments

### Agent Won't Start

```bash
# Check deployment status
agentainer list

# Check logs
agentainer logs <agent-id>

# Verify image exists
docker images | grep my-agent

# Check resources
docker stats
```

### Health Check Failures

```bash
# Test health endpoint manually
curl http://localhost:8081/agent/<agent-id>/health

# Check health check configuration
agentainer inspect <agent-id>

# Increase timeout or retries
--health-timeout 10s --health-retries 5
```

### Volume Mount Issues

```bash
# Verify host path exists
ls -la /path/to/volume

# Check permissions
chmod 755 /path/to/volume

# Use absolute paths
--volume /absolute/path:/container/path
```

## Next Steps

- Learn about [Building Resilient Agents](./RESILIENT_AGENTS.md)
- Explore [API Endpoints](./API_ENDPOINTS.md)
- Understand [Network Architecture](./NETWORK_ARCHITECTURE.md)