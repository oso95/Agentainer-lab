#!/bin/bash
# Stop Agentainer server

echo "Stopping Agentainer server..."

# Stop and remove the server container
if docker ps -a | grep -q agentainer-server; then
    docker stop agentainer-server
    docker rm agentainer-server
    echo "✅ Agentainer server stopped"
else
    echo "ℹ️  No running Agentainer server found"
fi

# Optionally stop Redis
read -p "Do you want to stop Redis as well? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if docker ps -a | grep -q agentainer-redis; then
        docker stop agentainer-redis
        docker rm agentainer-redis
        echo "✅ Redis stopped"
    fi
fi

echo ""
echo "To restart the server, run: ./scripts/start-server.sh"