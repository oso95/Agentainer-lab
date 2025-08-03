# Workflow Client API

The `WorkflowClient` provides programmatic access to the Agentainer Flow API.

## Client Initialization

### Basic Usage

```python
from agentainer_flow import WorkflowClient

# Using default settings
async with WorkflowClient() as client:
    # Use client
    pass

# Custom configuration
client = WorkflowClient(
    base_url="http://localhost:8081",
    token="your-api-token",
    timeout=60.0
)
```

### Parameters

- **base_url** (str): API endpoint (default: "http://localhost:8081")
- **token** (str): Authentication token (default: "default-token")
- **timeout** (float): Request timeout in seconds (default: 30.0)

## Workflow Management

### Creating Workflows

```python
# Create from decorator-based class
@workflow("my-workflow")
class MyWorkflow:
    @step(image="python:3.9")
    async def process(self, ctx):
        pass

workflow_instance = MyWorkflow()
execution = await client.run_workflow(
    workflow_instance,
    input_data={"key": "value"}
)

# Create workflow manually
workflow = await client.create_workflow(
    name="manual-workflow",
    description="Created via API",
    config={
        "max_parallel": 5,
        "timeout": "1h"
    }
)
```

### Listing Workflows

```python
# List all workflows
workflows = await client.list_workflows()

# Filter by status
from agentainer_flow import WorkflowStatus

running_workflows = await client.list_workflows(
    status=WorkflowStatus.RUNNING
)
```

### Getting Workflow Details

```python
workflow = await client.get_workflow(workflow_id)
print(f"Status: {workflow.status}")
print(f"Created: {workflow.created_at}")
```

## Workflow Execution

### Starting Workflows

```python
# Start a created workflow
await client.start_workflow(workflow.id)

# Run decorator-based workflow (creates and starts)
execution = await client.run_workflow(
    workflow_instance,
    input_data={"param": "value"}
)
```

### Monitoring Execution

```python
# Wait for completion
result = await execution.wait_for_completion(timeout=300)

# Check status
if execution.status == WorkflowStatus.COMPLETED:
    print("Success!")

# Get execution metrics
metrics = await execution.get_metrics()
print(f"Duration: {metrics['duration']}")
print(f"Steps completed: {len(metrics['step_metrics'])}")

# Monitor with callback
def on_update(workflow, metrics):
    print(f"Status: {workflow.status}")
    print(f"Progress: {len(metrics['step_metrics'])} steps")

await execution.monitor(callback=on_update, interval=2.0)
```

### Getting Workflow Jobs

```python
# Get all agents deployed for a workflow
jobs = await client.get_workflow_jobs(workflow_id)
for job in jobs:
    print(f"Agent {job.name}: {job.status}")
```

## State Management

### Reading State

```python
# Get entire workflow state
state = await client.get_workflow_state(workflow_id)

# Get specific key
value = await client.get_workflow_state(workflow_id, key="result")
```

### Updating State

```python
# Update single key
await client.update_workflow_state(
    workflow_id,
    key="status",
    value="processing"
)

# Update multiple keys
for key, value in updates.items():
    await client.update_workflow_state(workflow_id, key, value)
```

## Agent Management

### Deploying Agents

```python
agent = await client.deploy_agent(
    name="worker-1",
    image="python:3.9-slim",
    env_vars={
        "TASK_TYPE": "process",
        "BATCH_SIZE": "100"
    },
    cpu_limit=2000000000,  # 2 CPU cores
    memory_limit=4294967296,  # 4GB
    workflow_id=workflow_id,  # Associate with workflow
    step_id=step_id
)

print(f"Deployed agent: {agent.id}")
```

### Managing Agents

```python
# Start agent
await client.start_agent(agent.id)

# Stop agent
await client.stop_agent(agent.id)

# Get agent details
agent = await client.get_agent(agent.id)
print(f"Status: {agent.status}")

# Get logs
logs = await client.get_agent_logs(agent.id, follow=False)
print(logs)
```

## MapReduce Pattern

### Simple MapReduce

```python
from agentainer_flow import MapReduceConfig

config = MapReduceConfig(
    name="data-analysis",
    mapper_image="analyzer:latest",
    reducer_image="aggregator:latest",
    max_parallel=20,
    pool_size=15,
    timeout="30m"
)

execution = await client.run_mapreduce(config)
result = await execution.wait_for_completion()
```

