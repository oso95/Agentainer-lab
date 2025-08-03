# Redis Configuration Unification Summary

## Problem Solved

Previously, Agentainer had two different startup methods with incompatible Redis configurations:
- **docker-compose**: Redis at hostname `redis`
- **make run**: Redis at `host.docker.internal`

This caused workflow agents to fail connecting to Redis depending on how Agentainer was started.

## Changes Made

### 1. Docker Compose Configuration (`docker-compose.yml`)
- Added Redis to both networks (`agentainer-internal` and `agentainer-network`)
- Made `agentainer-network` non-external (created by docker-compose)
- Added `extra_hosts` for `host.docker.internal` compatibility
- Added `AGENTAINER_DOCKER_NETWORK` environment variable

### 2. Makefile Updates (`Makefile`)
- Replaced custom `start-server.sh` with unified startup script
- Updated `run` target to use docker-compose directly
- Enhanced `stop` target to clean up all containers
- Made `docker-run` and `docker-stop` aliases for consistency

### 3. Orchestrator Updates (`internal/workflow/orchestrator.go`)
- Updated to read `AGENTAINER_REDIS_HOST` from environment
- Passes correct Redis host to workflow agents
- Maintains backward compatibility with fallback to `host.docker.internal`

### 4. New Unified Startup Script (`scripts/unified-start.sh`)
- Comprehensive prerequisite checks
- Builds Agentainer image
- Cleans up old containers
- Starts services with docker-compose
- Provides clear status and configuration info

### 5. Documentation Updates
- Created migration guide (`docs/UNIFIED_STARTUP_MIGRATION.md`)
- Updated README with unified startup instructions
- Added migration note for existing users

## How It Works Now

1. **Single Startup Method**: `make run` uses docker-compose internally
2. **Consistent Redis Access**:
   - From Agentainer: `redis:6379`
   - From workflow agents: `redis:6379`
   - From host: `localhost:6379`
3. **Automatic Configuration**: Workflow agents receive correct Redis host automatically

## Benefits

- **Consistency**: Same configuration regardless of startup method
- **Reliability**: No more Redis connection failures
- **Simplicity**: One way to start Agentainer
- **Compatibility**: Works on all platforms (macOS, Linux, Windows)

## Testing Instructions

To test the unified approach:

```bash
# 1. Stop any existing services
make stop

# 2. Start with unified method
make run

# 3. Test workflow demo
cd examples/workflow-demo
python3 run_llm_workflow.py
```

The workflow should complete successfully with all agents connecting to Redis properly.