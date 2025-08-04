#!/bin/bash

# MapReduce Word Counter Runner
# Builds all images and runs the workflow in a container
set -e

echo "🔢 MapReduce Word Counter Workflow"
echo "=================================="

# Check for urls.txt
if [ ! -f "urls.txt" ]; then
    echo ""
    echo "⚠️  urls.txt not found. Creating default file..."
    cat > urls.txt << 'EOF'
# URLs to process - one per line
# Lines starting with # are ignored

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

# Verify Redis is accessible
echo ""
echo "🔍 Checking Redis..."
if ! redis-cli ping > /dev/null 2>&1; then
    if ! docker run --rm --network host redis:alpine redis-cli -h localhost ping > /dev/null 2>&1; then
        echo "❌ Redis not accessible!"
        echo "   Please start Redis:"
        echo "   docker run -d -p 6379:6379 redis:latest"
        exit 1
    fi
fi
echo "✅ Redis is accessible"

# Build all Docker images
echo ""
echo "🐳 Building Docker images..."
echo "  - Mapper image..."
docker build -q -f Dockerfile.mapper -t mapreduce-mapper:latest .
echo "  - Reducer image..."
docker build -q -f Dockerfile.reducer -t mapreduce-reducer:latest .
echo "  - Workflow runner..."
docker build -q -f Dockerfile.runner -t mapreduce-runner:latest .
echo "✅ All images built successfully"

# Create output directory
mkdir -p ./results

# Run the workflow in container
echo ""
echo "🔄 Starting MapReduce workflow..."
echo "================================="
echo ""
echo "This workflow will:"
echo "  📋 Process $(grep -v '^#' urls.txt | grep -v '^$' | wc -l) URLs"
echo "  🔄 Retry failed tasks automatically"
echo "  📊 Generate comprehensive reports"
echo ""

docker run --rm \
  -v $(pwd)/results:/output \
  -v $(pwd)/urls.txt:/app/urls.txt:ro \
  --network host \
  mapreduce-runner:latest "$@"

echo ""
echo "✅ Workflow complete!"
echo "📁 Results saved to: $(pwd)/results/"
echo ""
echo "To view results:"
echo "  cat results/*/FINAL_REPORT.md"