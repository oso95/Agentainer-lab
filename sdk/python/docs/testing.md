# Testing Workflows

Agentainer Flow provides a comprehensive testing framework for developing and testing workflows locally.

## Overview

The testing framework includes:
- **Mock agents**: Simulate agent behavior without Docker
- **Test runner**: Execute workflows locally
- **Assertions**: Validate workflow behavior
- **Test utilities**: Helper functions for testing

## Mock Framework

### MockAgent

Simulates agent behavior for testing:

```python
from agentainer_flow.testing import MockAgent

# Create a mock agent
agent = MockAgent(
    agent_id="mock-123",
    name="test-worker",
    image="worker:latest",
    return_value="processed_result"
)

# Use in tests
assert agent.id == "mock-123"
assert agent.status == "running"

# Async methods work as expected
await agent.start()
logs = await agent.get_logs()
```

### MockContext

Provides a test context for workflow steps:

```python
from agentainer_flow.testing import MockContext

# Create mock context
ctx = MockContext(
    workflow_id="test-workflow",
    step_id="test-step",
    initial_state={"input": "data"}
)

# Use like real context
ctx.state["output"] = "result"
agent = await ctx.deploy_agent("worker", "worker:latest")
```

### mock_agent Context Manager

```python
from agentainer_flow.testing import mock_agent

async with mock_agent("processor:latest", return_value="done") as agent:
    result = await agent.process()
    assert result == "done"
```

## WorkflowTestRunner

Executes workflows locally without the API:

### Basic Usage

```python
from agentainer_flow import workflow, step
from agentainer_flow.testing import WorkflowTestRunner

@workflow("test-workflow")
class TestWorkflow:
    @step(image="python:3.9")
    async def process(self, ctx):
        ctx.state["result"] = ctx.state["input"] * 2
        return ctx.state["result"]

async def test_workflow():
    runner = WorkflowTestRunner()
    workflow = TestWorkflow()
    
    result = await runner.run_workflow(
        workflow,
        initial_state={"input": 5}
    )
    
    assert result.success
    assert result.final_state["result"] == 10
```

### Testing Individual Steps

```python
async def test_single_step():
    runner = WorkflowTestRunner()
    workflow = TestWorkflow()
    
    # Run just one step
    result = await runner.run_step(
        workflow,
        "process",
        initial_state={"input": 3}
    )
    
    assert result == 6
```

### Providing Mock Agents

```python
async def test_with_mocks():
    runner = WorkflowTestRunner()
    
    # Create mock agents
    mocks = [
        MockAgent("mock-1", "worker-1", return_value="result-1"),
        MockAgent("mock-2", "worker-2", return_value="result-2")
    ]
    
    result = await runner.run_workflow(
        workflow,
        agent_mocks={"worker:latest": mocks}
    )
    
    assert result.success
```

## WorkflowAssertions

Provides convenient assertions for workflow testing:

```python
from agentainer_flow.testing import WorkflowAssertions

result = await runner.run_workflow(workflow)
assertions = WorkflowAssertions(result)

# Check workflow completed successfully
assertions.assert_success()
assertions.assert_no_errors()

# Check specific steps completed
assertions.assert_step_completed("process")
assertions.assert_step_completed("validate")

# Check state values
assertions.assert_state_contains("output", "expected_value")
assertions.assert_state_matches("count", lambda x: x > 10)

# Check agent deployments
assertions.assert_agents_deployed(5)  # Total agents
assertions.assert_agents_deployed(3, step="process")  # Per step
```

## Testing Patterns

### Unit Testing Steps

```python
import pytest
from agentainer_flow.testing import MockContext

@workflow("data-pipeline")
class DataPipeline:
    @step(image="processor:latest")
    async def validate_data(self, ctx):
        data = ctx.state.get("raw_data", [])
        
        if not data:
            raise ValueError("No data provided")
        
        valid_items = [item for item in data if item.get("valid")]
        ctx.state["valid_count"] = len(valid_items)
        ctx.state["validated_data"] = valid_items
        
        return valid_items

@pytest.mark.asyncio
async def test_validate_data_success():
    # Create test context
    ctx = MockContext(
        workflow_id="test",
        step_id="validate",
        initial_state={
            "raw_data": [
                {"id": 1, "valid": True},
                {"id": 2, "valid": False},
                {"id": 3, "valid": True}
            ]
        }
    )
    
    # Execute step
    pipeline = DataPipeline()
    result = await pipeline.validate_data(ctx)
    
    # Assertions
    assert len(result) == 2
    assert ctx.state["valid_count"] == 2
    assert ctx.state["validated_data"][0]["id"] == 1

@pytest.mark.asyncio
async def test_validate_data_empty():
    ctx = MockContext(
        workflow_id="test",
        step_id="validate",
        initial_state={"raw_data": []}
    )
    
    pipeline = DataPipeline()
    
    with pytest.raises(ValueError, match="No data provided"):
        await pipeline.validate_data(ctx)
```

### Integration Testing

