#!/bin/bash

# Setup script for Multi-URL Workflow Demo
set -e

echo "🔧 Setting up Multi-URL Workflow Demo"
echo "======================================="

# Check for API key files
echo ""
echo "📋 Checking API key files..."
if [ ! -f "gpt-workflow-agent/.env" ]; then
    echo "❌ gpt-workflow-agent/.env missing"
    echo "   Create it with: OPENAI_API_KEY=your-key-here"
    exit 1
fi

if [ ! -f "gemini-workflow-agent/.env" ]; then
    echo "❌ gemini-workflow-agent/.env missing"  
    echo "   Create it with: GEMINI_API_KEY=your-key-here"
    exit 1
fi

echo "✅ API key files found"

# Copy urls.txt to doc-extractor for Docker build
if [ -f "urls.txt" ]; then
    cp urls.txt doc-extractor/
    echo "✅ Copied urls.txt to doc-extractor/"
else
    echo "⚠️  urls.txt not found in workflow-demo directory"
    echo "   Creating default urls.txt..."
    cat > urls.txt << 'EOF'
# URLs for multi-article analysis
# Add your URLs here, one per line
https://example.com/article1
https://example.com/article2
EOF
    cp urls.txt doc-extractor/
fi

# Build Docker images
echo ""
echo "🐳 Building Docker images..."
docker build -t doc-extractor:latest doc-extractor
docker build -t gpt-workflow-agent:latest gpt-workflow-agent
docker build -t gemini-workflow-agent:latest gemini-workflow-agent

# Verify Agentainer is running
echo ""
echo "🔍 Checking Agentainer..."
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✅ Agentainer is running"
else
    echo "⚠️  Agentainer not running. Start with: cd ../.. && make run"
fi

echo ""
echo "✅ Setup complete! Run with: python3 run_workflow.py"