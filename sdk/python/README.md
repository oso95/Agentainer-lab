# Agentainer Flow Python SDK

The official Python SDK for Agentainer Flow - workflow orchestration for LLM agents.

## Installation

```bash
pip install agentainer-flow
```

For development:
```bash
pip install -e .[dev]
```

## Quick Start

```python
from agentainer_flow import workflow, step, Context

@workflow(name="data_pipeline", timeout="2h")
class DataPipeline:
    
    @step(name="fetch_data")
    async def fetch_data(self, ctx: Context):
        # Deploy an agent to fetch data
        agent = await ctx.deploy_agent(
            name="fetcher",
            image="data-fetcher:latest",
            env={"SOURCE": "api.example.com"}
        )
        result = await agent.wait()
        ctx.state["data"] = result
        return result
    
    @step(name="process", parallel=True, max_workers=5)
    async def process_data(self, ctx: Context, item):
        # Process each item in parallel
        agent = await ctx.deploy_agent(
            name=f"processor-{item['id']}",
            image="processor:latest"
        )
        return await agent.wait()
    
    @step(name="aggregate", reduce=True)
    async def aggregate_results(self, ctx: Context, results):
        # Aggregate all results
        return {"total": len(results), "data": results}

# Run the workflow
async def main():
    from agentainer_flow import WorkflowClient
    
    client = WorkflowClient("http://localhost:8081")
    workflow = DataPipeline()
    
    execution = await client.run_workflow(workflow)
    result = await execution.wait_for_completion()
    print(f"Workflow completed: {result}")
```

## Features

- **Decorators**: Define workflows with simple Python decorators
- **Type Safety**: Full type hints and IDE support
- **Async/Await**: Modern async Python patterns
- **State Management**: Built-in workflow state persistence
- **Parallel Execution**: Easy parallel and MapReduce patterns
- **Testing**: Local testing without deploying agents

## Documentation

See the [full documentation](https://github.com/oso95/Agentainer-lab/docs/AGENTAINER_FLOW.md) for more examples and advanced usage.