```python
@pytest.mark.asyncio
async def test_full_workflow():
    @workflow("etl-pipeline")
    class ETLPipeline:
        @step(image="fetcher:latest")
        async def fetch(self, ctx):
            # Simulate fetching data
            ctx.state["raw_data"] = [
                {"id": i, "value": i * 10}
                for i in range(10)
            ]
        
        @step(image="transformer:latest", depends_on=["fetch"])
        async def transform(self, ctx):
            data = ctx.state["raw_data"]
            transformed = [
                {"id": item["id"], "doubled": item["value"] * 2}
                for item in data
            ]
            ctx.state["transformed_data"] = transformed
        
        @step(image="loader:latest", depends_on=["transform"])
        async def load(self, ctx):
            # Simulate loading to database
            data = ctx.state["transformed_data"]
            ctx.state["loaded_count"] = len(data)
            ctx.state["status"] = "completed"
    
    runner = WorkflowTestRunner()
    pipeline = ETLPipeline()
    
    result = await runner.run_workflow(pipeline)
    
    assertions = WorkflowAssertions(result)
    assertions.assert_success()
    assertions.assert_all_steps_completed()
    assertions.assert_state_contains("loaded_count", 10)
    assertions.assert_state_contains("status", "completed")
```

### Testing Parallel Steps

```python
@pytest.mark.asyncio
async def test_parallel_execution():
    @workflow("parallel-test")
    class ParallelWorkflow:
        @step(image="worker:latest", parallel=True, max_workers=5)
        async def process_batch(self, ctx):
            # Each parallel instance processes one item
            item_id = ctx.parallel_index
            
            # Simulate processing
            result = f"processed_{item_id}"
            
            # Store with unique key
            ctx.state[f"item_{item_id}"] = result
            
            return result
    
    runner = WorkflowTestRunner()
    workflow = ParallelWorkflow()
    
    # Create mock agents for parallel execution
    mocks = [
        MockAgent(f"mock-{i}", f"worker-{i}", return_value=f"processed_{i}")
        for i in range(5)
    ]
    
    result = await runner.run_workflow(
        workflow,
        agent_mocks={"worker:latest": mocks}
    )
    
    # Verify all parallel tasks completed
    assertions = WorkflowAssertions(result)
    assertions.assert_success()
    
    # Check all items were processed
    for i in range(5):
        assertions.assert_state_contains(f"item_{i}", f"processed_{i}")
```

### Testing Error Handling

```python
@pytest.mark.asyncio
async def test_error_handling():
    @workflow("error-test")
    class ErrorWorkflow:
        @step(image="validator:latest")
        async def validate(self, ctx):
            if not ctx.state.get("skip_validation"):
                raise ValueError("Validation failed")
        
        @step(
            image="processor:latest",
            depends_on=["validate"],
            retry_policy={"max_attempts": 3}
        )
        async def process(self, ctx):
            # This step should not run if validation fails
            ctx.state["processed"] = True
    
    runner = WorkflowTestRunner()
    workflow = ErrorWorkflow()
    
    # Test failure case
    result = await runner.run_workflow(
        workflow,
        initial_state={"skip_validation": False}
    )
    
    assert not result.success
    assert "validate" in result.failed_steps
    assert "process" not in result.completed_steps
    
    # Test success case
    result = await runner.run_workflow(
        workflow,
        initial_state={"skip_validation": True}
    )
    
    assertions = WorkflowAssertions(result)
    assertions.assert_success()
    assertions.assert_step_completed("process")
```

## Testing Best Practices

1. **Test in isolation**: Test individual steps separately before integration
2. **Use meaningful test data**: Create realistic test scenarios
3. **Test error paths**: Verify error handling works correctly
4. **Mock external dependencies**: Use MockAgent for external services
5. **Assert thoroughly**: Check state, completion, and side effects
6. **Test edge cases**: Empty data, large datasets, concurrent access

## Pytest Integration

### Fixtures

```python
import pytest
from agentainer_flow.testing import WorkflowTestRunner

@pytest.fixture
def runner():
    return WorkflowTestRunner()

@pytest.fixture
def mock_agents():
    return [
        MockAgent("mock-1", "agent-1"),
        MockAgent("mock-2", "agent-2")
    ]

@pytest.mark.asyncio
async def test_with_fixtures(runner, mock_agents):
    result = await runner.run_workflow(
        workflow,
        agent_mocks={"worker:latest": mock_agents}
    )
    assert result.success
```

### Parametrized Tests

```python
@pytest.mark.asyncio
@pytest.mark.parametrize("input_value,expected", [
    (5, 10),
    (0, 0),
    (-5, -10),
    (100, 200)
])
async def test_processing(input_value, expected):
    runner = WorkflowTestRunner()
    
    result = await runner.run_step(
        workflow,
        "double_value",
        initial_state={"input": input_value}
    )
    
    assert result == expected
```

## Debugging Tests

### Enable Verbose Output

```python
runner = WorkflowTestRunner(verbose=True)
# Shows detailed execution logs
```

### Inspect Intermediate State

```python
result = await runner.run_workflow(workflow)

# Print execution details
print(f"Completed steps: {result.completed_steps}")
print(f"Failed steps: {result.failed_steps}")
print(f"Final state: {result.final_state}")
print(f"Errors: {result.errors}")

# Access step contexts
for step_name, context in result.contexts.items():
    print(f"Step {step_name} deployed agents: {context.deployed_agents}")
```

## Next Steps

- Explore [Advanced Topics](advanced.md)
- See [Examples](../examples/) for real-world testing scenarios
- Read the [API Reference](api-reference.md)
