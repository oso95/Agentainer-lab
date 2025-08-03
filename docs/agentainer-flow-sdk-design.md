# Agentainer Flow SDK Design

## Overview

The Agentainer Flow SDK provides a Python-first interface for defining and managing multi-step workflows. It abstracts the complexity of orchestration while providing flexibility for advanced use cases.

## Design Principles

1. **Intuitive API**: Leverage Python decorators and context managers for natural workflow definition
2. **Type Safety**: Full type hints and runtime validation
3. **Progressive Disclosure**: Simple workflows are simple, complex workflows are possible
4. **Testability**: Built-in support for local testing and mocking
5. **Async-First**: Native async/await support for efficient execution

## Core API Components

### 1. Workflow Definition

```python
from agentainer_flow import workflow, step, Context
from typing import Dict, List, Any

@workflow(
    name="my_workflow",
    description="Process data through multiple stages",
    max_retries=3,
    timeout="1h"
)
class DataWorkflow:
    """A complete data processing workflow."""
    
    def __init__(self, config: Dict[str, Any]):
        self.config = config
    
    @step(name="fetch", timeout="10m")
    async def fetch_data(self, ctx: Context) -> Dict[str, Any]:
        """Fetch data from external source."""
        agent = await ctx.deploy_agent(
            name="fetcher",
            image="my-fetcher:latest",
            env={
                "SOURCE_URL": self.config["source_url"],
                "API_KEY": ctx.secrets["api_key"]
            }
        )
        result = await agent.wait_for_completion()
        
        # Store in workflow state
        ctx.state["raw_data"] = result.output["data_path"]
        return result.output
    
    @step(name="validate", depends_on=["fetch"])
    async def validate_data(self, ctx: Context, fetch_result: Dict) -> bool:
        """Validate the fetched data."""
        agent = await ctx.deploy_agent(
            name="validator",
            image="data-validator:latest",
            env={"DATA_PATH": fetch_result["data_path"]}
        )
        result = await agent.wait_for_completion()
        return result.output["is_valid"]
    
    @step(
        name="process",
        depends_on=["validate"],
        parallel=True,
        max_workers=5
    )
    async def process_chunk(self, ctx: Context, chunk_id: int) -> Dict:
        """Process a single chunk of data in parallel."""
        agent = await ctx.deploy_agent(
            name=f"processor-{chunk_id}",
            image="processor:latest",
            env={
                "DATA_PATH": ctx.state["raw_data"],
                "CHUNK_ID": str(chunk_id),
                "TOTAL_CHUNKS": str(ctx.parallel_context.total)
            }
        )
        return await agent.wait_for_completion()
    
    @step(
        name="aggregate",
        depends_on=["process"],
        reduce=True  # Indicates this step aggregates parallel results
    )
    async def aggregate_results(
        self, 
        ctx: Context, 
        process_results: List[Dict]
    ) -> Dict:
        """Aggregate results from parallel processing."""
        # Store intermediate results
        ctx.state["processed_chunks"] = len(process_results)
        
        agent = await ctx.deploy_agent(
            name="aggregator",
            image="aggregator:latest",
            env={
                "CHUNK_RESULTS": ctx.json(process_results),
                "OUTPUT_PATH": self.config["output_path"]
            }
        )
        return await agent.wait_for_completion()
```

### 2. Workflow Execution

```python
from agentainer_flow import WorkflowClient, ExecutionMode

# Initialize client
client = WorkflowClient(
    api_url="http://localhost:8081",
    api_token="your-token"
)

# Create and run workflow
async def main():
    # Define workflow configuration
    config = {
        "source_url": "https://api.example.com/data",
        "output_path": "s3://bucket/output"
    }
    
    # Create workflow instance
    wf = DataWorkflow(config)
    
    # Execute workflow
    execution = await client.run_workflow(
        workflow=wf,
        mode=ExecutionMode.ASYNC,
        secrets={"api_key": "secret-key"},
        tags={"team": "data-eng", "priority": "high"}
    )
    
    # Monitor execution
    async for event in execution.stream_events():
        print(f"[{event.timestamp}] {event.step}: {event.message}")
    
    # Get final result
    result = await execution.get_result()
    print(f"Workflow completed: {result}")

# Run
import asyncio
asyncio.run(main())
```

