# Agentainer Flow - Workflow Orchestration

Agentainer Flow is the workflow orchestration layer that enables complex multi-step workflows with parallel execution, state management, and performance optimization.

## Overview

Agentainer Flow extends the core agent management capabilities to support:
- **Multi-step workflows** with dependencies
- **Parallel execution** with fan-out/fan-in patterns
- **State persistence** across workflow steps
- **Performance optimization** through agent pooling
- **Advanced patterns** like MapReduce and DAGs

## Core Concepts

### 1. Workflows
A workflow is a collection of steps that execute in a defined order:
```yaml
name: data-processing-workflow
steps:
  - name: prepare-data
    type: sequential
  - name: process-data
    type: map  # Parallel execution
  - name: aggregate-results
    type: reduce
```

### 2. Step Types
- **Sequential**: Execute one task
- **Parallel**: Execute multiple tasks concurrently
- **Map**: Dynamic parallel execution over arrays
- **Reduce**: Aggregate results from parallel tasks
- **Branch**: Conditional execution paths
- **Decision**: Multi-way branching

### 3. State Management
All workflow state is stored in Redis and shared across steps:
```python
# In one step
workflow.state.set("processed_items", items)

# In another step
items = workflow.state.get("processed_items")
```

### 4. Agent Pooling
Pre-warmed containers provide 20-50x performance improvement:
```yaml
pool_config:
  min_size: 2
  max_size: 10
  warm_up: true
```

## Key Features

### 🚀 Performance
- **Agent pooling** reduces startup time from 2-5s to ~0.1s
- **Parallel execution** with configurable concurrency limits
- **Resource management** with per-step CPU/memory limits

### 🔄 Reliability
- **Automatic retry** with exponential backoff
- **Error handling** with continue-on-error policies
- **Cleanup policies** for failed agents

### 📊 Observability
- **Real-time monitoring** of workflow progress
- **Comprehensive metrics** collection
- **Performance profiling** for optimization

## Quick Example

### MapReduce Word Counter
```python
# Run the example
cd examples/mapreduce-workflow
./build.sh
python3 run_workflow_api.py
```

This demonstrates:
- List phase to prepare URLs
- Parallel map phase to process each URL
- Reduce phase to aggregate results

## Architecture

For detailed architecture and implementation details, see:
- [Orchestrator Documentation](./ORCHESTRATOR.md) - Core orchestration engine
- [Workflow Architecture](./WORKFLOW_ARCHITECTURE.md) - Container lifecycle management

## Getting Started

1. **Define your workflow** using YAML or the API
2. **Build agent images** for each step
3. **Deploy the workflow** using Agentainer CLI or API
4. **Monitor execution** through the dashboard or API

## Examples

See the [examples](../examples/) directory for complete working examples:
- `mapreduce-workflow/` - MapReduce pattern implementation
- `workflow-demo/` - Multi-agent AI workflow

## Best Practices

1. **Design idempotent steps** - Safe to retry
2. **Use state wisely** - Store only necessary data
3. **Set resource limits** - Prevent resource exhaustion
4. **Enable monitoring** - Track performance metrics
5. **Handle failures gracefully** - Use error policies

## API Reference

See [API Endpoints](./API_ENDPOINTS.md) for the complete workflow API reference.