#!/bin/bash

# Agentainer CLI Installation Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Installing Agentainer CLI..."
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check for required dependencies
echo "Checking dependencies..."

# Check for Go
if ! command_exists go; then
    echo -e "${RED}Error: Go is not installed.${NC}"
    echo "Please install Go 1.21 or higher from https://go.dev/dl/"
    echo ""
    echo "Quick install for Ubuntu/Debian:"
    echo "  wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz"
    echo "  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz"
    echo "  export PATH=\$PATH:/usr/local/go/bin"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
MIN_VERSION="1.21"
if [ "$(printf '%s\n' "$MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$MIN_VERSION" ]; then
    echo -e "${RED}Error: Go version $GO_VERSION is too old. Need Go 1.21 or higher.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go $GO_VERSION found${NC}"

# Check for Docker
if ! command_exists docker; then
    echo -e "${YELLOW}Warning: Docker is not installed.${NC}"
    echo "Docker is required to run agents. Install from https://docs.docker.com/get-docker/"
    echo ""
fi

# Check for Docker Compose
if ! command_exists docker-compose && ! docker compose version >/dev/null 2>&1; then
    echo -e "${YELLOW}Warning: Docker Compose is not installed.${NC}"
    echo "Docker Compose is required for easy Redis setup."
    echo ""
fi

# Create user bin directory if it doesn't exist
mkdir -p "$HOME/bin"

# Build the binary
echo "Building agentainer..."
go build -o agentainer ./cmd/agentainer

# Copy binary to user bin
echo "Installing binary to $HOME/bin..."
cp agentainer "$HOME/bin/"

# Create config directory
echo "Creating configuration directory..."
mkdir -p "$HOME/.agentainer/data"

# Create system-wide config
echo "Creating configuration file..."
cat > "$HOME/.agentainer/config.yaml" << EOF
server:
  host: 127.0.0.1
  port: 8081

redis:
  host: 127.0.0.1
  port: 6379
  password: ""
  db: 0

storage:
  data_dir: ~/.agentainer/data

docker:
  host: unix:///var/run/docker.sock

security:
  default_token: agentainer-default-token
EOF

# Set secure permissions on config file
chmod 600 "$HOME/.agentainer/config.yaml"
chmod 700 "$HOME/.agentainer"

# Add to PATH if not already there
if [[ ":$PATH:" != *":$HOME/bin:"* ]]; then
    echo "Adding $HOME/bin to PATH..."
    echo 'export PATH="$HOME/bin:$PATH"' >> "$HOME/.bashrc"
    echo "Please run 'source ~/.bashrc' or start a new terminal session to use agentainer from anywhere."
else
    echo "PATH already includes $HOME/bin"
fi

echo ""
echo "🚨 ================================================"
echo "⚠️  PROOF-OF-CONCEPT SOFTWARE INSTALLED"
echo "🚨 ================================================"
echo "   WARNING: For local testing only!"
echo "   - Do NOT expose to external networks"
echo "   - Uses default authentication tokens"
echo "   - Minimal security controls"
echo "🚨 ================================================"
echo ""
echo "✅ Agentainer CLI installed successfully!"
echo ""
echo "Usage:"
echo "  agentainer server          # Start the server"
echo "  agentainer deploy --name my-agent --image nginx:latest"
echo "  agentainer list            # List all agents"
echo "  agentainer start <id>      # Start an agent"
echo ""
echo "Note: Make sure Redis is running (docker-compose up -d redis)"