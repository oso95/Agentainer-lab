# Agentainer Flow: MapReduce Example

## User's Use Case

> "Suppose I go to a website and get a list of 10 different versions of a bill. I then want to go to 10 different webpages (one for each bill version) and get data from that page (and this should be 10 parallel tasks). Then, after all 10 tasks are finished, I want to combine that data to do further processing."

## Solution with Agentainer Flow

Here's exactly how this would be implemented using our current design:

```python
from agentainer_flow import workflow, step, Context
from typing import List, Dict

@workflow(name="bill_analysis_pipeline")
class BillAnalysisWorkflow:
    
    @step(name="fetch_bill_list")
    async def fetch_bill_list(self, ctx: Context) -> List[Dict]:
        """Step 1: Go to website and get list of bill versions"""
        
        agent = await ctx.deploy_agent(
            name="bill_list_fetcher",
            image="web-scraper:latest",
            env={
                "TARGET_URL": "https://legislation.gov/bills",
                "SELECTOR": ".bill-version-link"
            }
        )
        
        result = await agent.wait_for_completion()
        
        # Store bill list in workflow state
        bill_versions = result.output["bill_versions"]
        ctx.state["bill_count"] = len(bill_versions)
        ctx.state["bill_versions"] = bill_versions
        
        return bill_versions  # e.g., [{"id": "HR1234-v1", "url": "..."}, ...]
    
    @step(
        name="fetch_bill_details",
        depends_on=["fetch_bill_list"],
        parallel=True,  # This enables parallel execution
        dynamic=True    # Number of parallel tasks determined at runtime
    )
    async def fetch_bill_details(self, ctx: Context, bill_version: Dict) -> Dict:
        """Step 2: Fetch details for each bill version IN PARALLEL"""
        
        # Each bill gets its own isolated agent
        agent = await ctx.deploy_agent(
            name=f"bill_fetcher_{bill_version['id']}",
            image="web-scraper:latest",
            env={
                "BILL_URL": bill_version["url"],
                "BILL_ID": bill_version["id"],
                "EXTRACT_FIELDS": "sponsor,summary,full_text,amendments"
            }
        )
        
        result = await agent.wait_for_completion()
        
        # Return the extracted data
        return {
            "bill_id": bill_version["id"],
            "data": result.output
        }
    
    @step(
        name="aggregate_and_analyze",
        depends_on=["fetch_bill_details"],
        reduce=True  # This indicates it receives all parallel results
    )
    async def aggregate_and_analyze(self, ctx: Context, bill_details: List[Dict]) -> Dict:
        """Step 3: Aggregate all bill data and perform analysis"""
        
        # bill_details contains results from ALL parallel executions
        ctx.state["total_bills_processed"] = len(bill_details)
        
        # Deploy agent to perform aggregation and analysis
        agent = await ctx.deploy_agent(
            name="bill_analyzer",
            image="nlp-analyzer:latest",
            env={
                "ANALYSIS_TYPE": "comparative",
                "BILL_DATA": ctx.json(bill_details),  # All 10 bill results
            }
        )
        
        analysis_result = await agent.wait_for_completion()
        
        # Store final results
        ctx.state["analysis_complete"] = True
        ctx.state["summary"] = analysis_result.output["summary"]
        
        return analysis_result.output

# Usage
async def main():
    # Initialize and run the workflow
    wf = BillAnalysisWorkflow()
    
    result = await workflow_client.run_workflow(
        workflow=wf,
        config={
            "timeout": "30m",
            "max_parallel": 10  # Process up to 10 bills simultaneously
        }
    )
    
    print(f"Analyzed {result['bills_analyzed']} bills")
    print(f"Summary: {result['summary']}")
```

## How This Solves the MapReduce Pattern

### 1. **Map Phase (Fan-out)**
```python
@step(parallel=True, dynamic=True)
async def fetch_bill_details(self, ctx: Context, bill_version: Dict):
    # This method is called ONCE for each bill version
    # All executions run in PARALLEL
    # Each runs in its own isolated container
```

