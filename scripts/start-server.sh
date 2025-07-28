#!/bin/bash
# Start Agentainer server properly

echo "Starting Agentainer server with proper network setup..."

# 1. Ensure network exists
if ! docker network inspect agentainer-network >/dev/null 2>&1; then
    echo "Creating agentainer-network..."
    docker network create agentainer-network
fi

# 2. Start Redis on host (for CLI access)
if ! docker ps | grep -q agentainer-redis; then
    echo "Starting Redis..."
    docker run -d \
        --name agentainer-redis \
        -p 6379:6379 \
        redis:7-alpine
fi

# 3. Build Agentainer image
echo "Building Agentainer image..."
docker build -t agentainer:latest .

# 4. Run Agentainer server as a container
echo "Starting Agentainer server..."
docker run -d \
    --name agentainer-server \
    --network agentainer-network \
    -p 8081:8081 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e AGENTAINER_REDIS_HOST=host.docker.internal \
    -e AGENTAINER_REDIS_PORT=6379 \
    -e AGENTAINER_SERVER_HOST=0.0.0.0 \
    -e AGENTAINER_AUTH_TOKEN=agentainer-default-token \
    --add-host host.docker.internal:host-gateway \
    agentainer:latest

echo ""
echo "✅ Agentainer server started!"
echo ""
echo "Server URL: http://localhost:8081"
echo "Health check: curl http://localhost:8081/health"
echo ""
echo "To view logs: docker logs -f agentainer-server"
echo "To stop: docker stop agentainer-server && docker rm agentainer-server"