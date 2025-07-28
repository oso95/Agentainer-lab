#!/bin/bash
# Run Agentainer server with access to Docker network
# This script creates a bridge to allow the server to communicate with containers

set -e

# Ensure agentainer network exists
docker network inspect agentainer-network >/dev/null 2>&1 || \
    docker network create agentainer-network

# Option 1: Run agentainer server in a container (recommended)
run_in_container() {
    echo "Starting Agentainer server in container mode..."
    
    # Build a minimal container with just the agentainer binary
    cat > /tmp/Dockerfile.server <<EOF
FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY agentainer /usr/local/bin/
CMD ["agentainer", "server"]
EOF

    # Build the server image
    docker build -t agentainer-server:local -f /tmp/Dockerfile.server .
    
    # Run the server container
    docker run -d \
        --name agentainer-server \
        --network agentainer-network \
        -p 8081:8081 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $HOME/.agentainer:/root/.agentainer \
        -e AGENTAINER_REDIS_HOST=host.docker.internal \
        -e AGENTAINER_REDIS_PORT=6379 \
        --add-host host.docker.internal:host-gateway \
        agentainer-server:local
    
    echo "Agentainer server is running in container mode"
    echo "Access at: http://localhost:8081"
    echo "View logs: docker logs -f agentainer-server"
}

# Option 2: Use socat to bridge networks (alternative)
run_with_bridge() {
    echo "Starting Agentainer server with network bridge..."
    
    # This would require additional setup with socat or similar tools
    echo "This option requires additional network configuration."
    echo "Recommended: Use container mode instead."
}

# Check if we should run in container mode
if [ "$1" == "--container" ] || [ "$1" == "-c" ]; then
    run_in_container
else
    echo "Usage: $0 --container"
    echo ""
    echo "This script runs Agentainer server with access to the Docker network."
    echo "The recommended approach is to run the server in a container."
    echo ""
    echo "Prerequisites:"
    echo "  - Redis must be accessible from Docker containers"
    echo "  - The agentainer binary must be in the current directory"
fi