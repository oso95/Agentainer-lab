# MapReduce Word Counter Example

This example demonstrates how to use Agentainer Flow's MapReduce pattern to process multiple URLs in parallel and count words.

## Overview

The workflow consists of three phases:
1. **List Phase**: Generate a list of URLs to process
2. **Map Phase**: Process each URL in parallel, counting words
3. **Reduce Phase**: Aggregate results from all mappers

## Building the Images

```bash
# Build mapper image
docker build -f Dockerfile.mapper -t mapreduce-mapper:latest .

# Build reducer image  
docker build -f Dockerfile.reducer -t mapreduce-reducer:latest .
```

Or use the provided build script:
```bash
./build.sh
```

## Running the Workflow

### Using Agentainer CLI

```bash
# Start Agentainer server (if not already running)
agentainer server

# Create and run the MapReduce workflow
agentainer workflow mapreduce \
  --name word-counter \
  --mapper mapreduce-mapper:latest \
  --reducer mapreduce-reducer:latest \
  --parallel 5 \
  --pool-size 3
```

### Using the API

```bash
curl -X POST http://localhost:8081/workflows/mapreduce \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "word-counter",
    "mapper_image": "mapreduce-mapper:latest",
    "reducer_image": "mapreduce-reducer:latest",
    "max_parallel": 5,
    "pool_size": 3
  }'
```

## Monitoring Progress

```bash
# Get workflow status
agentainer workflow get <workflow-id>

# List all jobs in the workflow
agentainer workflow jobs <workflow-id>

# View logs from a specific agent
agentainer logs <agent-id>
```

## How It Works

1. **List Phase**: The mapper with `STEP_TYPE=list` generates a list of URLs and stores them in workflow state

2. **Map Phase**: Multiple mapper instances (with `STEP_TYPE=map`) run in parallel:
   - Each mapper processes one URL
   - Counts words and finds most common words
   - Stores results in workflow state

3. **Reduce Phase**: The reducer aggregates all results:
   - Calculates total words across all URLs
   - Finds the most common words overall
   - Generates a final summary

## Workflow State

The workflow uses Redis-backed state to share data between steps:
- `urls`: List of URLs to process
- `map_results`: Results from each mapper
- `map_errors`: Any errors encountered
- `final_summary`: Aggregated results

## Performance Benefits

With agent pooling enabled:
- Mappers start in ~0.1s instead of 2-5s
- 5 URLs can be processed by 3 pooled agents
- 20-50x performance improvement for parallel tasks

## Customization

You can modify the mapper to:
- Process different types of data (files, API endpoints, etc.)
- Perform different computations (image processing, data transformation, etc.)
- Handle more complex aggregation logic

The pattern remains the same: list → parallel map → reduce.