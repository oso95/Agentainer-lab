BINARY_NAME=agentainer
DOCKER_IMAGE=agentainer:latest
EXAMPLE_IMAGE=simple-agent:latest

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: all build clean test docker-build docker-run example-build run-redis stop-redis help \
        install install-user uninstall uninstall-user verify setup install-prerequisites

# Default target
all: help

# Build the application
build:
	go build -o $(BINARY_NAME) ./cmd/agentainer

# Build with optimizations for production
build-prod:
	go build -ldflags="-w -s" -o $(BINARY_NAME) ./cmd/agentainer

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# Build example agent image
example-build:
	cd examples/simple-agent && docker build -t $(EXAMPLE_IMAGE) .

# Start Redis for local development
run-redis:
	docker run -d --name agentainer-redis -p 6379:6379 redis:7-alpine

# Stop Redis
stop-redis:
	docker stop agentainer-redis && docker rm agentainer-redis

# Run the server locally
run: build run-redis
	./$(BINARY_NAME) server

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Initialize go modules
mod-init:
	go mod init github.com/agentainer/agentainer-lab

# Download dependencies
mod-download:
	go mod download

# Update dependencies
mod-update:
	go get -u ./...
	go mod tidy

# Install the binary system-wide (requires sudo)
install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Install the binary for current user (no sudo required)
install-user: build
	mkdir -p $$HOME/bin $$HOME/.agentainer/data
	cp $(BINARY_NAME) $$HOME/bin/
	@if [ ! -f $$HOME/.agentainer/config.yaml ]; then \
		cp config.yaml $$HOME/.agentainer/; \
		sed -i 's|data_dir: ./data|data_dir: ~/.agentainer/data|' $$HOME/.agentainer/config.yaml; \
	fi
	@echo "Binary installed to $$HOME/bin/"
	@echo "Add $$HOME/bin to your PATH if not already done:"
	@echo "  echo 'export PATH=\"\$$HOME/bin:\$$PATH\"' >> ~/.bashrc"
	@echo "  source ~/.bashrc"

# Install prerequisites on fresh VM (requires sudo)
install-prerequisites:
	@echo "$(BLUE)Installing prerequisites for fresh VM...$(NC)"
	@chmod +x scripts/install-prerequisites.sh
	@./scripts/install-prerequisites.sh

# Complete setup for fresh VM (prerequisites + install)
setup: install-prerequisites install-user
	@echo "$(GREEN)Complete setup finished!$(NC)"
	@echo "Run 'source ~/.bashrc' to update PATH"

# Verify installation
verify:
	@echo "$(BLUE)Verifying Agentainer setup...$(NC)"
	@echo "===================================="
	@echo ""
	@echo "Checking prerequisites..."
	@echo "------------------------"
	@command -v git >/dev/null 2>&1 && echo "$(GREEN)✓ git is installed$(NC)" || echo "$(RED)✗ git is NOT installed$(NC)"
	@command -v go >/dev/null 2>&1 && echo "$(GREEN)✓ go is installed$(NC)" || echo "$(RED)✗ go is NOT installed$(NC)"
	@command -v docker >/dev/null 2>&1 && echo "$(GREEN)✓ docker is installed$(NC)" || echo "$(RED)✗ docker is NOT installed$(NC)"
	@(command -v docker-compose >/dev/null 2>&1 || docker compose version >/dev/null 2>&1) && echo "$(GREEN)✓ docker-compose is installed$(NC)" || echo "$(RED)✗ docker-compose is NOT installed$(NC)"
	@if command -v go >/dev/null 2>&1; then \
		GO_VERSION=$$(go version | awk '{print $$3}' | sed 's/go//'); \
		echo "  Go version: $$GO_VERSION"; \
	fi
	@if command -v docker >/dev/null 2>&1; then \
		if docker ps >/dev/null 2>&1; then \
			echo "$(GREEN)✓ Docker daemon is running$(NC)"; \
		else \
			echo "$(RED)✗ Docker daemon is NOT running or not accessible$(NC)"; \
		fi \
	fi
	@echo ""
	@echo "Checking installation..."
	@echo "------------------------"
	@[ -f "$$HOME/bin/$(BINARY_NAME)" ] && echo "$(GREEN)✓ Binary installed at ~/bin/$(BINARY_NAME)$(NC)" || echo "$(RED)✗ Binary not found in ~/bin$(NC)"
	@[ -d "$$HOME/.agentainer" ] && echo "$(GREEN)✓ Config directory exists$(NC)" || echo "$(RED)✗ Config directory missing$(NC)"
	@[ -f "$$HOME/.agentainer/config.yaml" ] && echo "$(GREEN)✓ Config file exists$(NC)" || echo "$(RED)✗ Config file missing$(NC)"
	@command -v $(BINARY_NAME) >/dev/null 2>&1 && echo "$(GREEN)✓ $(BINARY_NAME) command available$(NC)" || echo "$(YELLOW)⚠ $(BINARY_NAME) command not in PATH$(NC)"

