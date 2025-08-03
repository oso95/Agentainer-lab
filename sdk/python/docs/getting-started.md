# Getting Started with Agentainer Flow SDK

## Installation

### Prerequisites

- Python 3.7 or higher
- Agentainer services running (see [main documentation](../../../README.md))
- Docker installed and running

### Install from PyPI

```bash
pip install agentainer-flow
```

### Install from Source

```bash
git clone https://github.com/oso95/Agentainer-lab.git
cd Agentainer-lab/sdk/python
pip install -e .
```

### Install with Development Dependencies

```bash
pip install -e ".[dev]"
```

## Quick Start

### 1. Start Agentainer Services

```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:alpine

# Start Agentainer
agentainer start
```

### 2. Create Your First Workflow

```python
import asyncio
from agentainer_flow import workflow, step, WorkflowClient

@workflow("hello-world")
class HelloWorldWorkflow:
    """A simple hello world workflow"""
    
    @step()
    async def greet(self, ctx):
        """Generate a greeting"""
        name = ctx.state.get("name", "World")
        greeting = f"Hello, {name}!"
        ctx.state["greeting"] = greeting
        print(greeting)
        return greeting
    
    @step(depends_on=["greet"])
    async def farewell(self, ctx):
        """Say goodbye"""
        name = ctx.state.get("name", "World")
        farewell_msg = f"Goodbye, {name}!"
        ctx.state["farewell"] = farewell_msg
        print(farewell_msg)
        return farewell_msg

async def main():
    # Create workflow client
    async with WorkflowClient() as client:
        # Run the workflow
        workflow_instance = HelloWorldWorkflow()
        execution = await client.run_workflow(
            workflow_instance,
            input_data={"name": "Alice"}
        )
        
        # Wait for completion
        result = await execution.wait_for_completion()
        print(f"Workflow completed: {result}")

if __name__ == "__main__":
    asyncio.run(main())
```

### 3. Run the Workflow

```bash
python hello_world.py
```

## Basic Concepts

### Workflows

A workflow is a collection of steps that execute in a defined order. Workflows are defined using the `@workflow` decorator on a class.

```python
@workflow("my-workflow", description="Process data pipeline")
class DataPipeline:
    pass
```

### Steps

Steps are individual units of work within a workflow. They are defined as methods decorated with `@step`.

```python
@step(name="process-data")
async def process(self, ctx):
    # Step logic here
    pass
```

### Context

The context (`ctx`) provides access to:
- **State**: Shared data between steps
- **Workflow metadata**: IDs, configuration
- **Agent deployment**: Deploy containerized agents

```python
@step()
async def deploy_worker(self, ctx):
    # Access state
    input_data = ctx.state["input"]
    
    # Deploy an agent
    agent = await ctx.deploy_agent(
        name="worker",
        image="python:3.9",
        env_vars={"TASK": "process"}
    )
    
    # Update state
    ctx.state["worker_id"] = agent.id
```

### Dependencies

Steps can depend on other steps, creating an execution graph:

```python
@step()
async def step_a(self, ctx):
    pass

@step(depends_on=["step_a"])
async def step_b(self, ctx):
    # Executes after step_a completes
    pass
```

### Parallel Execution

Steps can run in parallel for improved performance:

```python
@step(parallel=True, max_workers=10)
async def process_batch(self, ctx):
    # This step will run with up to 10 parallel instances
    task_id = ctx.task_id  # Unique ID for this parallel instance
    pass
```

## Next Steps

- Learn about [Workflow Decorators](decorators.md)
- Explore the [Workflow Client API](client.md)
- See more [Examples](../examples/)
- Read about [Testing Workflows](testing.md)
