# Multi-URL Analysis Workflow Demo

This example demonstrates Agentainer's map/reduce workflow orchestration for parallel web content analysis using AI.

## What This Demo Does

1. **Prepares** a list of URLs for processing
2. **Maps** URL processing across parallel containers (using Agentainer's new map step type)
3. **Reduces** results into aggregated data
4. **Analyzes** content with AI agents (Gemini for entity extraction, GPT for insights)
5. **Generates** a comprehensive report combining all findings

## Quick Start

### Method 1: All-in-One Script (Recommended)

```bash
# 1. Set up API keys
echo "OPENAI_API_KEY=your-key-here" > gpt-workflow-agent/.env
echo "GEMINI_API_KEY=your-key-here" > gemini-workflow-agent/.env

# 2. Run everything (builds images + runs workflow in container)
./run.sh
```

This single command:
- Checks prerequisites
- Builds all Docker images
- Runs the workflow in a container
- Saves results to `results/` directory

### Method 2: Direct Python (For Development)

```bash
# 1. Set up API keys (same as above)

# 2. Build images manually
docker build -t doc-extractor:latest doc-extractor
docker build -t gpt-workflow-agent:latest gpt-workflow-agent
docker build -t gemini-workflow-agent:latest gemini-workflow-agent

# 3. Install Python dependencies
pip install redis requests

# 4. Run directly
python3 run_workflow.py
```

Results will be saved to `analysis_results_*` directory.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Prepare   │ --> │     Map     │ --> │   Reduce    │
│ (Sequential)│     │ (Parallel)  │     │(Sequential) │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                    ┌──────┴──────┐
                    │             │
                 URL1          URL2         URL3
                           
                           ↓
                           
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Entities   │ --> │  Insights   │ --> │   Report    │
│  (Gemini)   │     │   (GPT)     │     │  (Gemini)   │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Workflow Configuration

### Key Components

1. **Map Step Configuration** (`run_workflow.py`):
```python
{
    "id": "process",
    "name": "Process URLs in Parallel",
    "type": "map",  # New Agentainer map step type
    "config": {
        "map_config": {
            "input_path": "urls",  # Path to array in workflow state
            "item_alias": "current_url",  # Variable name for each item
            "max_concurrency": 3,  # Max parallel containers
            "error_handling": "continue_on_error"
        }
    }
}
```

2. **Task Types** (defined in env_vars):
- `prepare_urls` - Load and validate URLs
- `process_url` - Fetch and analyze single URL (map phase)
- `aggregate` - Combine results from all URLs
- `extract_entities_multi` - AI entity extraction
- `generate_insights_multi` - AI insight generation
- `final_report_multi` - AI report generation

## File Structure

```
workflow-demo/
├── README.md                    # This file
├── setup.sh                     # Build script
├── run_workflow.py              # Main workflow orchestrator
├── urls.txt                     # Input URLs (user reference)
│
├── doc-extractor/               # Web content processor
│   ├── Dockerfile
│   ├── app_multi.py            # Handles prepare/map/reduce phases
│   └── urls.txt                # URLs for Docker build
│
├── gemini-workflow-agent/       # Gemini AI agent
│   ├── Dockerfile
│   ├── app.py                  # Entity extraction & report generation
│   └── .env                    # API key (create this)
│
└── gpt-workflow-agent/          # GPT AI agent
    ├── Dockerfile
    ├── app.py                  # Insight generation
    └── .env                    # API key (create this)
```

## Key Implementation Details

### 1. Map Step Execution (Agentainer Core)

The map step dynamically creates tasks based on input data:
```go
// internal/workflow/orchestrator.go
func (o *Orchestrator) executeMapStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
    // Extract array from workflow state
    items := extractMapInput(workflow.State, mapConfig.InputPath)
    
    // Create parallel tasks for each item
    for i, item := range items {
        taskInput[mapConfig.ItemAlias] = item
        // Deploy container for this task
    }
}
```

### 2. State Management

All agents communicate through Redis:
```python
# Store result for URL processing
redis_client.hset(f"workflow:{WORKFLOW_ID}:state", f"result_url_{i}", json.dumps(result))

# Retrieve in aggregation phase
result = redis_client.hget(f"workflow:{WORKFLOW_ID}:state", f"result_url_{i}")
```

### 3. Output Files

Results are saved to `analysis_results_TIMESTAMP/`:
- `url_N_content.txt` - Full article content
- `url_N_metadata.json` - URL metadata (title, word count, etc.)
- `aggregated_results.json` - Combined processing results
- `entities.md` - Extracted entities
- `insights.md` - Cross-article insights
- `FINAL_REPORT.md` - Comprehensive analysis

## Customization Guide

### Adding More URLs

Edit `doc-extractor/urls.txt`:
```
https://example.com/article1
https://example.com/article2
# Comments are supported
```

### Modifying AI Prompts

1. **Entity Extraction**: Edit `gemini-workflow-agent/app.py` → `extract_entities_multi()`
2. **Insights**: Edit `gpt-workflow-agent/app.py` → `generate_insights_multi()`
3. **Report**: Edit `gemini-workflow-agent/app.py` → `final_report_multi()`

### Changing Parallel Concurrency

In `run_workflow.py`, modify the map_config:
```python
"max_concurrency": 5,  # Process 5 URLs simultaneously
```

### Adding New Analysis Steps

1. Create new agent directory with Dockerfile and app.py
2. Add new step to workflow configuration in `run_workflow.py`
3. Implement task handler matching the TASK_TYPE

## Troubleshooting

### Common Issues

1. **"urls.txt not found"**: Ensure `urls.txt` exists in `doc-extractor/` directory
2. **"Unknown task type"**: Check that TASK_TYPE in env_vars matches handler in agent
3. **Redis connection failed**: Verify Agentainer is running (`make run`)
4. **API errors**: Check `.env` files have valid API keys

### Debugging Tips

- Check container logs: `docker logs <container-name>`
- Monitor workflow: `curl http://localhost:8081/dashboard/workflows`
- Inspect Redis state: `redis-cli hgetall workflow:<id>:state`

## API Reference

### Workflow API Endpoints

- `POST /workflows` - Create new workflow
- `POST /workflows/{id}/start` - Start workflow execution
- `GET /workflows/{id}` - Get workflow status
- `GET /dashboard/workflows` - View all workflows

### Workflow State Keys

- `urls` - Array of URL objects to process
- `result_url_N` - Processing result for URL N
- `content_url_N` - Full content for URL N
- `aggregated_results` - Combined results from all URLs
- `extracted_entities_multi` - AI-extracted entities
- `insights_multi` - AI-generated insights
- `final_report_multi` - Final analysis report

## Advanced Features

### Error Handling

The map step supports different error handling strategies:
- `continue_on_error` - Continue processing even if some URLs fail
- `fail_fast` - Stop on first error

### Resource Limits

Each container has configurable resource limits:
```python
"resource_limits": {
    "cpu_limit": 500000000,    # 0.5 CPU cores
    "memory_limit": 268435456   # 256 MB
}
```

### Dynamic Task Creation

The map step creates tasks dynamically based on workflow state, enabling:
- Variable number of URLs
- Conditional processing
- Data-driven parallelism

## Contributing

To extend this demo:
1. Fork the repository
2. Create your feature branch
3. Add new agents or workflow steps
4. Update this README
5. Submit a pull request

## Related Examples

- `mapreduce-workflow/` - Pure map-reduce computation example
- `simple-workflow/` - Basic sequential workflow
- `scheduler-demo/` - Scheduled workflow execution