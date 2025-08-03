# Agentainer Workflow Demo: Multi-URL Analysis with Map-Reduce

This example demonstrates Agentainer's powerful workflow orchestration capabilities, showcasing parallel processing with Map-Reduce patterns for analyzing multiple web articles simultaneously.

## Overview

This workflow demonstrates key Agentainer features:
- 🔄 **Sequential Processing**: Step-by-step execution with dependencies
- 🗺️ **Map Phase**: Parallel processing of multiple URLs
- 🔀 **Reduce Phase**: Aggregation of results from parallel tasks
- 🤖 **Multi-Agent Orchestration**: Coordinating different AI agents (GPT, Gemini)
- 📊 **State Management**: Sharing data between workflow steps via Redis
- 💾 **File Output**: Saving all results locally for further analysis

## Key Agentainer Concepts Demonstrated

### 1. Workflow Steps and Dependencies
Workflows consist of steps that can depend on each other:
```json
{
  "id": "process",
  "type": "map",
  "depends_on": ["prepare"],
  "config": {
    "map_over": "urls"  // Creates parallel tasks
  }
}
```

### 2. Step Types
- **Sequential**: Runs once, processes data linearly
- **Map**: Creates multiple parallel tasks from input data
- **Reduce**: Aggregates results from map phase

### 3. Dynamic Task Creation
The `map_over` field dynamically creates tasks:
- Input: `["url1", "url2", "url3"]`
- Result: 3 parallel agents processing each URL

## Prerequisites

1. **Agentainer Server**: Make sure Agentainer server is running
   ```bash
   # From the root directory
   make run
   ```

2. **API Keys**: Set up your API keys in the agent `.env` files:
   - `gpt-workflow-agent/.env` - Add your OpenAI API key
   - `gemini-workflow-agent/.env` - Add your Google API key

3. **Docker**: Ensure Docker is installed and running

## Setup

Before running workflows, set up the environment and build Docker images:

```bash
./setup.sh
```

This script will:
1. Check and create `.env` files from examples if needed
2. Verify API keys are configured
3. Build all Docker images:
   - `doc-extractor:latest` - Web content extraction agent
   - `gpt-workflow-agent:latest` - GPT-based analysis agent
   - `gemini-workflow-agent:latest` - Gemini-based analysis agent
4. Verify Agentainer server is running

## Running the Workflow

### Basic Usage

```bash
# Run the multi-URL analysis workflow
python3 run_workflow.py
```

The workflow will:
1. Load URLs from `urls.txt` (edit this file with your URLs)
2. Create a workflow with multiple processing steps
3. Launch parallel agents to analyze each URL
4. Aggregate results and generate cross-article insights
5. Save all results to a timestamped directory

### Adding URLs to Analyze

Edit `urls.txt` to add the URLs you want to analyze:
```txt
# One URL per line
https://www.example.com/article1
https://www.example.com/article2
# Add up to 10 URLs
```

### Workflow Execution Flow

1. **Prepare Phase** (Sequential):
   - Validates and prepares URLs for processing
   - Stores URL list in workflow state
   
2. **Process Phase** (Map - Parallel):
   - Agentainer creates one agent per URL
   - Each agent fetches and analyzes its assigned URL
   - Results stored in Redis for aggregation
   
3. **Aggregate Phase** (Reduce):
   - Combines results from all URL processors
   - Calculates statistics (success rate, total words, etc.)
   
4. **Entity Extraction** (Sequential):
   - AI agent analyzes aggregated content
   - Extracts entities across all articles
   
5. **Insights Generation** (Sequential):
   - Identifies patterns and trends across articles
   - Creates cross-article insights
   
6. **Report Generation** (Sequential):
   - Produces comprehensive analysis report
   - Includes findings from all previous steps

### Output Files

Results are saved to a timestamped folder (e.g., `analysis_results_20240115_143022`):

- **workflow_config.json** - Complete workflow configuration
- **workflow_metadata.json** - Workflow ID and URL information
- **aggregated_results.json** - Combined results from all URLs
- **processing_summary.txt** - Statistics about URL processing
- **entities_all_articles.json** - Entities extracted from all articles
- **cross_article_insights.json** - AI-generated insights
- **insights.txt** - Human-readable insights
- **final_report.md** - Comprehensive analysis report
- **ANALYSIS_SUMMARY.md** - Workflow execution summary

## Example Output

