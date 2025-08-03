# Unified Startup Migration Guide

## Overview

We have unified the Agentainer startup process to resolve Redis connectivity issues between different deployment methods. Previously, there were inconsistencies between `docker-compose up` and `make run` that caused workflow agents to fail connecting to Redis.

## What Changed

### Before
- **docker-compose**: Redis was accessible at hostname `redis` inside the network
- **make run**: Redis was in a separate container, accessible at `host.docker.internal`
- **Problem**: Workflow agents couldn't connect to Redis consistently

### After
- **Unified approach**: All services use docker-compose
- **Redis hostname**: Consistent across all deployment methods
- **Automatic configuration**: Workflow agents receive correct Redis connection info

## Migration Steps

### 1. Stop Existing Services

If you have Agentainer running with the old method:

```bash
# Stop any existing services
docker stop agentainer-server agentainer-redis
docker rm agentainer-server agentainer-redis
docker compose down
```

### 2. Update Your Code

Pull the latest changes:
```bash
git pull
```

### 3. Start Agentainer

Use the unified startup method:
```bash
make run
```

Or directly with docker-compose:
```bash
docker compose up -d
```

## Key Changes

### For Users

1. **Single startup command**: Just use `make run`
2. **Consistent Redis access**: No more connection issues
3. **Better error messages**: Clear feedback if something goes wrong

### For Developers

1. **Environment Variables**:
   - Agentainer reads `AGENTAINER_REDIS_HOST` from environment
   - Workflow agents receive correct `REDIS_HOST` automatically
   - No manual configuration needed

2. **Docker Compose Configuration**:
   - Redis is part of both `agentainer-internal` and `agentainer-network`
   - Agentainer has `extra_hosts` for `host.docker.internal` compatibility
   - Network is created automatically, not external

3. **Orchestrator Updates**:
   - Reads `AGENTAINER_REDIS_HOST` from environment
   - Passes correct Redis host to workflow agents
   - Falls back to `host.docker.internal` if not set

## Troubleshooting

### Redis Connection Issues

If workflow agents still can't connect to Redis:

1. **Check Redis is running**:
   ```bash
   docker compose ps
   docker compose exec redis redis-cli ping
   ```

2. **Verify environment variables**:
   ```bash
   docker compose exec agentainer env | grep REDIS
   ```

3. **Check agent logs**:
   ```bash
   docker logs <agent-container-id>
   ```

### Network Issues

1. **Verify networks exist**:
   ```bash
   docker network ls | grep agentainer
   ```

2. **Inspect network configuration**:
   ```bash
   docker network inspect agentainer-network
   ```

## Benefits

1. **Consistency**: Same Redis configuration regardless of startup method
2. **Reliability**: Workflow agents always connect successfully
3. **Simplicity**: One way to start Agentainer
4. **Compatibility**: Works on macOS, Linux, and Windows

## Technical Details

### Redis Access Points

- **From Agentainer container**: `redis:6379`
- **From workflow agents**: `redis:6379` (when on agentainer-network)
- **From host machine**: `localhost:6379`
- **Legacy compatibility**: `host.docker.internal:6379` (if needed)

### Environment Variable Flow

1. docker-compose.yml sets `AGENTAINER_REDIS_HOST=redis`
2. Agentainer reads this and connects to Redis
3. Orchestrator reads `AGENTAINER_REDIS_HOST` when creating agents
4. Workflow agents receive `REDIS_HOST=redis` in their environment
5. Agents connect successfully to Redis

## Rollback

If you need to use the old method temporarily:

```bash
# Use the old start-server.sh script
chmod +x scripts/start-server.sh
./scripts/start-server.sh
```

However, we recommend migrating to the unified approach as soon as possible.