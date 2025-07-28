# Building Resilient Agents

This guide covers patterns and best practices for building agents that can handle failures gracefully and maintain state across restarts.

## Overview

Agentainer provides infrastructure-level resilience features:
- **Automatic Request Replay**: Failed requests are queued and replayed when agents recover
- **Crash Recovery**: Agents can be restarted with `agentainer resume` after any failure
- **Persistent Volumes**: Mount volumes to preserve agent data across restarts

However, your agent code needs to handle its own application-level state. This guide covers proven patterns for implementing resilient agent logic.

## Division of Responsibilities

| Feature | Agentainer Provides | Your Agent Code Handles |
|---------|-------------------|------------------------|
| Container restart | ✅ Auto-restart on crash | Save state before crash |
| HTTP requests | ✅ Queue & replay failed requests | Process requests idempotently |
| Storage | ✅ Persistent volume mounts | Read/write state files |
| Networking | ✅ Proxy & internal network | Handle connection errors |
| Lifecycle | ✅ Start/stop/pause/resume | Graceful shutdown logic |

## Why These Patterns?

While Agentainer handles:
- ✅ Restarting crashed containers
- ✅ Replaying failed HTTP requests
- ✅ Preserving volume data

Your agent code should handle:
- ❌ Saving processing state between requests
- ❌ Resuming interrupted batch operations
- ❌ Graceful shutdown on SIGTERM
- ❌ Checkpoint/restore for long-running tasks

## Pattern 1: State Persistence

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

## Pattern 2: Auto-Recovery with Checkpoints

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

## Pattern 3: Redis-Based State Management

**Use this when:** You need fast, shared state across agent instances

```python
# Redis-backed agent using Agentainer's Redis
import redis
import json
from flask import Flask, request, jsonify

app = Flask(__name__)

# Connect to Agentainer's Redis
redis_client = redis.Redis(
    host='host.docker.internal',  # Agentainer's Redis
    port=6379,
    decode_responses=True
)

@app.route('/process', methods=['POST'])
def process():
    data = request.json
    task_id = data['task_id']
    
    # Check if already processed (idempotency)
    if redis_client.exists(f"processed:{task_id}"):
        result = redis_client.get(f"processed:{task_id}")
        return jsonify({"result": json.loads(result), "cached": True})
    
    # Process the task
    result = perform_processing(data)
    
    # Save result with expiration
    redis_client.setex(
        f"processed:{task_id}", 
        3600,  # 1 hour TTL
        json.dumps(result)
    )
    
    # Update metrics
    redis_client.hincrby("agent:metrics", "processed_count", 1)
    
    return jsonify({"result": result, "cached": False})

def perform_processing(data):
    # Your processing logic here
    return {"status": "completed", "data": data}
```

## Pattern 4: Event-Driven Processing

**Use this when:** Your agent needs to process events or messages asynchronously

```python
# Event-driven agent with retry logic
import time
import json
from threading import Thread
from queue import Queue

class EventProcessor:
    def __init__(self):
        self.event_queue = Queue()
        self.retry_queue = Queue()
        self.running = True
        
        # Start processing threads
        Thread(target=self.process_events, daemon=True).start()
        Thread(target=self.retry_failed_events, daemon=True).start()
    
    def add_event(self, event):
        """Add event to processing queue"""
        self.event_queue.put(event)
    
    def process_events(self):
        """Main event processing loop"""
        while self.running:
            try:
                event = self.event_queue.get(timeout=1)
                self.process_single_event(event)
            except Exception as e:
                print(f"Error processing event: {e}")
                # Add to retry queue
                event['retry_count'] = event.get('retry_count', 0) + 1
                if event['retry_count'] < 3:
                    self.retry_queue.put(event)
    
    def retry_failed_events(self):
        """Retry failed events with exponential backoff"""
        while self.running:
            try:
                event = self.retry_queue.get(timeout=1)
                # Exponential backoff
                wait_time = 2 ** event['retry_count']
                time.sleep(wait_time)
                self.event_queue.put(event)
            except:
                pass
    
    def process_single_event(self, event):
        """Process individual event - override this"""
        pass
```

## Deployment Examples

### Basic Resilient Agent

```bash
# Deploy with persistent volume
agentainer deploy \
  --name my-resilient-agent \
  --image ./Dockerfile \
  --volume /host/data:/app/data \
  --auto-restart

# The agent will:
# - Save state to /app/data (persisted on host)
# - Automatically restart on failure
# - Resume from last checkpoint on startup
# - Have requests queued if it crashes
```

### High-Availability Agent

