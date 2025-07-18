# Simple Agent Example

This is a simple Python Flask-based agent that demonstrates how to create agents for the Agentainer platform.

## Features

- Health check endpoint
- Status reporting
- Request processing
- Simple echo functionality
- Request counting

## Building the Image

```bash
cd examples/simple-agent
docker build -t simple-agent:latest .
```

## Running with Agentainer

1. Deploy the agent:
```bash
./agentainer deploy --name simple-agent --image simple-agent:latest --env AGENT_NAME=my-simple-agent
```

2. Start the agent:
```bash
./agentainer start <agent-id>
```

3. Test the agent:
```bash
curl -X POST http://localhost:8080/agents/<agent-id>/invoke \
  -H "Authorization: Bearer agentainer-default-token" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello from Agentainer!"}'
```

## API Endpoints

- `GET /health` - Health check
- `GET /status` - Agent status and metrics
- `POST /process` - Process arbitrary data
- `POST /invoke` - Main invocation endpoint
- `GET /` - Root endpoint with information

## Environment Variables

- `AGENT_NAME` - Custom name for the agent (default: "simple-agent")

## Example Usage

```bash
# Health check
curl http://localhost:8000/health

# Get status
curl http://localhost:8000/status

# Process data
curl -X POST http://localhost:8000/process \
  -H "Content-Type: application/json" \
  -d '{"data": "test message"}'

# Invoke agent
curl -X POST http://localhost:8000/invoke \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello Agent!"}'
```