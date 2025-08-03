#!/bin/bash

echo "Building MapReduce example images..."

# Build mapper image
echo "Building mapper image..."
docker build -f Dockerfile.mapper -t mapreduce-mapper:latest .
if [ $? -ne 0 ]; then
    echo "Failed to build mapper image"
    exit 1
fi

# Build reducer image
echo "Building reducer image..."
docker build -f Dockerfile.reducer -t mapreduce-reducer:latest .
if [ $? -ne 0 ]; then
    echo "Failed to build reducer image"
    exit 1
fi

echo "Successfully built both images!"
echo ""
echo "To run the workflow:"
echo "  agentainer workflow mapreduce --name word-counter --mapper mapreduce-mapper:latest --reducer mapreduce-reducer:latest --parallel 5"