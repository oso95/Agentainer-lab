# Agentainer Network Architecture

## Overview

Agentainer uses Docker's internal networking to isolate agents while providing a unified proxy interface. This document explains the network architecture and deployment options.

## Network Design

### Agent Network Isolation

- All agents run in Docker containers on the `agentainer-network`
- Agents are NOT exposed on host ports for security
- Each agent gets a hostname equal to its agent ID within the Docker network
- Agents can communicate with each other using their agent IDs as hostnames

### Proxy Architecture

The Agentainer server acts as a reverse proxy:
- Listens on host port 8081
- Routes requests to agents based on URL path: `/agent/{id}/*`
- Handles authentication, request persistence, and monitoring

## Deployment Options

### Option 1: Docker-in-Docker (Recommended for Development)

Run Agentainer server as a container on the same Docker network:

```bash
# Create the network
docker network create agentainer-network

# Run Redis (accessible from both host and containers)
docker run -d \
  --name agentainer-redis \
  -p 6379:6379 \
  redis:7-alpine

# Build and run Agentainer server container
docker build -t agentainer-server .
docker run -d \
  --name agentainer-server \
  --network agentainer-network \
  -p 8081:8081 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENTAINER_REDIS_HOST=host.docker.internal \
  --add-host host.docker.internal:host-gateway \
  agentainer-server
```

### Option 2: Host with Bridge Network (Production)

For production deployments where the server runs on the host:

1. **Use Container IPs**: The server automatically resolves container IPs
2. **Network Bridge**: Set up a bridge to allow host-to-container communication
3. **Service Mesh**: Use a service mesh or proxy container

### Option 3: Kubernetes/Docker Swarm

For orchestrated environments:
- Deploy Agentainer as a DaemonSet or Global Service
- Use service discovery for agent resolution
- Leverage native load balancing

## Network Flow

```
User Request → Agentainer Server (Port 8081) → Agent Container (Port 8000)
     ↓                    ↓                            ↓
   HTTP              Proxy Logic               Internal Network
              (Auth, Routing, Persistence)        (agentainer-network)
```

## Important Notes

1. **Redis Location**: Redis should be accessible from both the host (for CLI) and containers (for server when containerized)

2. **Docker Socket**: The server needs access to Docker socket for container management

3. **Security**: Agents are isolated and can only be accessed through the authenticated proxy

## Troubleshooting

### "502 Bad Gateway" Errors

This typically means the server cannot reach the agent container:

1. **Check agent is running**: `agentainer list`
2. **Verify network**: `docker network inspect agentainer-network`
3. **Test connectivity**: Use the server's network resolver to get container IP

### "Agent not found" Errors

1. **Sync delay**: Wait 10 seconds for automatic state sync
2. **Check Redis**: Ensure Redis is accessible from all components
3. **Verify agent exists**: `docker ps | grep <agent-id>`

## Best Practices

1. **Development**: Use docker-compose or run server in container
2. **Production**: Use orchestration platform with service discovery
3. **Testing**: Always verify network connectivity before deploying agents
4. **Monitoring**: Use health checks and metrics endpoints

## Architecture Decisions

The decision to use container hostnames instead of IPs was made to:
1. Support dynamic environments where IPs change
2. Enable service discovery patterns
3. Simplify multi-host deployments
4. Align with cloud-native practices

However, this requires the Agentainer server to be on the same network as agents or have a routing mechanism to reach them.