### Entities Report
```markdown
# Extracted Entities

## People
- Geoffrey Hinton
- Yann LeCun
- Andrew Ng

## Organizations
- OpenAI
- Google DeepMind
- MIT

## Technologies
- Neural Networks
- Deep Learning
- Transformers
```

### Insights
```
Key Insights:
1. The article emphasizes the rapid advancement of AI in recent years
2. There's a strong focus on ethical considerations and safety
3. Multiple organizations are competing in the AI space
4. Regulatory frameworks are still catching up with technology
```

## Understanding the Workflow Implementation

### Core Workflow Configuration (from `run_workflow.py`)

```python
workflow_config = {
    "name": "multi-url-analysis",
    "steps": [
        {
            "id": "prepare",
            "type": "sequential",  # Runs once
            "config": {
                "image": "doc-extractor:latest",
                "input": {"urls": urls}
            }
        },
        {
            "id": "process",
            "type": "map",  # Creates parallel tasks
            "depends_on": ["prepare"],
            "config": {
                "map_over": "urls"  # KEY: One task per URL
            }
        },
        {
            "id": "aggregate",
            "type": "reduce",  # Combines all results
            "depends_on": ["process"]
        }
    ]
}
```

### How Map-Reduce Works in Agentainer

1. **Map Phase**:
   - `map_over: "urls"` tells Agentainer to create one task per URL
   - Each task gets one URL as input: `{"url": "...", "url_id": "..."}`
   - Tasks run in parallel (up to `max_parallel` limit)

2. **Reduce Phase**:
   - Waits for all map tasks to complete
   - Aggregates results from workflow state
   - Produces combined output

### State Management

Agents share data through Redis-backed workflow state:
```python
# Writing to state (in agent)
set_workflow_state(redis_client, "result_url_0", analysis)

# Reading from state (in reducer)
result = get_workflow_state(redis_client, "result_url_0")
```

## Configuration

### Environment Variables

- `AGENTAINER_API_URL` - Agentainer API endpoint (default: `http://localhost:8081`)
- `AGENTAINER_AUTH_TOKEN` - Authentication token (default: `agentainer-default-token`)
- `OPENAI_API_KEY` - Required for GPT agent (set in `gpt-workflow-agent/.env`)
- `GOOGLE_API_KEY` - Required for Gemini agent (set in `gemini-workflow-agent/.env`)

### Redis Connection

Agents connect to Redis using the host automatically configured by Agentainer:
- **Docker Compose deployment**: `redis:6379` (all platforms)
- **Note**: The unified startup approach (`make run`) ensures Redis connectivity works correctly on all platforms (macOS, Linux, Windows WSL)

## Building Complex Workflows: A Developer Guide

### 1. Defining Workflow Steps

Each step in your workflow should have:
```python
{
    "id": "unique_step_id",
    "name": "Human-readable name",
    "type": "sequential|map|reduce",
    "depends_on": ["previous_step_id"],  # Optional
    "config": {
        "image": "your-agent:latest",
        "command": ["python", "app.py"],
        "env": {"TASK_TYPE": "your_task"},
        "input": {},  # Data for this step
        "map_over": "field_name"  # For map steps only
    }
}
```

### 2. Agent Implementation Pattern

Agents should follow this pattern:
```python
# 1. Get task type from environment
TASK_TYPE = os.environ.get('TASK_TYPE')

# 2. Route to appropriate handler
if TASK_TYPE == "prepare":
    # Prepare data for map phase
    result = prepare_data(task_input)
elif TASK_TYPE == "process":
    # Process individual item (map)
    result = process_item(task_input)
elif TASK_TYPE == "aggregate":
    # Combine all results (reduce)
    result = aggregate_results()
```

### 3. Passing Data Between Steps

**Via Input Field** (for initial data):
```python
"input": {
    "urls": ["url1", "url2"],
    "config": {"timeout": 30}
}
```

**Via Workflow State** (for intermediate data):
```python
# Write in one step
set_workflow_state(redis, "processed_data", data)

# Read in another step
data = get_workflow_state(redis, "processed_data")
```

### 4. Error Handling

```python
"config": {
    "failure_strategy": "continue_on_partial",
    "max_retries": 3,
    "timeout": "5m"
}
```

### 5. Monitoring and Debugging

- Check workflow status: View the dashboard at http://localhost:8080
- Monitor Redis state: Use `redis-cli` to inspect workflow data
- View agent logs: Check the timestamped output directory

