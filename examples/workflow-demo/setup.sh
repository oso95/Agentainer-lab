#!/bin/bash

echo "🔧 Setting up LLM Workflow Demo"
echo "==============================="
echo ""

# Detect platform
PLATFORM="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    PLATFORM="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    PLATFORM="macos"
elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
    PLATFORM="windows"
fi

print_info "Detected platform: $PLATFORM"

# Function to display colored output
print_error() {
    echo -e "\033[0;31m❌ $1\033[0m"
}

print_warning() {
    echo -e "\033[1;33m⚠️  $1\033[0m"
}

print_success() {
    echo -e "\033[0;32m✅ $1\033[0m"
}

print_info() {
    echo -e "\033[0;34mℹ️  $1\033[0m"
}

# Step 1: Check for .env files
echo "📋 Checking environment files..."

ENV_SETUP_NEEDED=false

if [ ! -f "gpt-workflow-agent/.env" ]; then
    if [ -f "gpt-workflow-agent/.env.example" ]; then
        cp gpt-workflow-agent/.env.example gpt-workflow-agent/.env
        print_warning "Created gpt-workflow-agent/.env from example"
        ENV_SETUP_NEEDED=true
    else
        print_error "gpt-workflow-agent/.env.example not found!"
        exit 1
    fi
else
    print_success "gpt-workflow-agent/.env exists"
fi

if [ ! -f "gemini-workflow-agent/.env" ]; then
    if [ -f "gemini-workflow-agent/.env.example" ]; then
        cp gemini-workflow-agent/.env.example gemini-workflow-agent/.env
        print_warning "Created gemini-workflow-agent/.env from example"
        ENV_SETUP_NEEDED=true
    else
        print_error "gemini-workflow-agent/.env.example not found!"
        exit 1
    fi
else
    print_success "gemini-workflow-agent/.env exists"
fi

# Check if API keys are configured
if grep -q "your-openai-api-key" gpt-workflow-agent/.env 2>/dev/null || ! grep -q "OPENAI_API_KEY=" gpt-workflow-agent/.env 2>/dev/null; then
    print_warning "OpenAI API key not configured in gpt-workflow-agent/.env"
    ENV_SETUP_NEEDED=true
fi

if grep -q "your-gemini-api-key" gemini-workflow-agent/.env 2>/dev/null || ! grep -q "GEMINI_API_KEY=" gemini-workflow-agent/.env 2>/dev/null; then
    print_warning "Gemini API key not configured in gemini-workflow-agent/.env"
    ENV_SETUP_NEEDED=true
fi

if [ "$ENV_SETUP_NEEDED" = true ]; then
    echo ""
    print_warning "Please configure your API keys:"
    echo "  1. Edit gpt-workflow-agent/.env and add your OpenAI API key"
    echo "  2. Edit gemini-workflow-agent/.env and add your Gemini API key"
    echo ""
    echo "After configuring, run this script again."
    exit 1
fi

# Step 2: Build Docker images
echo ""
echo "🐳 Building Docker images..."

# Function to build with progress
build_image() {
    local name=$1
    local path=$2
    echo "Building $name..."
    if docker build -t "$name" "$path" > /tmp/docker_build_$$.log 2>&1; then
        print_success "$name built successfully"
        return 0
    else
        print_error "Failed to build $name"
        echo "Error log:"
        tail -20 /tmp/docker_build_$$.log
        rm -f /tmp/docker_build_$$.log
        return 1
    fi
}

# Build all images
BUILD_FAILED=false

if ! build_image "gpt-workflow-agent:latest" "./gpt-workflow-agent/"; then
    BUILD_FAILED=true
fi

if ! build_image "gemini-workflow-agent:latest" "./gemini-workflow-agent/"; then
    BUILD_FAILED=true
fi

if ! build_image "doc-extractor:latest" "./doc-extractor/"; then
    BUILD_FAILED=true
fi

# Clean up temp files
rm -f /tmp/docker_build_$$.log

if [ "$BUILD_FAILED" = true ]; then
    echo ""
    print_error "Some images failed to build. Please check the errors above."
    exit 1
fi

# Step 3: Verify setup
echo ""
echo "🔍 Verifying setup..."

# Check if Agentainer is running
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    print_success "Agentainer server is running"
else
    print_warning "Agentainer server is not running"
    echo "  Start it with: cd ../.. && make run"
    if [ "$PLATFORM" = "linux" ]; then
        print_info "Linux users: The unified startup ensures Redis connectivity works correctly"
    fi
fi

# Check Docker images
IMAGES_OK=true
for image in "gpt-workflow-agent:latest" "gemini-workflow-agent:latest" "doc-extractor:latest"; do
    if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${image}$"; then
        print_success "Image $image exists"
    else
        print_error "Image $image not found"
        IMAGES_OK=false
    fi
done

# Final summary
echo ""
echo "==============================="
if [ "$IMAGES_OK" = true ]; then
    print_success "Setup complete! 🎉"
    echo ""
    echo "📚 Next steps:"
    echo "  1. Ensure Agentainer is running: cd ../.. && make run"
    echo "  2. Run the workflow: python3 run_workflow.py"
    echo "  3. Results will be saved to a timestamped folder"
else
    print_error "Setup incomplete. Please fix the issues above."
    exit 1
fi