BINARY_NAME=agentainer
DOCKER_IMAGE=agentainer:latest
EXAMPLE_IMAGE=simple-agent:latest

.PHONY: build clean test docker-build docker-run example-build run-redis stop-redis help

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

# Quick install using the install script
quick-install:
	./install.sh

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
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  build-prod   - Build with production optimizations"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose"
	@echo "  example-build - Build example agent image"
	@echo "  run-redis    - Start Redis for local development"
	@echo "  stop-redis   - Stop Redis"
	@echo "  run          - Build and run server locally"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  mod-download - Download dependencies"
	@echo "  mod-update   - Update dependencies"
	@echo "  install      - Install binary to /usr/local/bin (requires sudo)"
	@echo "  install-user - Install binary to ~/bin (no sudo required)"
	@echo "  quick-install- Run install script for easy setup"
	@echo "  uninstall    - Uninstall system installation"
	@echo "  uninstall-user - Uninstall user installation"
	@echo "  help         - Show this help message"