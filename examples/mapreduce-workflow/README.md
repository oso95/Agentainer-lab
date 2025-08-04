# MapReduce Word Counter Workflow

A production-ready example demonstrating Agentainer's MapReduce pattern for parallel URL processing with automatic retry, error handling, and comprehensive reporting.

## 🎯 What This Example Does

This workflow analyzes multiple web pages in parallel to:
1. Extract and count words from each URL
2. Find the most common words per page
3. Aggregate results into comprehensive statistics
4. Export results in multiple formats (JSON, CSV, Markdown)

## 🏗️ Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  List Phase │ --> │  Map Phase  │ --> │Reduce Phase │
│ (Sequential)│     │  (Parallel) │     │(Sequential) │
└─────────────┘     └─────────────┘     └─────────────┘
      |                    |                    |
   urls.txt          Process URLs         Final Report
```

### Phase Details

1. **List Phase** (`mapper.py` with `STEP_TYPE=list`)
   - Reads URLs from `urls.txt` or environment
   - Validates and prepares URLs for processing
   - Stores URL list in Redis workflow state

2. **Map Phase** (`mapper.py` with `STEP_TYPE=map`)
   - Agentainer creates parallel tasks for each URL
   - Each mapper fetches and analyzes one URL
   - Extracts text, counts words, finds top words
   - Handles retries for failed requests
   - Results stored in Redis lists

3. **Reduce Phase** (`reducer.py`)
   - Aggregates all mapper results
   - Calculates total statistics
   - Creates final summary and reports

## 🚀 Quick Start

### Prerequisites
1. **Docker Desktop** running
2. **Agentainer server** running: `./agentainer server`
3. **Redis** running (auto-started by Agentainer)

### Run the Example

```bash
# 1. Build Docker images
./build.sh

# 2. Add URLs to analyze (optional - defaults provided)
echo "https://example.com" >> urls.txt

# 3. Run the workflow
python3 run_workflow_api.py
```

## 📝 Configuration

### Input URLs (`urls.txt`)
```text
# URLs to process - one per line
# Lines starting with # are ignored

https://example.com
https://techcrunch.com/article-url
https://your-site.com
```

### Command Line Options
```bash
# Use custom URLs file
python3 run_workflow_api.py --urls myurls.txt

# Specify output directory
python3 run_workflow_api.py --output results_dir

# Skip CSV/detailed exports
python3 run_workflow_api.py --no-export
```

## 🔧 Key Features

### 1. Automatic Retry
Failed tasks retry with configurable backoff:
- **List phase**: 2 attempts, constant backoff
- **Map phase**: 3 attempts, exponential backoff
- **Reduce phase**: 2 attempts, linear backoff

### 2. Error Resilience
- Continues processing even if some URLs fail
- Failed URLs tracked separately in `map_errors.json`
- Success rate calculated in final report

### 3. Resource Management
- CPU limits: 0.5 CPU per mapper, 1 CPU for reducer
- Memory limits: 256MB per mapper, 512MB for reducer
- Max 5 concurrent mappers

### 4. Agent Cleanup
- `cleanup_policy: "on_success"` - keeps failed agents for debugging
- Failed agents preserved until retry attempts exhausted
- Successful agents cleaned up automatically

## 📊 Output Files

Results are saved to timestamped directory: `mapreduce_results_YYYYMMDD_HHMMSS/`

### Core Outputs
| File | Description |
|------|-------------|
| `workflow_config.json` | Complete workflow configuration |
| `workflow_metadata.json` | Workflow ID and timestamp |
| `input_urls.json` | URLs that were processed |
| `generated_urls.json` | URLs prepared by list phase |
| `map_results.json` | Detailed results from each URL |
| `map_errors.json` | Information about failed URLs |
| `final_summary.json` | Aggregated statistics |
| `FINAL_REPORT.md` | Human-readable summary |

### Per-URL Outputs
| File | Description |
|------|-------------|
| `wordcount_<url>.json` | Word statistics for each URL |

### Export Formats
| File | Description |
|------|-------------|
| `word_frequencies.csv` | Top words with counts and percentages |
| `url_performance.csv` | Response times and word counts per URL |
| `DETAILED_ANALYSIS.md` | Comprehensive analysis report |

## 🐛 Debugging

### Common Issues

1. **"Cannot connect to Docker"**
   ```bash
   # Ensure Docker Desktop is running
   docker info
   ```

2. **"Connection refused :8081"**
   ```bash
   # Start Agentainer server from repo root
   ./agentainer server
   ```

3. **"No such image: mapreduce-mapper"**
   ```bash
   # Build images first
   ./build.sh
   ```

### Monitoring Tools

```bash
# View workflow status
curl http://localhost:8081/workflows/<workflow-id>

# Check Redis state
redis-cli HGETALL workflow:<workflow-id>:state

# View mapper logs
docker logs <container-id>

# List workflow agents
agentainer list --workflow <workflow-id>
```

## 🔬 Technical Details

### State Management
All state stored in Redis with these key patterns:
- `workflow:{id}:state` - Main workflow state hash
- `workflow:{id}:state:list:{key}` - List data (map results)
- `task:{id}:result` - Individual task results
- `task:{id}:complete` - Completion notifications (pub/sub)

### Task Communication
1. Mapper writes result to `task:{id}:result`
2. Publishes to `task:{id}:complete` channel
3. Orchestrator detects completion via pub/sub
4. Reducer reads all results from Redis lists

### Map Step Configuration
```python
"map_config": {
    "input_path": "urls",        # Array from workflow state
    "item_alias": "current_url", # Variable name in task
    "max_concurrency": 5,        # Parallel limit
    "error_handling": "continue_on_error"
}
```

## 🎨 Customization

### Modify Processing Logic
Edit `mapper.py` to change what's analyzed:
```python
def map_phase():
    # Your custom processing logic
    url = workflow_state.get("current_url")
    # Process URL...
    result = {"your": "data"}
    append_to_list("map_results", result)
```

### Change Aggregation
Edit `reducer.py` to modify how results are combined:
```python
def aggregate_results():
    # Your custom aggregation logic
    for result in results:
        # Aggregate...
```

### Add New Metrics
Extend the result dictionaries to include:
- Sentiment analysis
- Link extraction
- Image counting
- Response headers
- Custom analytics

## 📚 Learning Resources

### Key Files to Study
1. `run_workflow_api.py` - Workflow orchestration
2. `mapper.py` - List and map phase implementation
3. `reducer.py` - Aggregation logic

### Concepts Demonstrated
- Dynamic parallel task creation
- Redis-based state sharing
- Retry mechanisms with backoff
- Error handling and resilience
- Resource constraints
- Result aggregation patterns

## 🤝 Contributing

To improve this example:
1. Add more sophisticated text analysis
2. Implement different retry strategies
3. Add more export formats
4. Create visualizations of results
5. Add unit tests

## 📄 License

This example is part of the Agentainer project and follows the same license terms.