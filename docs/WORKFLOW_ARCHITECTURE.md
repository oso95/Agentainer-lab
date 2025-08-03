# How Workflows Work with Agent Containers in Agentainer

## Overview

Workflows in Agentainer orchestrate multiple agent containers to complete complex tasks. Each workflow step deploys and manages one or more agent containers.

## Key Concepts

### 1. One Step = One or More Containers

- **Sequential Steps**: Deploy a single agent container
- **Parallel Steps**: Deploy multiple agent containers (up to `max_workers`)
- **Pooled Steps**: Reuse existing containers from an agent pool

### 2. Workflow Execution Flow

```
Workflow Started
    │
    ├─→ Step 1 (Sequential) → Deploy Agent Container 1
    │                        → Execute Command
    │                        → Capture Results
    │                        → Stop/Remove Container
    │
    ├─→ Step 2 (Parallel)   → Deploy Agent Container 2
    │                       → Deploy Agent Container 3
    │                       → Deploy Agent Container 4
    │                       → Execute in Parallel
    │                       → Collect Results
    │
    └─→ Step 3 (Sequential) → Deploy Agent Container 5
                            → Process Results
                            → Complete Workflow
```

## Example: MapReduce Workflow

When you run a MapReduce workflow:

1. **Initialize Step**
   - Deploys 1 container (e.g., `alpine:latest`)
   - Runs initialization command
   - Container name: `Simple MapReduce Word Count-Initialize MapReduce`

2. **Map Steps** (Parallel)
   - Deploys 5 containers (one for each chunk)
   - Container names: `Simple MapReduce Word Count-Map Chunk 1-0` through `-4`
   - All run in parallel

3. **Reduce Step**
   - Deploys 1 container
   - Aggregates results from map steps
   - Container name: `Simple MapReduce Word Count-Reduce Results`

4. **Finalize Step**
   - Deploys 1 container
   - Generates final report
   - Container name: `Simple MapReduce Word Count-Generate Report`

## Container Lifecycle in Workflows

1. **Deploy**: Container is created with workflow metadata
   - Workflow ID is attached to the agent
   - Step ID is attached to the agent
   - Status: "created"

2. **Start**: Container begins executing
   - Status: "running"
   - Command is executed inside container

3. **Complete**: Container finishes execution
   - Results are captured
   - Status: "stopped"
   - Container can be removed or kept for debugging

## Agent Pooling (Performance Optimization)

For frequently used images, Agentainer can maintain a pool of pre-warmed containers:

```yaml
pool_config:
  min_size: 2
  max_size: 10
  warm_up: true
```

Instead of deploying new containers for each step:
1. Request agent from pool
2. Execute task
3. Return agent to pool
4. Agent is reset and ready for next task

This provides 20-50x performance improvement for short-lived tasks.

## State Management

- **Workflow State**: Stored in Redis
- **Agent State**: Tracked by Agent Manager
- **Step Results**: Passed between steps via workflow state
- **Container Logs**: Available via Docker API

## Example Code

Here's how a workflow step deploys an agent:

```go
// Sequential step - one container
agent, err := o.agentManager.DeployWithWorkflow(
    ctx,
    fmt.Sprintf("%s-%s", workflow.Name, step.Name),
    step.Config.Image,
    step.Config.EnvVars,
    step.Config.ResourceLimits.CPULimit,
    step.Config.ResourceLimits.MemoryLimit,
    false, // auto-restart
    "",    // token
    nil,   // ports
    nil,   // volumes
    nil,   // health check
    workflow.ID,
    step.ID,
)
```

## Benefits of This Architecture

1. **Isolation**: Each step runs in its own container
2. **Scalability**: Parallel steps can scale horizontally
3. **Flexibility**: Different images/environments per step
4. **Fault Tolerance**: Failed steps don't affect others
5. **Resource Control**: Per-step resource limits
6. **Debugging**: Each container can be inspected independently