### 3. Advanced Patterns

#### Conditional Execution

```python
@workflow(name="conditional_workflow")
class ConditionalWorkflow:
    
    @step(name="check_condition")
    async def check_condition(self, ctx: Context) -> str:
        # Some logic to determine path
        if ctx.input["data_size"] > 1000:
            return "large"
        else:
            return "small"
    
    @step(
        name="process_large",
        depends_on=["check_condition"],
        when=lambda result: result == "large"
    )
    async def process_large_data(self, ctx: Context):
        # Process large dataset
        pass
    
    @step(
        name="process_small",
        depends_on=["check_condition"],
        when=lambda result: result == "small"
    )
    async def process_small_data(self, ctx: Context):
        # Process small dataset
        pass
```

#### Sub-Workflows

```python
@workflow(name="parent_workflow")
class ParentWorkflow:
    
    @step(name="prepare")
    async def prepare_data(self, ctx: Context):
        # Prepare data
        return {"data_path": "/path/to/data"}
    
    @step(name="run_sub_workflow", depends_on=["prepare"])
    async def run_sub_workflow(self, ctx: Context, prepare_result: Dict):
        # Run another workflow as a step
        sub_wf = DataProcessingWorkflow(prepare_result)
        result = await ctx.run_workflow(sub_wf)
        return result
```

#### Dynamic Parallelism

```python
@workflow(name="dynamic_parallel")
class DynamicParallelWorkflow:
    
    @step(name="determine_tasks")
    async def determine_tasks(self, ctx: Context) -> List[Dict]:
        # Dynamically determine what to process
        agent = await ctx.deploy_agent(
            name="analyzer",
            image="analyzer:latest"
        )
        result = await agent.wait_for_completion()
        return result.output["tasks"]  # List of task definitions
    
    @step(
        name="process_task",
        depends_on=["determine_tasks"],
        parallel=True,
        dynamic=True  # Indicates dynamic parallelism
    )
    async def process_task(self, ctx: Context, task: Dict):
        # Process each task from the previous step
        agent = await ctx.deploy_agent(
            name=f"processor-{task['id']}",
            image=task["processor_image"],
            env=task["env_vars"]
        )
        return await agent.wait_for_completion()
```

### 4. Context API

```python
class Context:
    """Workflow execution context."""
    
    # Workflow metadata
    workflow_id: str
    workflow_name: str
    execution_id: str
    
    # Step metadata
    step_id: str
    step_name: str
    
    # Workflow state (persistent across steps)
    state: WorkflowState
    
    # Secrets management
    secrets: SecretStore
    
    # Input from workflow initialization
    input: Dict[str, Any]
    
    # Parallel execution context (if in parallel step)
    parallel_context: Optional[ParallelContext]
    
    # Agent deployment
    async def deploy_agent(
        self,
        name: str,
        image: str,
        env: Dict[str, str] = None,
        command: List[str] = None,
        resources: ResourceRequirements = None,
        timeout: str = "30m"
    ) -> AgentHandle:
        """Deploy an agent as part of the workflow."""
        pass
    
    # Sub-workflow execution
    async def run_workflow(
        self,
        workflow: Workflow,
        wait: bool = True
    ) -> WorkflowExecution:
        """Run a sub-workflow."""
        pass
    
    # Utilities
    def json(self, obj: Any) -> str:
        """Serialize object to JSON."""
        pass
    
    async def log(self, message: str, level: str = "info"):
        """Log a message to workflow logs."""
        pass
    
    async def metric(self, name: str, value: float, tags: Dict = None):
        """Record a metric."""
        pass
```

### 5. Testing Support

```python
from agentainer_flow.testing import WorkflowTestCase, mock_agent

class TestDataWorkflow(WorkflowTestCase):
    
    async def test_successful_workflow(self):
        # Setup test data
        config = {"source_url": "test://data", "output_path": "test://output"}
        wf = DataWorkflow(config)
        
        # Mock agent responses
        with mock_agent("fetcher") as fetcher:
            fetcher.returns({"data_path": "/test/data.json"})
        
        with mock_agent("validator") as validator:
            validator.returns({"is_valid": True})
        
        with mock_agent("processor-*") as processor:
            processor.returns({"processed": True})
        
        with mock_agent("aggregator") as aggregator:
            aggregator.returns({"result": "success"})
        
        # Run workflow
        result = await self.run_workflow(wf)
        
        # Assertions
        assert result["result"] == "success"
        assert self.workflow_state["raw_data"] == "/test/data.json"
        assert self.workflow_state["processed_chunks"] == 5
```

