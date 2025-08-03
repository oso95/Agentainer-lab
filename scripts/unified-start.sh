#!/bin/bash
# Unified Agentainer startup script
# Handles both docker-compose and standalone deployment scenarios

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting Agentainer with unified configuration...${NC}"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if docker compose is available
docker_compose_exists() {
    docker compose version >/dev/null 2>&1 || command_exists docker-compose
}

# Function to get docker compose command
get_docker_compose_cmd() {
    if docker compose version >/dev/null 2>&1; then
        echo "docker compose"
    elif command_exists docker-compose; then
        echo "docker-compose"
    else
        echo ""
    fi
}

# Check prerequisites
echo -e "${BLUE}Checking prerequisites...${NC}"

if ! command_exists docker; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Error: Docker daemon is not running${NC}"
    exit 1
fi

COMPOSE_CMD=$(get_docker_compose_cmd)
if [ -z "$COMPOSE_CMD" ]; then
    echo -e "${RED}Error: Docker Compose is not installed${NC}"
    echo "Please install Docker Compose to continue"
    exit 1
fi

# Ensure we're in the project root
if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}Error: docker-compose.yml not found${NC}"
    echo "Please run this script from the Agentainer project root"
    exit 1
fi

# Build the Agentainer image
echo -e "${BLUE}Building Agentainer image...${NC}"
if ! docker build -t agentainer:latest .; then
    echo -e "${RED}Docker build failed${NC}"
    exit 1
fi

# Stop any existing containers
echo -e "${BLUE}Stopping any existing Agentainer services...${NC}"
$COMPOSE_CMD down >/dev/null 2>&1 || true

# Remove old standalone containers if they exist
docker stop agentainer-server agentainer-redis >/dev/null 2>&1 || true
docker rm agentainer-server agentainer-redis >/dev/null 2>&1 || true

# Clean up existing network if it wasn't created by compose
if docker network inspect agentainer-network >/dev/null 2>&1; then
    # Check if network was created by compose
    NETWORK_LABELS=$(docker network inspect agentainer-network --format '{{json .Labels}}')
    if ! echo "$NETWORK_LABELS" | grep -q "com.docker.compose.network"; then
        echo -e "${YELLOW}Removing old agentainer-network (not created by compose)...${NC}"
        # First, disconnect any containers
        for container in $(docker ps -a --filter "network=agentainer-network" --format "{{.ID}}"); do
            docker network disconnect agentainer-network "$container" 2>/dev/null || true
        done
        # Now remove the network
        docker network rm agentainer-network 2>/dev/null || true
    fi
fi

# Ensure the external network exists
if ! docker network inspect agentainer-network >/dev/null 2>&1; then
    echo -e "${BLUE}Creating agentainer-network...${NC}"
    docker network create agentainer-network
fi

# Start services with docker-compose
echo -e "${BLUE}Starting Agentainer services...${NC}"
if ! $COMPOSE_CMD up -d; then
    echo -e "${RED}Failed to start services${NC}"
    exit 1
fi

# Wait for services to be ready
echo -e "${BLUE}Waiting for services to be ready...${NC}"
for i in {1..30}; do
    if curl -s http://localhost:8081/health >/dev/null 2>&1; then
        break
    fi
    printf "."
    sleep 1
done
echo ""

# Verify services are running
if ! curl -s http://localhost:8081/health >/dev/null 2>&1; then
    echo -e "${RED}Error: Agentainer server failed to start${NC}"
    echo "Check logs with: $COMPOSE_CMD logs"
    exit 1
fi

# Get Redis status
REDIS_STATUS="Not connected"
if $COMPOSE_CMD exec -T redis redis-cli ping >/dev/null 2>&1; then
    REDIS_STATUS="Connected"
fi

echo ""
echo -e "${GREEN}✅ Agentainer started successfully!${NC}"
echo ""
echo -e "${BLUE}🌐 Server Endpoints:${NC}"
echo "   • API Server: http://localhost:8081"
echo "   • Health Check: http://localhost:8081/health"
echo ""
echo -e "${BLUE}📊 Agentainer Flow Dashboard:${NC} http://localhost:8081/dashboard"
echo ""
echo -e "${BLUE}🔧 Service Status:${NC}"
echo "   • Agentainer Server: Running"
echo "   • Redis: $REDIS_STATUS"
echo ""
echo -e "${YELLOW}🔧 Server Management:${NC}"
echo "   • View logs: $COMPOSE_CMD logs -f"
echo "   • Stop server: make stop"
echo ""
echo -e "${YELLOW}📚 Quick Start Examples:${NC}"
echo "   • GPT Agent: cd examples/gpt-agent && ./run.sh"
echo "   • Gemini Agent: cd examples/gemini-agent && ./run.sh"
echo "   • Workflow Demo: cd examples/workflow-demo && python3 run_llm_workflow.py"
echo ""
echo -e "${BLUE}ℹ️  Configuration:${NC}"
echo "   • Redis is accessible at:"
echo "     - Inside containers: redis:6379"
echo "     - From host: localhost:6379"
echo "   • Workflow agents will automatically connect to the correct Redis host"