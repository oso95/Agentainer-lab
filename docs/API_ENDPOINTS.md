# Agentainer API Endpoints Reference

## Overview

Agentainer has two types of endpoints:

1. **API Endpoints** (`/agents/*`) - For managing agents (requires authentication)
2. **Proxy Endpoints** (`/agent/*`) - For accessing agents directly (no authentication)

## API Endpoints (Management)

All API endpoints require authentication via Bearer token in the header:
```
Authorization: Bearer <your-token>
```

### Agent Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/agents` | Deploy a new agent |
| GET | `/agents` | List all agents |
| GET | `/agents/{id}` | Get specific agent details |
| DELETE | `/agents/{id}` | Remove an agent |

### Agent Control

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/agents/{id}/start` | Start an agent |
| POST | `/agents/{id}/stop` | Stop an agent |
| POST | `/agents/{id}/restart` | Restart an agent |
| POST | `/agents/{id}/pause` | Pause an agent |
| POST | `/agents/{id}/resume` | Resume a paused agent |

### Agent Monitoring

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/agents/{id}/logs` | Get agent logs |
| GET | `/agents/{id}/health` | Get agent health status |
| GET | `/agents/{id}/metrics` | Get current metrics |
| GET | `/agents/{id}/metrics/history` | Get metrics history |
| GET | `/health/agents` | Get all agents health status |

### Request Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/agents/{id}/invoke` | Invoke agent endpoint |
| GET | `/agents/{id}/requests` | List agent requests |
| GET | `/agents/{id}/requests/{reqId}` | Get specific request |
| POST | `/agents/{id}/requests/{reqId}/replay` | Replay a request |

## Proxy Endpoints (Direct Access)

The proxy endpoint provides direct access to agents **without authentication**:

| Method | Endpoint | Description |
|--------|----------|-------------|
| ANY | `/agent/{id}/*` | Proxy any request to the agent |

### Examples

```bash
# Access agent's root endpoint
GET http://localhost:8081/agent/agent-123/

# Access agent's health endpoint
GET http://localhost:8081/agent/agent-123/health

# POST to agent's API
POST http://localhost:8081/agent/agent-123/api/chat
```

## Key Differences

### `/agents/{id}` (API)
- **Purpose**: Management operations
- **Auth**: Required
- **Returns**: Agent metadata and status
- **Example**: `GET /agents/agent-123` returns agent configuration

### `/agent/{id}/` (Proxy)
- **Purpose**: Direct agent access
- **Auth**: Not required
- **Returns**: Whatever the agent returns
- **Example**: `GET /agent/agent-123/` forwards to agent's root endpoint

## Common Confusion Points

1. **Plural vs Singular**:
   - `/agents/*` = API endpoints (plural)
   - `/agent/*` = Proxy endpoint (singular)

2. **Authentication**:
   - API endpoints require Bearer token
   - Proxy endpoints are public

3. **Trailing Slash**:
   - Proxy paths must include trailing slash: `/agent/{id}/`
   - API paths don't use trailing slash: `/agents/{id}`

## Quick Reference

```bash
# Deploy agent (API - requires auth)
curl -X POST http://localhost:8081/agents \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-agent", "image": "my-agent:latest"}'

# Get agent info (API - requires auth)
curl http://localhost:8081/agents/agent-123 \
  -H "Authorization: Bearer your-token"

# Access agent directly (Proxy - no auth)
curl http://localhost:8081/agent/agent-123/

# Call agent's API (Proxy - no auth)
curl -X POST http://localhost:8081/agent/agent-123/api/endpoint \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'
```