```bash
# Deploy with health checks and monitoring
agentainer deploy \
  --name ha-agent \
  --image ./Dockerfile \
  --volume /data/agent:/app/data \
  --health-endpoint /health \
  --health-interval 30s \
  --health-retries 3 \
  --auto-restart \
  --env REDIS_HOST=host.docker.internal

# Features:
# - Health monitoring every 30 seconds
# - Auto-restart after 3 failed health checks
# - Shared state via Redis
# - Persistent data storage
```

### Batch Processing Agent

```bash
# Deploy batch processor with large timeout
agentainer deploy \
  --name batch-processor \
  --image ./batch-agent:latest \
  --volume /data/batches:/app/batches \
  --volume /data/checkpoints:/app/checkpoints \
  --env BATCH_SIZE=1000 \
  --env CHECKPOINT_INTERVAL=100 \
  --memory 2G \
  --cpu 2

# Handles:
# - Large batch processing
# - Checkpoint every 100 items
# - Resume from checkpoint on restart
# - Resource limits for stability
```

## Best Practices

### 1. Idempotent Operations

Make your operations idempotent so they can be safely retried:

```python
@app.route('/process', methods=['POST'])
def process():
    request_id = request.json.get('request_id')
    
    # Check if already processed
    existing_result = check_if_processed(request_id)
    if existing_result:
        return existing_result
    
    # Process and save with request_id
    result = do_processing(request.json)
    save_result(request_id, result)
    
    return result
```

### 2. Graceful Shutdown

Always handle shutdown signals:

```python
import signal
import sys

def shutdown_handler(signum, frame):
    print("Shutting down gracefully...")
    # Save state
    save_current_state()
    # Close connections
    close_all_connections()
    sys.exit(0)

signal.signal(signal.SIGTERM, shutdown_handler)
signal.signal(signal.SIGINT, shutdown_handler)
```

### 3. Health Endpoints

Implement comprehensive health checks:

```python
@app.route('/health')
def health():
    checks = {
        "redis": check_redis_connection(),
        "disk_space": check_disk_space(),
        "memory": check_memory_usage(),
        "processing_queue": check_queue_size()
    }
    
    status = "healthy" if all(checks.values()) else "unhealthy"
    status_code = 200 if status == "healthy" else 503
    
    return jsonify({
        "status": status,
        "checks": checks,
        "timestamp": datetime.now().isoformat()
    }), status_code
```

### 4. Structured Logging

Use structured logging for better debugging:

```python
import logging
import json

class JSONFormatter(logging.Formatter):
    def format(self, record):
        log_obj = {
            "timestamp": datetime.now().isoformat(),
            "level": record.levelname,
            "message": record.getMessage(),
            "agent_id": os.environ.get('AGENT_ID'),
            "function": record.funcName,
            "line": record.lineno
        }
        return json.dumps(log_obj)

# Configure logging
handler = logging.StreamHandler()
handler.setFormatter(JSONFormatter())
logging.getLogger().addHandler(handler)
logging.getLogger().setLevel(logging.INFO)
```

## Testing Resilience

### Test Crash Recovery

```bash
# 1. Deploy your agent
agentainer deploy --name test-agent --image ./Dockerfile

# 2. Start sending requests
for i in {1..100}; do
  curl -X POST http://localhost:8081/agent/test-agent/process \
    -d "{\"id\": $i, \"data\": \"test\"}"
done &

# 3. Simulate crash mid-processing
docker kill test-agent

# 4. Check queued requests
agentainer requests test-agent

# 5. Resume agent
agentainer resume test-agent

# 6. Verify all requests were processed
curl http://localhost:8081/agent/test-agent/stats
```

### Test State Persistence

```bash
# 1. Send data that updates state
curl -X POST http://localhost:8081/agent/my-agent/update \
  -d '{"key": "user_123", "value": "important_data"}'

# 2. Stop the agent
agentainer stop my-agent

# 3. Start it again
agentainer start my-agent

# 4. Verify state was preserved
curl http://localhost:8081/agent/my-agent/get?key=user_123
# Should return: {"value": "important_data"}
```

## Common Pitfalls to Avoid

1. **Not handling SIGTERM**: Agents get 10 seconds to shutdown gracefully
2. **Blocking operations**: Use timeouts for all external calls
3. **Memory leaks**: Monitor memory usage and implement cleanup
4. **Not saving state frequently**: Save after each significant operation
5. **Ignoring idempotency**: Assume every request might be retried

## Conclusion

Building resilient agents requires thinking about failure modes and recovery strategies. Agentainer provides the infrastructure foundation—request persistence, crash recovery, and state management—while your agent code implements application-specific resilience patterns.

The key is to leverage both layers effectively:
- Use Agentainer for infrastructure resilience
- Implement these patterns for application resilience
- Test failure scenarios regularly
- Monitor and iterate based on production experience