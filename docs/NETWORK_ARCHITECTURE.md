# Network Architecture

Agentainer uses Docker's networking capabilities to provide secure, isolated environments for agents while maintaining easy access through a unified proxy interface.

## Overview

```mermaid
graph TB
    subgraph "External Access"
        User[User/Client]
    end
    
    subgraph "Agentainer Server (Host)"
        Proxy[Proxy Service<br/>:8081]
        API[API Service<br/>:8081]
        Redis[Redis<br/>:6379]
    end
    
    subgraph "Docker Network (agentainer-network)"
        Agent1[Agent 1<br/>Container]
        Agent2[Agent 2<br/>Container]
        Agent3[Agent 3<br/>Container]
    end
    
    User -->|HTTP Requests| Proxy
    User -->|API Calls| API
    
    Proxy -->|Routes to| Agent1
    Proxy -->|Routes to| Agent2
    Proxy -->|Routes to| Agent3
    
    API -->|Manages| Agent1
    API -->|Manages| Agent2
    API -->|Manages| Agent3
    
    Agent1 -->|State| Redis
    Agent2 -->|State| Redis
    Agent3 -->|State| Redis
    
    Agent1 -.->|Internal| Agent2
    Agent2 -.->|Internal| Agent3
    
    style Proxy fill:#ffe4b5
    style API fill:#e6f3ff
    style Redis fill:#ffe4e4
```

## Key Concepts

### 1. Network Isolation

- All agents run in Docker containers on the `agentainer-network`
- Agents are NOT exposed on host ports directly
- All external access goes through the Agentainer proxy
- Agents can communicate internally using their agent IDs

### 2. Proxy Architecture

The Agentainer proxy provides:
- **Unified Access Point**: Single port (8081) for all agents
- **Request Routing**: Routes `/agent/{id}/*` to appropriate containers
- **No Authentication**: Proxy endpoints are public by design
- **Request Persistence**: Queues requests when agents are unavailable

### 3. Service Discovery

Agents can discover and communicate with each other:
```python
# From agent-123, call agent-456
response = requests.get('http://agent-456:8000/api/data')

# Or use the proxy (works from anywhere)
response = requests.get('http://host.docker.internal:8081/agent/agent-456/api/data')
```

## Network Modes

### Default Mode (Recommended)

Agentainer creates and manages its own Docker network:

```bash
# Automatic network creation
docker network create agentainer-network

# Agents join this network
docker run --network agentainer-network ...
```

### Host Network Access

Agents can access services running on the host:

```python
# Access Redis on host
redis_client = redis.Redis(host='host.docker.internal', port=6379)

# Access database on host
conn = psycopg2.connect("postgresql://host.docker.internal:5432/db")

# Access another service on host
response = requests.get('http://host.docker.internal:3000/api')
```

## Communication Patterns

### 1. External → Agent

Users access agents through the proxy:

```bash
# No authentication required
curl http://localhost:8081/agent/my-agent/
curl http://localhost:8081/agent/my-agent/api/endpoint
curl -X POST http://localhost:8081/agent/my-agent/process -d '{...}'
```

### 2. Agent → Agent

Agents communicate internally:

```python
# Direct internal communication (fast)
response = requests.post('http://other-agent:8000/api', json=data)

# Through proxy (works from anywhere)
response = requests.post(
    'http://host.docker.internal:8081/agent/other-agent/api',
    json=data
)
```

### 3. Agent → External

Agents can make external requests:

```python
# Call external APIs
response = requests.get('https://api.example.com/data')

# Access host services
redis_client = redis.Redis(host='host.docker.internal', port=6379)
```

### 4. Agent → Agentainer API

Agents can interact with Agentainer's API:

```python
# Get own status
headers = {'Authorization': 'Bearer agentainer-default-token'}
response = requests.get(
    f'http://host.docker.internal:8081/agents/{AGENT_ID}',
    headers=headers
)

# Get other agents' status
response = requests.get(
    'http://host.docker.internal:8081/agents',
    headers=headers
)
```

## Security Model

### Network Isolation

- Agents cannot bind to host ports
- All traffic flows through Agentainer proxy
- Inter-agent communication stays within Docker network
- No direct external access to agents

### Authentication Layers

1. **Proxy Access** (`/agent/*`): No authentication (public)
2. **API Access** (`/agents/*`): Bearer token required
3. **Agent Internal**: Agents handle their own auth

### Example Security Setup

```python
# Agent with internal authentication
from flask import Flask, request, jsonify

app = Flask(__name__)

# Public endpoint (accessible via proxy)
@app.route('/health')
def health():
    return {'status': 'healthy'}

# Protected endpoint (agent handles auth)
@app.route('/api/sensitive', methods=['POST'])
def sensitive():
    api_key = request.headers.get('X-API-Key')
    if api_key != 'secret-key':
        return {'error': 'Unauthorized'}, 401
    return {'data': 'sensitive information'}
```

## Deployment Scenarios

### 1. Single Host (Development)

```bash
# Agentainer runs on host
make run

# Agents run in containers
agentainer deploy --name dev-agent --image my-agent:latest

# Access pattern
curl http://localhost:8081/agent/dev-agent/
```

### 2. Docker-in-Docker

```yaml
# docker-compose.yml
services:
  agentainer:
    image: agentainer:latest
    ports:
      - "8081:8081"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - AGENTAINER_REDIS_HOST=redis
    networks:
      - agentainer-network

  redis:
    image: redis:7-alpine
    networks:
      - agentainer-network

networks:
  agentainer-network:
    driver: bridge
```

