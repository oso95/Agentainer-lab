# Workflow Decorators

Agentainer Flow uses decorators to define workflows and their steps in a declarative way.

## @workflow Decorator

The `@workflow` decorator marks a class as a workflow definition.

### Basic Usage

```python
from agentainer_flow import workflow

@workflow("data-processing")
class DataProcessingWorkflow:
    """Process incoming data through multiple stages"""
    pass
```

### Parameters

- **name** (str, required): Unique identifier for the workflow
- **description** (str, optional): Human-readable description
- **max_parallel** (int, optional): Maximum parallel executions
- **timeout** (str, optional): Workflow timeout (e.g., "30m", "1h")
- **failure_strategy** (str, optional): "fail_fast" (default) or "continue_on_partial"
- **retry_policy** (dict, optional): Retry configuration

### Advanced Example

```python
@workflow(
    "etl-pipeline",
    description="Extract, Transform, Load pipeline",
    max_parallel=5,
    timeout="2h",
    failure_strategy="continue_on_partial",
    retry_policy={
        "max_attempts": 3,
        "backoff": "exponential",
        "delay": "30s"
    }
)
class ETLPipeline:
    pass
```

## @step Decorator

The `@step` decorator defines individual workflow steps.

### Basic Usage

```python
@workflow("my-workflow")
class MyWorkflow:
    @step()
    async def process_data(self, ctx):
        # Step implementation
        data = ctx.state["input_data"]
        result = await process(data)
        ctx.state["processed"] = result
```

### Parameters

- **name** (str, optional): Custom step name (defaults to method name)
- **image** (str, required): Docker image for the step
- **parallel** (bool, optional): Enable parallel execution
- **max_workers** (int, optional): Maximum parallel workers
- **depends_on** (list, optional): List of step names this depends on
- **timeout** (str, optional): Step timeout
- **retry_policy** (dict, optional): Retry configuration
- **env_vars** (dict, optional): Environment variables
- **pooled** (bool, optional): Use agent pooling
- **pool_size** (int, optional): Pool size for pooled execution

### Sequential Steps

```python
@workflow("sequential-workflow")
class SequentialWorkflow:
    @step(image="python:3.9")
    async def step1(self, ctx):
        ctx.state["step1_result"] = "completed"
    
    @step(
        image="python:3.9",
        depends_on=["step1"]
    )
    async def step2(self, ctx):
        # Runs after step1
        previous = ctx.state["step1_result"]
        ctx.state["final"] = f"Step 2 after {previous}"
```

### Parallel Steps

```python
@workflow("parallel-workflow")
class ParallelWorkflow:
    @step(
        image="worker:latest",
        parallel=True,
        max_workers=10
    )
    async def process_items(self, ctx):
        # Each parallel instance gets a unique task_id
        item_id = ctx.task_id
        
        # Process individual item
        result = await process_item(item_id)
        
        # Store result with unique key
        ctx.state[f"item_{item_id}_result"] = result
```

### Steps with Agent Pooling

```python
@workflow("pooled-workflow")
class PooledWorkflow:
    @step(
        image="processor:latest",
        parallel=True,
        pooled=True,
        pool_size=20,
        max_agent_uses=100
    )
    async def high_volume_processing(self, ctx):
        # Agents are reused from pool for better performance
        # Perfect for high-frequency, short-duration tasks
        pass
```

## @parallel Decorator

Shorthand for creating parallel steps.

```python
@workflow("parallel-workflow")
class ParallelWorkflow:
    @parallel(max_workers=5, image="worker:latest")
    async def distributed_task(self, ctx):
        # Automatically configured for parallel execution
        pass
```

### Parameters

Same as `@step` but with `parallel=True` preset.

## @mapreduce Decorator

Simplifies MapReduce pattern implementation.

### Basic Usage

```python
@workflow("mapreduce-workflow")
class MapReduceWorkflow:
    @mapreduce(
        mapper_image="mapper:latest",
        reducer_image="reducer:latest",
        max_parallel=20
    )
    async def process_dataset(self, ctx):
        # This creates two steps automatically:
        # 1. process_dataset_map (parallel)
        # 2. process_dataset_reduce (depends on map)
        pass
```

### Parameters

- **mapper_image** (str, required): Docker image for map tasks
- **reducer_image** (str, required): Docker image for reduce task
- **max_parallel** (int, optional): Maximum parallel mappers
- **pool_size** (int, optional): Use pooling for mappers
- **timeout** (str, optional): Timeout for the operation

### Complete Example

```python
@workflow("analytics-pipeline")
class AnalyticsPipeline:
    @step(image="fetcher:latest")
    async def fetch_data(self, ctx):
        """Fetch data from source"""
        data_urls = await get_data_sources()
        ctx.state["urls"] = data_urls
    
    @mapreduce(
        mapper_image="analyzer:latest",
        reducer_image="aggregator:latest",
        max_parallel=50,
        pool_size=30,
        depends_on=["fetch_data"]
    )
    async def analyze_data(self, ctx):
        """MapReduce analysis of fetched data"""
        # Mappers process individual URLs
        # Reducer aggregates results
        pass
    
    @step(
        image="reporter:latest",
        depends_on=["analyze_data_reduce"]
    )
    async def generate_report(self, ctx):
        """Generate final report"""
        results = ctx.state["analysis_results"]
        report = await create_report(results)
        ctx.state["report"] = report
```

## Decorator Composition

Decorators can be combined for complex workflows:

```python
@workflow(
    "complex-pipeline",
    max_parallel=3,
    timeout="4h"
)
class ComplexPipeline:
    @step(image="validator:latest")
    async def validate_input(self, ctx):
        # Validation logic
        pass
    
    @parallel(
        max_workers=10,
        image="processor:latest",
        depends_on=["validate_input"],
        pooled=True,
        pool_size=15
    )
    async def parallel_processing(self, ctx):
        # Parallel processing with pooling
        pass
    
    @mapreduce(
        mapper_image="mapper:latest",
        reducer_image="reducer:latest",
        max_parallel=20,
        depends_on=["parallel_processing"]
    )
    async def aggregate_results(self, ctx):
        # MapReduce aggregation
        pass
```

## Best Practices

1. **Use descriptive names**: Choose clear, meaningful workflow and step names
2. **Document your workflows**: Use docstrings to explain what each workflow and step does
3. **Handle errors gracefully**: Use try-except blocks and update state with error information
4. **Optimize with pooling**: Use agent pooling for high-frequency, short-duration tasks
5. **Set appropriate timeouts**: Prevent workflows from running indefinitely
6. **Use dependencies wisely**: Create clear execution graphs without circular dependencies

## Next Steps

- Learn about the [Workflow Client](client.md)
- Explore [Testing Workflows](testing.md)
- See [Advanced Topics](advanced.md) for optimization techniques
