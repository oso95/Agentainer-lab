#!/bin/bash

# Multi-URL Workflow Demo Runner
# Builds all images and runs the workflow in a container
set -e

echo "🚀 Multi-URL Workflow Demo"
echo "=========================="

# Check for API key files
echo ""
echo "📋 Checking API key files..."
if [ ! -f "gpt-workflow-agent/.env" ]; then
    echo "❌ gpt-workflow-agent/.env missing"
    echo "   Create it with: echo 'OPENAI_API_KEY=your-key-here' > gpt-workflow-agent/.env"
    exit 1
fi

if [ ! -f "gemini-workflow-agent/.env" ]; then
    echo "❌ gemini-workflow-agent/.env missing"  
    echo "   Create it with: echo 'GEMINI_API_KEY=your-key-here' > gemini-workflow-agent/.env"
    exit 1
fi

echo "✅ API key files found"

# Check for urls.txt
if [ ! -f "urls.txt" ]; then
    echo ""
    echo "⚠️  urls.txt not found. Creating default file..."
    cat > urls.txt << 'EOF'
# URLs for multi-article analysis
# Add your URLs here, one per line
https://techcrunch.com/2025/08/02/tim-cook-reportedly-tells-employees-apple-must-win-in-ai/
https://techcrunch.com/2025/08/01/more-details-emerge-on-how-windsurfs-vcs-and-founders-got-paid-from-the-google-deal/
https://techcrunch.com/2025/08/01/google-rolls-out-gemini-deep-think-ai-a-reasoning-model-that-tests-multiple-ideas-in-parallel/
EOF
    echo "✅ Created urls.txt with sample URLs"
fi

# Verify Agentainer is running
echo ""
echo "🔍 Checking Agentainer server..."
if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "❌ Agentainer server not running!"
    echo "   Please start it first:"
    echo "   cd ../.. && make run"
    exit 1
fi
echo "✅ Agentainer server is running"

# Build all Docker images
echo ""
echo "🐳 Building Docker images..."
echo "  - Document extractor..."
docker build -q -t doc-extractor:latest doc-extractor
echo "  - GPT workflow agent..."
docker build -q -t gpt-workflow-agent:latest gpt-workflow-agent
echo "  - Gemini workflow agent..."
docker build -q -t gemini-workflow-agent:latest gemini-workflow-agent
echo "  - Workflow runner..."
docker build -q -f Dockerfile.runner -t workflow-runner:latest .
echo "✅ All images built successfully"

# Create output directory
mkdir -p ./results

# Run the workflow in container
echo ""
echo "🔄 Starting workflow execution..."
echo "================================="
docker run --rm \
  -v $(pwd)/results:/output \
  -v $(pwd)/urls.txt:/app/urls.txt:ro \
  --network host \
  workflow-runner:latest

echo ""
echo "✅ Workflow complete!"
echo "📁 Results saved to: $(pwd)/results/"
echo ""
echo "To view results:"
echo "  ls -la results/"