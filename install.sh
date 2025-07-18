#!/bin/bash

# Agentainer CLI Installation Script

set -e

echo "Installing Agentainer CLI..."

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