## Scheduling and Triggers

### Creating Schedule Triggers

```python
# Schedule workflow to run every hour
trigger = await client.schedule_workflow(
    workflow_id,
    cron_expression="0 * * * *",
    timezone="UTC",
    input_data={"scheduled": True}
)

print(f"Created trigger: {trigger.id}")
print(f"Next run: {trigger.next_run}")
```

### Managing Triggers

```python
# List triggers for a workflow
triggers = await client.list_triggers(workflow_id)

# Enable/disable triggers
await client.enable_trigger(trigger.id)
await client.disable_trigger(trigger.id)

# Manually trigger workflow
execution_id = await client.trigger_workflow(workflow_id)
```

### Event-Based Triggers

```python
from agentainer_flow import TriggerType, TriggerConfig

config = TriggerConfig(
    event_type="data_uploaded",
    event_filter={"bucket": "input-data"},
    skip_if_running=True
)

trigger = await client.create_trigger(
    workflow_id,
    TriggerType.EVENT,
    config
)
```

## Metrics and Monitoring

### Workflow Metrics

```python
# Get metrics for specific workflow
metrics = await client.get_workflow_metrics(workflow_id)

print(f"Status: {metrics['status']}")
print(f"Duration: {metrics.get('duration')}")
print(f"Resource usage: {metrics.get('resource_usage')}")

# Step-level metrics
for step_id, step_metrics in metrics['step_metrics'].items():
    print(f"Step {step_id}: {step_metrics['status']}")
    if step_metrics.get('agent_pooled'):
        print(f"  Used pooled agent")
```

### Historical Metrics

```python
# Get workflow history for last 24 hours
history = await client.get_workflow_history(duration="24h")

for workflow in history:
    print(f"{workflow['workflow_id']}: {workflow['status']}")
    print(f"  Started: {workflow['start_time']}")
    print(f"  Duration: {workflow.get('duration')}")
```

### Aggregate Metrics

```python
# Get aggregate metrics for last hour
aggregates = await client.get_aggregate_metrics(duration="1h")

print(f"Total workflows: {aggregates['total_workflows']}")
print(f"Success rate: {aggregates['success_rate']:.1f}%")
print(f"Average duration: {aggregates['avg_duration']}")
print(f"Pool efficiency: {aggregates['pool_efficiency']:.1f}%")
```

## Error Handling

```python
from agentainer_flow import ClientError, WorkflowError

try:
    workflow = await client.get_workflow(workflow_id)
except ClientError as e:
    print(f"API error: {e}")
except Exception as e:
    print(f"Unexpected error: {e}")

# Handle workflow execution errors
try:
    execution = await client.run_workflow(workflow_instance)
    result = await execution.wait_for_completion(timeout=300)
except WorkflowError as e:
    print(f"Workflow failed: {e}")
    # Get detailed metrics to debug
    metrics = await execution.get_metrics()
    for error in metrics.get('errors', []):
        print(f"  Error: {error}")
```

## Advanced Usage

### Custom Request Headers

```python
# Add custom headers to requests
client.session.headers.update({
    "X-Custom-Header": "value"
})
```

### Streaming Logs

```python
# Follow agent logs in real-time
import asyncio

async def stream_logs(agent_id):
    logs = await client.get_agent_logs(agent_id, follow=True)
    async for line in logs:
        print(line, end='')

# Run in background
asyncio.create_task(stream_logs(agent.id))
```

### Batch Operations

```python
# Deploy multiple agents concurrently
import asyncio

async def deploy_agents(count):
    tasks = []
    for i in range(count):
        task = client.deploy_agent(
            name=f"worker-{i}",
            image="worker:latest",
            workflow_id=workflow_id
        )
        tasks.append(task)
    
    agents = await asyncio.gather(*tasks)
    return agents

agents = await deploy_agents(10)
```

## Best Practices

1. **Use context managers**: Always use `async with` for automatic cleanup
2. **Handle timeouts**: Set appropriate timeouts for long-running operations
3. **Check status**: Verify workflow/agent status before operations
4. **Log errors**: Capture and log detailed error information
5. **Monitor metrics**: Use metrics to optimize workflow performance
6. **Batch when possible**: Use concurrent operations for better performance

## Next Steps

- Learn about [Testing Workflows](testing.md)
- Explore [Advanced Topics](advanced.md)
- See [API Reference](api-reference.md) for complete documentation