### 3. Kubernetes (Future)

```yaml
# Conceptual - not yet implemented
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agentainer
spec:
  template:
    spec:
      containers:
      - name: agentainer
        image: agentainer:latest
        ports:
        - containerPort: 8081
        volumeMounts:
        - name: docker-sock
          mountPath: /var/run/docker.sock
```

## Network Troubleshooting

### Common Issues

#### "502 Bad Gateway"

The proxy cannot reach the agent:

```bash
# Check agent is running
agentainer list
docker ps | grep <agent-id>

# Check agent logs
agentainer logs <agent-id>

# Verify network
docker network inspect agentainer-network
```

#### "Connection Refused"

Agent isn't listening on expected port:

```bash
# Check agent's exposed port
docker inspect <agent-id> | grep -A 10 "Config"

# Ensure agent listens on 0.0.0.0:8000
# Not on 127.0.0.1:8000 (localhost only)
```

#### "Cannot reach other agent"

Inter-agent communication issues:

```bash
# Verify both agents on same network
docker inspect <agent-1> | grep NetworkMode
docker inspect <agent-2> | grep NetworkMode

# Test connectivity
docker exec <agent-1> ping <agent-2>
```

### Network Debugging Tools

```bash
# List all agents and their IPs
docker network inspect agentainer-network

# Test connectivity from agent
docker exec <agent-id> curl http://other-agent:8000/health

# Check DNS resolution
docker exec <agent-id> nslookup other-agent

# Monitor network traffic
docker exec <agent-id> tcpdump -i eth0 port 8000
```

## Best Practices

### 1. Use Agent Names for Internal Communication

```python
# Good - uses Docker DNS
response = requests.get('http://auth-service:8000/verify')

# Avoid - hardcoded IPs
response = requests.get('http://172.17.0.3:8000/verify')
```

### 2. Handle Network Failures

```python
import requests
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

# Configure retries
session = requests.Session()
retry = Retry(total=3, backoff_factor=0.3)
adapter = HTTPAdapter(max_retries=retry)
session.mount('http://', adapter)

# Use session for requests
response = session.get('http://other-agent:8000/api')
```

### 3. Set Proper Timeouts

```python
# Set reasonable timeouts
try:
    response = requests.get(
        'http://other-agent:8000/api',
        timeout=(3, 30)  # (connect, read) timeouts
    )
except requests.Timeout:
    # Handle timeout gracefully
    pass
```

### 4. Use Health Checks

```python
# Implement health endpoint
@app.route('/health')
def health():
    # Check dependencies
    checks = {
        'database': check_database(),
        'redis': check_redis(),
        'dependent_service': check_other_agent()
    }
    
    if all(checks.values()):
        return {'status': 'healthy', 'checks': checks}
    else:
        return {'status': 'unhealthy', 'checks': checks}, 503
```

## Advanced Patterns

### Service Mesh Pattern

Create a service mesh with agent intercommunication:

```yaml
agents:
  # API Gateway (public facing)
  - name: gateway
    env:
      UPSTREAM_SERVICES:
        - http://auth:8000
        - http://users:8000
        - http://orders:8000
  
  # Internal services
  - name: auth
    env:
      REDIS_URL: redis://host.docker.internal:6379
      
  - name: users
    env:
      AUTH_SERVICE: http://auth:8000
      DB_URL: postgres://host.docker.internal:5432/users
      
  - name: orders
    env:
      AUTH_SERVICE: http://auth:8000
      USER_SERVICE: http://users:8000
      DB_URL: postgres://host.docker.internal:5432/orders
```

### Event-Driven Architecture

Use Redis for pub/sub between agents:

```python
# Publisher agent
import redis

r = redis.Redis(host='host.docker.internal', port=6379)

def publish_event(event_type, data):
    r.publish(f'events:{event_type}', json.dumps(data))

# Subscriber agent
def listen_for_events():
    pubsub = r.pubsub()
    pubsub.subscribe('events:*')
    
    for message in pubsub.listen():
        if message['type'] == 'message':
            handle_event(message['channel'], message['data'])
```

## Performance Considerations

### Proxy Overhead

- Proxy adds ~1-2ms latency
- Use direct agent-to-agent communication for high-frequency calls
- Proxy handles thousands of requests/second

### Network Optimization

```python
# Connection pooling for agent-to-agent
class ServiceClient:
    def __init__(self, service_name):
        self.base_url = f'http://{service_name}:8000'
        self.session = requests.Session()
        # Keep connections alive
        self.session.mount('http://', HTTPAdapter(
            pool_connections=10,
            pool_maxsize=20
        ))
```

### Caching Strategies

```python
# Cache other agent responses
import redis
import json

cache = redis.Redis(host='host.docker.internal', port=6379)

def get_user_data(user_id):
    # Check cache first
    cached = cache.get(f'user:{user_id}')
    if cached:
        return json.loads(cached)
    
    # Fetch from user service
    response = requests.get(f'http://user-service:8000/users/{user_id}')
    data = response.json()
    
    # Cache for 5 minutes
    cache.setex(f'user:{user_id}', 300, json.dumps(data))
    return data
```

## Future Enhancements

- **Service Discovery**: Automatic service registration
- **Load Balancing**: Multiple instances per agent
- **Circuit Breakers**: Automatic failure handling
- **Distributed Tracing**: Request flow visualization
- **mTLS**: Mutual TLS between agents