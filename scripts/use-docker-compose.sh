#!/bin/bash
# Script to use Agentainer CLI with docker-compose setup

# Export environment variables to point to docker-compose services
export AGENTAINER_REDIS_HOST=localhost
export AGENTAINER_REDIS_PORT=6379
export AGENTAINER_SERVER_HOST=localhost
export AGENTAINER_SERVER_PORT=8081

echo "Configured to use docker-compose Agentainer"
echo "Redis: $AGENTAINER_REDIS_HOST:$AGENTAINER_REDIS_PORT"
echo "Server: $AGENTAINER_SERVER_HOST:$AGENTAINER_SERVER_PORT"
echo ""
echo "You can now use 'agentainer' commands with the docker-compose setup"