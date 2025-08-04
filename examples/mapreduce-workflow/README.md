# MapReduce Word Counter Example

This example demonstrates how to use Agentainer's MapReduce pattern to process multiple URLs in parallel with automatic retry support, error handling, and resource management.

## Overview

The workflow consists of three phases:
1. **List Phase**: Generate a list of URLs to process (with retry support)
2. **Map Phase**: Process each URL in parallel, counting words (with exponential backoff retry)
3. **Reduce Phase**: Aggregate results from all mappers (handles partial failures)

## Features Demonstrated

- **Automatic Retry**: Failed tasks are automatically retried with configurable backoff
- **Error Resilience**: Continues processing even if some URLs fail
- **Resource Limits**: Each mapper has CPU and memory constraints
- **Agent Cleanup**: Failed agents are kept for debugging (`on_success` policy)
- **Comprehensive Monitoring**: Detailed progress and error reporting

## Agent Cleanup Policy

By default, Agentainer automatically removes agents after their tasks complete to prevent accumulation of stopped containers. You can control this behavior using the `cleanup_policy` in your workflow configuration:

- **`always`** (default): Always remove agents after task completion
- **`on_success`**: Only remove agents if the task succeeded (keep failed agents for debugging)
- **`never`**: Never automatically remove agents (manual cleanup required)

### Interaction with Retry Policies

When a step has a retry policy configured and fails:
1. The agent is **NOT** immediately removed, regardless of cleanup policy
2. The agent is kept for potential retry attempts
3. Only after all retry attempts are exhausted will the cleanup policy be applied

Example:
```yaml
steps:
  - name: process-data
    image: processor:latest
    config:
      retry_policy:
        max_attempts: 3
        backoff: exponential
        delay: 5s
```

In this case, even with `cleanup_policy: always`, a failed agent will be kept until all 3 retry attempts are exhausted.

## Retry Policies

Agentainer supports automatic retry of failed steps with configurable backoff strategies:

### Configuration

Add a `retry_policy` to any step configuration:

```yaml
steps:
  - name: unreliable-service
    image: processor:latest
    config:
      retry_policy:
        max_attempts: 3        # Total attempts (including initial)
        backoff: exponential   # Backoff strategy
        delay: 5s             # Base delay between retries
```

### Backoff Strategies

- **`constant`**: Fixed delay between retries (e.g., 5s, 5s, 5s)
- **`linear`**: Linear increase (e.g., 5s, 10s, 15s)
- **`exponential`**: Exponential increase (e.g., 5s, 10s, 20s)

### How It Works

1. When a step fails, Agentainer checks if a retry policy is configured
2. If retries remain, the agent is kept alive (not cleaned up)
3. After the backoff delay, the agent is restarted for the retry attempt
4. The process repeats until success or max attempts are exhausted
5. Only after all retries fail is the cleanup policy applied

### Example with MapReduce

The included workflow.yaml shows retry configuration for the map step:
- If a URL processing fails, it will retry up to 3 times
- Uses exponential backoff starting at 2 seconds
- Failed agents are kept for retry attempts

## Test URLs

The demo includes various URLs to demonstrate different scenarios:
- **Normal URLs**: example.com, wikipedia.org - Process successfully
- **Timeout URL**: `/delay/3` - May timeout on first attempt
- **Server Errors**: `/status/500` - Fails initially, may succeed on retry
- **Rate Limited**: `/status/429` - Triggers retry with backoff
- **Various Content**: HTML, JSON, plain text, base64 encoded

This mix demonstrates how the retry mechanism handles different failure types.

## Prerequisites

1. Docker Desktop installed and running
2. Agentainer built and server running
3. Python 3.x (uses only standard library modules)

## Providing URLs

Create a `urls.txt` file with URLs to analyze (one per line):

```text
# URLs to process - one per line
# Lines starting with # are ignored

https://example.com
https://www.wikipedia.org
https://www.python.org

# Test various content types
https://httpbin.org/html
https://httpbin.org/json

# Add your own URLs below:
https://your-site.com
```

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

### Prerequisites

1. **Start Docker Desktop** - Required for running containers
2. **Start Agentainer Server** (from repository root):
   ```bash
   # Build Agentainer if not already built
   go build -o agentainer ./cmd/agentainer/
   
   # Start the server
   ./agentainer server
   ```
3. **Build the Docker images**:
   ```bash
   ./build.sh
   ```

### Method 1: Using the Python API Script (Recommended)

1. **Create your URLs file** (or use the provided example):
   ```bash
   # Edit urls.txt to add your URLs (one per line)
   nano urls.txt
   ```

2. **Run the workflow**:
   ```bash
   # Run with default urls.txt
   python3 run_workflow_api.py
   
   # Specify a different URLs file
   python3 run_workflow_api.py --urls myurls.txt
   
   # Specify output directory
   python3 run_workflow_api.py --output my_results
   
   # Skip additional export formats
   python3 run_workflow_api.py --no-export
   ```

This script:
- Loads URLs from `urls.txt` (one URL per line)
- Uses Agentainer's HTTP API directly (no SDK required)
- Creates the workflow configuration programmatically
- Monitors progress in real-time
- Saves detailed results and reports
- Shows retry attempts and failures

### Method 2: Using Agentainer CLI Directly

```bash
# Start Agentainer server (if not already running)
agentainer server

# Create workflow from YAML file
agentainer workflow create -f workflow.yaml

# Or create workflow manually
agentainer workflow create \
  --name mapreduce-word-counter \
  --steps '[
    {
      "name": "list-urls",
      "image": "mapreduce-mapper:latest",
      "env": {"STEP_TYPE": "list"}
    },
    {
      "name": "process-urls",
      "type": "map",
      "map_config": {"items_from": "urls", "max_parallel": 5},
      "image": "mapreduce-mapper:latest",
      "env": {"STEP_TYPE": "map"}
    },
    {
      "name": "aggregate-results",
      "image": "mapreduce-reducer:latest",
      "depends_on": ["process-urls"]
    }
  ]'
```

### Method 4: Using the API

```bash
# Create workflow from configuration
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Authorization: Bearer ${AGENTAINER_API_KEY}" \
  -H "Content-Type: application/json" \
  -d @workflow.yaml
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

## Exported Results

The workflow exports comprehensive results to a timestamped directory:

### Core Results
- `generated_urls.json` - List of URLs that were processed
- `map_results.json` - Detailed word count results from each URL
- `map_errors.json` - Information about failed URL processing
- `final_summary.json` - Aggregated statistics and analysis
- `FINAL_REPORT.md` - Human-readable summary report

### Individual URL Results
- `wordcount_<url>.json` - Word count details for each successful URL
  - Word count
  - Unique words
  - Top words
  - Response time

### Additional Exports (automatic)
The workflow automatically exports these additional formats:
- `word_frequencies.csv` - Word frequency data in CSV format
- `url_performance.csv` - Performance metrics for each URL
- `DETAILED_ANALYSIS.md` - Comprehensive analysis with statistics

To skip automatic export of additional formats, use:
```bash
python3 run_workflow_api.py --no-export
```

## Customization

You can modify the mapper to:
- Process different types of data (files, API endpoints, etc.)
- Perform different computations (image processing, data transformation, etc.)
- Handle more complex aggregation logic

The pattern remains the same: list → parallel map → reduce.