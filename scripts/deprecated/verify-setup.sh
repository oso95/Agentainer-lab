#!/bin/bash

# Verification script to check Agentainer Lab setup

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Agentainer Lab - Setup Verification"
echo "==================================="
echo ""

# Function to check command
check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ $1 is installed${NC}"
        return 0
    else
        echo -e "${RED}✗ $1 is NOT installed${NC}"
        return 1
    fi
}

# Function to check file
check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓ File exists: $1${NC}"
        return 0
    else
        echo -e "${RED}✗ File missing: $1${NC}"
        return 1
    fi
}

# Function to check directory
check_dir() {
    if [ -d "$1" ]; then
        echo -e "${GREEN}✓ Directory exists: $1${NC}"
        return 0
    else
        echo -e "${RED}✗ Directory missing: $1${NC}"
        return 1
    fi
}

# Check prerequisites
echo "Checking prerequisites..."
echo "------------------------"
check_command git
check_command go
check_command docker
check_command docker-compose || docker compose version >/dev/null 2>&1

# Check Go version
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "  Go version: $GO_VERSION"
fi

# Check Docker status
if command -v docker >/dev/null 2>&1; then
    if docker ps >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Docker daemon is running${NC}"
    else
        echo -e "${RED}✗ Docker daemon is NOT running or not accessible${NC}"
    fi
fi

echo ""
echo "Checking repository files..."
echo "---------------------------"

# Check critical files
check_file "go.mod"
check_file "go.sum"
check_file "Dockerfile"
check_file "docker-compose.yml"
check_file "cmd/agentainer/main.go"
check_dir "internal"
check_dir "pkg"

echo ""
echo "Checking build context..."
echo "------------------------"

# Check if we're in the right directory
if [ -f "go.mod" ] && grep -q "module github.com/oso95/agentainer-lab" go.mod 2>/dev/null; then
    echo -e "${GREEN}✓ In correct directory${NC}"
else
    echo -e "${RED}✗ Not in Agentainer Lab root directory${NC}"
fi

# Test Docker build
echo ""
echo "Testing Docker build..."
echo "----------------------"
if [ -f "Dockerfile" ] && [ -f "cmd/agentainer/main.go" ]; then
    echo "Running: docker build --no-cache -t agentainer-test ."
    if docker build --no-cache -t agentainer-test . >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Docker build successful${NC}"
        docker rmi agentainer-test >/dev/null 2>&1
    else
        echo -e "${RED}✗ Docker build failed${NC}"
        echo "  Run 'docker build .' to see detailed error"
    fi
else
    echo -e "${YELLOW}⚠ Cannot test Docker build - missing required files${NC}"
fi

echo ""
echo "Summary"
echo "-------"
echo "If any checks failed above, please:"
echo "1. Ensure you're in the Agentainer Lab directory"
echo "2. Run './setup.sh' for automatic setup on fresh VMs"
echo "3. Or follow manual installation steps in README.md"