### 6. CLI Integration

```python
# agentainer_flow/cli.py
import click
from agentainer_flow import WorkflowClient

@click.group()
def workflow():
    """Manage Agentainer workflows."""
    pass

@workflow.command()
@click.argument('workflow_file')
@click.option('--config', type=click.Path(), help='Workflow config file')
@click.option('--wait/--no-wait', default=True, help='Wait for completion')
def run(workflow_file, config, wait):
    """Run a workflow from file."""
    # Load and execute workflow
    pass

@workflow.command()
@click.argument('workflow_id')
def status(workflow_id):
    """Get workflow status."""
    client = WorkflowClient()
    status = client.get_workflow_status(workflow_id)
    click.echo(f"Workflow {workflow_id}: {status}")

@workflow.command()
@click.argument('workflow_id')
@click.option('--follow', '-f', is_flag=True, help='Follow log output')
def logs(workflow_id, follow):
    """View workflow logs."""
    client = WorkflowClient()
    if follow:
        for log in client.stream_logs(workflow_id):
            click.echo(log)
    else:
        logs = client.get_logs(workflow_id)
        click.echo('\n'.join(logs))
```

### 7. Error Handling and Retries

```python
from agentainer_flow import RetryPolicy, ErrorHandler

@workflow(
    name="resilient_workflow",
    retry_policy=RetryPolicy(
        max_attempts=3,
        backoff="exponential",
        max_delay="5m"
    )
)
class ResilientWorkflow:
    
    @step(
        name="flaky_step",
        retry_policy=RetryPolicy(max_attempts=5),
        error_handler=ErrorHandler.CONTINUE  # Continue workflow on failure
    )
    async def flaky_operation(self, ctx: Context):
        # Operation that might fail
        pass
    
    @step(
        name="critical_step",
        error_handler=ErrorHandler.FAIL  # Fail workflow on error
    )
    async def critical_operation(self, ctx: Context):
        # Critical operation
        pass
    
    @step(
        name="cleanup",
        always_run=True  # Run even if previous steps failed
    )
    async def cleanup(self, ctx: Context):
        # Cleanup resources
        pass
```

### 8. Monitoring and Observability

```python
from agentainer_flow import WorkflowClient

client = WorkflowClient()

# Get workflow metrics
metrics = await client.get_workflow_metrics(
    workflow_name="data_workflow",
    time_range="24h"
)

print(f"Total executions: {metrics.total_executions}")
print(f"Success rate: {metrics.success_rate}%")
print(f"Average duration: {metrics.avg_duration}")

# Stream workflow events
async for event in client.stream_workflow_events(
    workflow_name="data_workflow",
    event_types=["step_started", "step_completed", "workflow_failed"]
):
    print(f"{event.timestamp}: {event.type} - {event.details}")
```

## SDK Package Structure

```
agentainer_flow/
├── __init__.py
├── decorators.py      # @workflow, @step decorators
├── context.py         # Context and related classes
├── client.py          # WorkflowClient implementation
├── models.py          # Data models
├── execution.py       # Execution engine
├── testing.py         # Testing utilities
├── cli.py            # CLI commands
├── errors.py         # Exception classes
└── utils.py          # Utility functions
```

## Integration with Agentainer Core

The SDK integrates seamlessly with existing Agentainer functionality:

1. **Agent Deployment**: Uses existing AgentManager for container lifecycle
2. **State Storage**: Leverages Redis for workflow state persistence
3. **Request Tracking**: Integrates with RequestManager for job tracking
4. **Network Isolation**: Maintains Agentainer's security model
5. **Monitoring**: Extends existing metrics collection

## Summary

The Agentainer Flow SDK provides a powerful yet approachable interface for defining complex workflows. It maintains the simplicity that makes Agentainer attractive while adding the orchestration capabilities needed for production workloads. The progressive disclosure approach ensures developers can start simple and add complexity as needed.