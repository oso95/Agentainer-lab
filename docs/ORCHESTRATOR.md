# Agentainer Orchestrator Documentation

The Orchestrator is the core component that manages workflow execution in Agentainer. It coordinates the deployment and execution of containerized agents according to workflow definitions.

## 🏗️ Architecture Overview

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│ Workflow Manager│────▶│ Orchestrator │────▶│Agent Manager│
└─────────────────┘     └──────────────┘     └─────────────┘
         │                      │                     │
         │                      ▼                     ▼
         │              ┌──────────────┐      ┌─────────────┐
         └─────────────▶│    Redis     │      │   Docker    │
                        └──────────────┘      └─────────────┘
```

## 📋 Core Components

### 1. Orchestrator (`orchestrator.go`)
The main coordinator that:
- Executes workflows step by step
- Manages step dependencies
- Handles retries and failures
- Coordinates parallel execution
- Manages workflow state

### 2. State Manager
- Stores workflow state in Redis
- Manages inter-step data sharing
- Tracks execution progress
- Handles task results

### 3. Agent Monitor
- Tracks agent health
- Manages agent lifecycle
- Handles cleanup policies
- Monitors resource usage

### 4. Pool Manager
- Manages pre-warmed agent pools
- Handles agent allocation
- Optimizes performance for repeated tasks

### 5. Metrics Collector
- Collects performance metrics
- Tracks execution times
- Monitors resource usage
- Generates reports

## 🔄 Workflow Execution Flow

### 1. Workflow Creation
```go
workflow := &Workflow{
    ID:     uuid.New().String(),
    Name:   "my-workflow",
    Status: "pending",
    Steps:  []WorkflowStep{...},
}
```

### 2. Step Execution Types

#### Sequential Steps
Execute one at a time in order:
```go
func (o *Orchestrator) executeSequentialStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
    // Deploy single agent
    agent, err := o.agentManager.DeployWithWorkflow(...)
    // Execute task
    // Wait for completion
    // Cleanup if needed
}
```

#### Parallel Steps
Execute multiple tasks simultaneously:
```go
func (o *Orchestrator) executeParallelSteps(ctx context.Context, workflow *Workflow, steps []*WorkflowStep) error {
    var wg sync.WaitGroup
    for _, step := range steps {
        wg.Add(1)
        go func(s *WorkflowStep) {
            defer wg.Done()
            o.executeStep(ctx, workflow, s)
        }(step)
    }
    wg.Wait()
}
```

#### Map Steps
Dynamic parallel execution over arrays:
```go
func (o *Orchestrator) executeMapStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
    // Extract array from state
    items := o.extractMapInput(workflow.State, mapConfig.InputPath)
    
    // Create parallel tasks for each item
    for i, item := range items {
        mapStep := o.createMapStepInstance(step, i, item)
        go o.executeStep(ctx, workflow, mapStep)
    }
}
```

## 🔧 Key Features

### 1. Retry Mechanisms
```yaml
retry_policy:
  max_attempts: 3
  backoff: exponential  # constant, linear, exponential
  delay: 5s
```

Backoff calculations:
- **Constant**: `delay`
- **Linear**: `delay * attempt`
- **Exponential**: `delay * (2^(attempt-1))`

### 2. Cleanup Policies
```yaml
cleanup_policy: "on_success"  # always, on_success, never
```

The orchestrator determines cleanup based on:
```go
func (o *Orchestrator) shouldCleanupAgent(workflow *Workflow, step *WorkflowStep, stepFailed bool) bool {
    // Check if retries pending
    if stepFailed && hasRetriesRemaining(step) {
        return false  // Keep for retry
    }
    
    // Apply cleanup policy
    switch workflow.Config.CleanupPolicy {
    case "always":
        return true
    case "on_success":
        return !stepFailed
    case "never":
        return false
    }
}
```

### 3. State Management

#### Workflow State Storage
```go
// State stored in Redis hash
key := fmt.Sprintf("workflow:%s:state", workflowID)
redis.HSet(key, field, value)
```

#### Task Communication
```go
// Task writes result
redis.Set("task:{id}:result", result)
redis.Publish("task:{id}:complete", "completed")

// Orchestrator listens
sub := redis.Subscribe("task:*:complete")
```

### 4. Map Step Configuration
```json
{
  "map_config": {
    "input_path": "urls",           // Path to array in state
    "item_alias": "current_url",    // Variable name for item
    "max_concurrency": 5,           // Parallel limit
    "error_handling": "continue_on_error"
  }
}
```

### 5. Resource Management
```yaml
resource_limits:
  cpu_limit: 500000000    # 0.5 CPU (in nanoseconds)
  memory_limit: 268435456 # 256MB (in bytes)
  gpu_limit: 1            # Number of GPUs
```

## 📊 Performance Optimization

### Agent Pooling
Pre-warm containers for frequently used images:
```yaml
pool_config:
  min_size: 2
  max_size: 10
  idle_timeout: 5m
  warm_up: true
```

### Performance Profiling
```go
if workflow.Config.EnableProfiling {
    o.performanceProfiler.StartProfiling(workflowID)
    defer o.performanceProfiler.StopProfiling(workflowID)
}
```

## 🛠️ Advanced Features

### 1. Conditional Execution
```yaml
condition:
  field: "$.status"
  operator: "eq"
  value: "success"
```

### 2. Sub-workflows
```yaml
sub_workflow_id: "data-processing-workflow"
```

### 3. Decision Nodes
```yaml
decision_node:
  input_path: "$.result"
  choices:
    - condition:
        field: "$.status"
        operator: "eq"
        value: "success"
      next: "success-handler"
    - condition:
        field: "$.status"
        operator: "eq"
        value: "error"
      next: "error-handler"
  default: "default-handler"
```

## 🔍 Debugging Orchestrator Issues

### 1. Check Workflow State
```bash
redis-cli HGETALL workflow:{id}:state
```

### 2. Monitor Task Completion
```bash
redis-cli SUBSCRIBE "task:*:complete"
```

### 3. View Orchestrator Logs
```bash
docker logs agentainer-server | grep Orchestrator
```

### 4. Check Step Status
```bash
curl http://localhost:8081/workflows/{id} | jq '.data.steps'
```

## 🎯 Best Practices

### 1. Design Idempotent Steps
- Steps should be safe to retry
- Use unique IDs for operations
- Check for existing results before processing

### 2. Handle Partial Failures
- Use `failure_strategy: continue` for resilience
- Aggregate both successes and failures
- Report comprehensive results

### 3. Optimize Resource Usage
- Set appropriate resource limits
- Use agent pooling for repeated tasks
- Clean up resources promptly

### 4. Monitor Performance
- Enable profiling for complex workflows
- Track execution times
- Monitor resource consumption

## 🔗 Related Documentation

- [Workflow Architecture](./WORKFLOW_ARCHITECTURE.md) - Container lifecycle in workflows
- [API Endpoints](./API_ENDPOINTS.md) - Workflow API reference
- [MapReduce Example](../examples/mapreduce-workflow/) - Complete orchestrator example