# Uninstall user installation
uninstall-user:
	rm -f $$HOME/bin/$(BINARY_NAME)
	rm -rf $$HOME/.agentainer
	@echo "Agentainer uninstalled from user directory"

# Uninstall system installation
uninstall:
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Agentainer uninstalled from system"

# Show help
help:
	@echo "$(BLUE)Agentainer Lab - Makefile Commands$(NC)"
	@echo "$(BLUE)====================================$(NC)"
	@echo ""
	@echo "$(YELLOW)Quick Start (for fresh VMs):$(NC)"
	@echo "  $(GREEN)make setup$(NC)          - Complete setup (prerequisites + install)"
	@echo "  $(GREEN)make verify$(NC)         - Verify installation"
	@echo ""
	@echo "$(YELLOW)Installation:$(NC)"
	@echo "  $(GREEN)make install-prerequisites$(NC) - Install Git, Go, Docker (fresh VMs)"
	@echo "  $(GREEN)make install-user$(NC)   - Install binary to ~/bin (recommended)"
	@echo "  $(GREEN)make install$(NC)        - Install binary to /usr/local/bin (requires sudo)"
	@echo "  $(GREEN)make uninstall-user$(NC) - Remove user installation"
	@echo "  $(GREEN)make uninstall$(NC)      - Remove system installation"
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@echo "  $(GREEN)make build$(NC)          - Build the application"
	@echo "  $(GREEN)make build-prod$(NC)     - Build with production optimizations"
	@echo "  $(GREEN)make test$(NC)           - Run tests"
	@echo "  $(GREEN)make clean$(NC)          - Clean build artifacts"
	@echo "  $(GREEN)make fmt$(NC)            - Format code"
	@echo "  $(GREEN)make lint$(NC)           - Run linter"
	@echo ""
	@echo "$(YELLOW)Docker:$(NC)"
	@echo "  $(GREEN)make docker-build$(NC)   - Build Docker image"
	@echo "  $(GREEN)make docker-run$(NC)     - Run with Docker Compose"
	@echo "  $(GREEN)make docker-stop$(NC)    - Stop Docker Compose"
	@echo "  $(GREEN)make example-build$(NC)  - Build example agent image"
	@echo ""
	@echo "$(YELLOW)Local Development:$(NC)"
	@echo "  $(GREEN)make run$(NC)            - Build and run server locally"
	@echo "  $(GREEN)make run-redis$(NC)      - Start Redis container"
	@echo "  $(GREEN)make stop-redis$(NC)     - Stop Redis container"
	@echo ""
	@echo "$(YELLOW)Dependencies:$(NC)"
	@echo "  $(GREEN)make mod-download$(NC)   - Download Go dependencies"
	@echo "  $(GREEN)make mod-update$(NC)     - Update Go dependencies"
	@echo ""
	@echo "$(YELLOW)Help:$(NC)"
	@echo "  $(GREEN)make help$(NC)           - Show this help message"
	@echo ""
	@echo "$(BLUE)For more information, see README.md$(NC)"