## Platform Compatibility

This workflow demo works on all platforms:
- **macOS**: Full compatibility with Docker Desktop
- **Linux**: Full compatibility with native Docker
- **Windows**: Use WSL2 with Docker Desktop

The unified Agentainer startup (`make run` from root directory) automatically handles platform-specific Redis connectivity.

## Troubleshooting

### Common Issues

1. **Agents not starting**: 
   - Run `./setup.sh` to ensure all images are built
   - Check Docker is running: `docker ps`

2. **Authentication errors**: 
   - Ensure token matches config.yaml
   - Check AGENTAINER_AUTH_TOKEN environment variable

3. **Redis connection errors**: 
   - Verify Agentainer is running with `make run`
   - Check Redis container is running: `docker ps | grep redis`

4. **API key errors**: 
   - Check `.env` files have valid API keys
   - Ensure no extra spaces or quotes in API keys

5. **Web extraction errors**:
   - Some websites block automated access
   - Try a different URL
   - Check your internet connection

6. **Missing Results**:
   - The workflow step may have failed
   - Check the workflow status in the dashboard
   - Look at `workflow_final_state.json` for error details

### Debug Commands

```bash
# Check workflow status
curl -H "Authorization: Bearer agentainer-default-token" \
     http://localhost:8081/workflows/<workflow_id>

# Check Redis connectivity
docker exec -it agentainer-lab-redis-1 redis-cli ping

# View agent logs
docker logs <agent-container-id>
```

## Tips for Best Results

1. **Choose Good URLs**: 
   - Articles with substantial text content work best
   - Avoid pages heavy with JavaScript or dynamic content
   - News articles, blog posts, and documentation are ideal

2. **API Rate Limits**:
   - Be mindful of OpenAI and Google API quotas
   - The workflow processes content in chunks, which uses multiple API calls

3. **Performance**:
   - Longer articles take more time to process
   - The parallel processing speeds up analysis significantly
   - Expect 1-3 minutes for typical articles

## Real-World Use Cases

### 1. News Aggregation and Analysis
```txt
# urls.txt for news analysis
https://techcrunch.com/latest/
https://www.theverge.com/tech
https://arstechnica.com/
# Agentainer will find common themes across sources
```

### 2. Competitive Analysis
```txt
# urls.txt for competitor analysis
https://competitor1.com/features
https://competitor2.com/pricing
https://competitor3.com/about
# Get insights about market positioning
```

### 3. Research Paper Analysis
```txt
# urls.txt for academic research
https://arxiv.org/abs/2301.00234
https://arxiv.org/abs/2301.00567
https://arxiv.org/abs/2301.00890
# Extract methodologies and findings
```

### 4. Documentation Review
```txt
# urls.txt for API documentation
https://docs.service.com/api/v1
https://docs.service.com/api/v2
https://docs.service.com/migration
# Understand API evolution and changes
```

## Extending the Workflow

### Adding New Processing Steps

1. Create a new agent with specific capabilities
2. Add the step to workflow configuration:
```python
{
    "id": "sentiment",
    "name": "Analyze Sentiment",
    "type": "sequential",
    "depends_on": ["aggregate"],
    "config": {
        "image": "sentiment-analyzer:latest"
    }
}
```

### Customizing Parallel Processing

```python
"config": {
    "max_parallel": 10,  # Max concurrent agents
    "pool_size": 5,     # Pre-warmed agent pool
    "timeout": "30m"    # Workflow timeout
}
```

### Advanced State Management

For complex data flows:
```python
# Store structured data
set_workflow_state(redis, "analysis_matrix", {
    "url1": {"sentiment": 0.8, "entities": [...]},
    "url2": {"sentiment": 0.6, "entities": [...]}
})

# Use in reduce phase
matrix = get_workflow_state(redis, "analysis_matrix")
avg_sentiment = sum(d["sentiment"] for d in matrix.values()) / len(matrix)
```

## Best Practices

1. **Design for Parallelism**: Structure data so map phase can process independently
2. **Handle Failures Gracefully**: Use `continue_on_partial` for resilient workflows
3. **Optimize State Storage**: Store only necessary data in workflow state
4. **Monitor Progress**: Use the dashboard to track parallel execution
5. **Test Incrementally**: Start with few URLs, then scale up

Enjoy building complex workflows with Agentainer! 🚀