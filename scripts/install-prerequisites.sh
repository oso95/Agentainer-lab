#!/bin/bash

# Agentainer Lab - Prerequisites Installation Script
# This script installs all prerequisites on fresh VMs
# Extracted from the original setup.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS
OS=""
DISTRO=""
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
fi

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}     Installing Prerequisites for Agentainer    ${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo "Detected OS: $OS $DISTRO"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install packages based on distro
install_package() {
    local package=$1
    echo -e "${YELLOW}Installing $package...${NC}"
    
    if [[ "$DISTRO" == "ubuntu" ]] || [[ "$DISTRO" == "debian" ]]; then
        sudo apt-get update -qq
        sudo apt-get install -y $package
    elif [[ "$DISTRO" == "centos" ]] || [[ "$DISTRO" == "rhel" ]] || [[ "$DISTRO" == "fedora" ]]; then
        sudo yum install -y $package
    elif [[ "$OS" == "macos" ]]; then
        if command_exists brew; then
            brew install $package
        else
            echo -e "${RED}Homebrew not found. Please install from https://brew.sh${NC}"
            exit 1
        fi
    else
        echo -e "${RED}Unsupported distribution. Please install $package manually.${NC}"
        exit 1
    fi
}

# Install Git if needed
if ! command_exists git; then
    echo -e "${YELLOW}Git not found. Installing...${NC}"
    install_package git
else
    echo -e "${GREEN}✓ Git is already installed${NC}"
fi

# Install Go if needed
if ! command_exists go; then
    echo -e "${YELLOW}Go not found. Installing Go 1.21...${NC}"
    
    GO_VERSION="1.21.0"
    ARCH=$(uname -m)
    
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "aarch64" ]]; then
        ARCH="arm64"
    fi
    
    if [[ "$OS" == "linux" ]]; then
        GO_TAR="go${GO_VERSION}.linux-${ARCH}.tar.gz"
        wget -q "https://go.dev/dl/${GO_TAR}"
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "${GO_TAR}"
        rm "${GO_TAR}"
        
        # Add to PATH
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        export PATH=$PATH:/usr/local/go/bin
        
    elif [[ "$OS" == "macos" ]]; then
        install_package go
    fi
else
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    MIN_VERSION="1.21"
    if [ "$(printf '%s\n' "$MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$MIN_VERSION" ]; then
        echo -e "${RED}Error: Go version $GO_VERSION is too old. Need Go 1.21 or higher.${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Go $GO_VERSION is already installed${NC}"
fi

# Install Docker if needed
if ! command_exists docker; then
    echo -e "${YELLOW}Docker not found. Installing...${NC}"
    
    if [[ "$DISTRO" == "ubuntu" ]] || [[ "$DISTRO" == "debian" ]]; then
        # Install Docker on Ubuntu/Debian
        sudo apt-get update
        sudo apt-get install -y ca-certificates curl gnupg
        
        # Add Docker's official GPG key
        sudo install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/$DISTRO/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
        
        # Add repository
        echo \
          "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$DISTRO \
          $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
          sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        
        # Install Docker
        sudo apt-get update
        sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
        
    elif [[ "$OS" == "macos" ]]; then
        echo -e "${YELLOW}Please install Docker Desktop from: https://www.docker.com/products/docker-desktop${NC}"
        echo "After installation, ensure Docker Desktop is running and try again."
        exit 1
    else
        echo -e "${RED}Please install Docker manually from: https://docs.docker.com/get-docker/${NC}"
        exit 1
    fi
    
    # Add user to docker group (Linux only)
    if [[ "$OS" == "linux" ]]; then
        sudo usermod -aG docker $USER
        echo -e "${YELLOW}Added $USER to docker group. You may need to log out and back in.${NC}"
    fi
else
    echo -e "${GREEN}✓ Docker is already installed${NC}"
fi

# Install Docker Compose if needed
if ! command_exists docker-compose && ! docker compose version >/dev/null 2>&1; then
    echo -e "${YELLOW}Docker Compose not found. Installing...${NC}"
    
    if [[ "$OS" == "linux" ]]; then
        # Install Docker Compose V2 as Docker plugin (already done above with docker-compose-plugin)
        # Also install standalone for compatibility
        COMPOSE_VERSION="v2.20.2"
        sudo curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
    fi
else
    echo -e "${GREEN}✓ Docker Compose is already installed${NC}"
fi

# Start Docker service (Linux only, skip for WSL)
if [[ "$OS" == "linux" ]]; then
    # Check if we're in WSL
    if grep -qi microsoft /proc/version 2>/dev/null; then
        echo -e "${GREEN}✓ Running in WSL - Docker Desktop manages the Docker daemon${NC}"
    else
        # Check if docker service exists
        if systemctl list-unit-files | grep -q "docker.service"; then
            if ! sudo systemctl is-active --quiet docker; then
                echo -e "${YELLOW}Starting Docker service...${NC}"
                sudo systemctl start docker
                sudo systemctl enable docker
            else
                echo -e "${GREEN}✓ Docker service is running${NC}"
            fi
        else
            echo -e "${GREEN}✓ Docker is running (managed externally)${NC}"
        fi
    fi
fi

echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}     Prerequisites Installation Complete!       ${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo "Next steps:"
echo "1. If you were added to the docker group, log out and back in"
echo "2. Run: source ~/.bashrc"
echo "3. Continue with: make install-user"
echo ""