The `parallel=True` and `dynamic=True` decorators tell Agentainer Flow to:
- Execute this step multiple times in parallel
- Determine the number of executions based on the previous step's output
- Each execution gets one item from the list

### 2. **Reduce Phase (Fan-in)**
```python
@step(reduce=True)
async def aggregate_and_analyze(self, ctx: Context, bill_details: List[Dict]):
    # Automatically receives ALL results from parallel executions
    # Only runs AFTER all parallel tasks complete
    # Gets a list containing all individual results
```

The `reduce=True` decorator tells Agentainer Flow to:
- Wait for all parallel executions to complete
- Collect all their results into a list
- Pass that list to this aggregation step

### 3. **Automatic Orchestration**
- **No manual task management**: Agentainer Flow handles spawning parallel agents
- **Automatic synchronization**: The reduce step waits for all map tasks
- **Failure handling**: If any parallel task fails, you can configure whether to continue or fail
- **State preservation**: All intermediate results are stored and can be recovered

## Comparison with Other Solutions

### Airflow Equivalent
```python
# In Airflow, this would require:
- DAG definition
- Task operators
- Manual XCom for data passing
- Complex dependency management
- External state management
```

### AWS Step Functions Equivalent
```json
{
  "Comment": "Much more complex JSON definition",
  "StartAt": "FetchBillList",
  "States": {
    "FetchBillList": { /* ... */ },
    "MapState": {
      "Type": "Map",
      "ItemsPath": "$.bills",
      "MaxConcurrency": 10,
      /* Complex configuration */
    }
  }
}
```

### Agentainer Flow Advantages
1. **Natural Python code** - No JSON/YAML definitions
2. **Automatic agent management** - No manual container orchestration
3. **Built-in state persistence** - No external state store needed
4. **Simple testing** - Mock agents for local development
5. **Type safety** - Full Python type hints

## Advanced Features for MapReduce

### 1. Partial Failure Handling
```python
@step(
    parallel=True,
    error_handler=ErrorHandler.CONTINUE,  # Continue even if some fail
    min_success_rate=0.8  # Succeed if 80% complete
)
async def fetch_bill_details(self, ctx: Context, bill_version: Dict):
    # Tolerates up to 20% failure rate
```

### 2. Progress Tracking
```python
# In the parallel step
await ctx.metric("bills_processed", 1, tags={"status": "success"})

# Monitor progress in real-time
async for event in execution.stream_events():
    if event.type == "metric" and event.name == "bills_processed":
        print(f"Processed {event.value} bills so far")
```

### 3. Resource Control
```python
@workflow(
    name="bill_analysis",
    max_parallel_jobs=10,  # Limit concurrent executions
    resource_pool="high-memory"  # Use specific agent resources
)
```

## Why This Is Better Than Existing Solutions

1. **Developer Experience**
   - Write workflows in Python, not YAML/JSON
   - Local testing with mock agents
   - Type hints and IDE support

2. **Operational Simplicity**
   - No separate cluster to manage (uses existing Docker)
   - Built-in state persistence (Redis)
   - Automatic retry and recovery

3. **Performance**
   - Agents start in seconds
   - Efficient parallel execution
   - Minimal orchestration overhead

4. **Cost**
   - No managed service fees
   - Run on existing infrastructure
   - Pay only for compute resources used

## Conclusion

Agentainer Flow provides **seamless MapReduce capabilities** that match or exceed what users get from Airflow, Dagster, or AWS Step Functions, but with:
- Simpler code (Python decorators vs complex DSLs)
- Better isolation (each task in its own container)
- Easier deployment (no separate orchestration cluster)
- Built-in state management (no external state store)

The bill analysis example shows how naturally MapReduce patterns are expressed in Agentainer Flow, making it a compelling alternative to traditional orchestration solutions.