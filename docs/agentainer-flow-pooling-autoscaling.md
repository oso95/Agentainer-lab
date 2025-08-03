# Agent Pooling and Auto-Scaling: Unified Design

## Overview

Agent pooling and auto-scaling are closely related features that share the same underlying infrastructure. Pooling provides warm, reusable agents while auto-scaling dynamically adjusts pool size based on demand.

## Core Pooling Infrastructure

### 1. Base Pool Manager

```go
type AgentPool struct {
    Name         string
    Image        string
    MinSize      int                    // Minimum agents to keep warm
    MaxSize      int                    // Maximum agents allowed
    IdleAgents   chan *PooledAgent      // Available agents
    ActiveAgents map[string]*PooledAgent // In-use agents
    Metrics      *PoolMetrics           // Usage statistics
}

type PooledAgent struct {
    Agent         *Agent
    LastUsed      time.Time
    UsageCount    int
    State         PooledAgentState
    Reserved      bool
}

type PoolMetrics struct {
    RequestsTotal     int64
    RequestsActive    int64
    QueueDepth        int64
    AvgWaitTime       time.Duration
    AvgExecutionTime  time.Duration
    UtilizationRate   float64
}
```

### 2. Pool Lifecycle

```go
func (p *AgentPool) GetAgent(ctx context.Context) (*PooledAgent, error) {
    select {
    case agent := <-p.IdleAgents:
        // Reuse warm agent
        agent.State = StateActive
        agent.LastUsed = time.Now()
        p.ActiveAgents[agent.Agent.ID] = agent
        return agent, nil
        
    case <-time.After(100 * time.Millisecond):
        // No idle agents available
        if len(p.ActiveAgents) < p.MaxSize {
            // Create new agent if under limit
            return p.createNewAgent(ctx)
        }
        // Queue request if at capacity
        return nil, ErrPoolAtCapacity
    }
}

func (p *AgentPool) ReleaseAgent(agent *PooledAgent) {
    agent.State = StateIdle
    agent.UsageCount++
    
    // Return to pool for reuse
    select {
    case p.IdleAgents <- agent:
        // Successfully returned to pool
    default:
        // Pool is full, terminate agent
        p.terminateAgent(agent)
    }
}
```

## Use Case 1: Parallel Task Pooling

For MapReduce and parallel workflows:

```python
@step(
    parallel=True,
    execution_mode="pooled",
    pool_config={
        "min_size": 2,      # Always keep 2 warm
        "max_size": 10,     # Never exceed 10
        "idle_timeout": 300  # Terminate after 5min idle
    }
)
async def process_item(self, ctx: Context, item: Dict):
    # Each parallel execution gets an agent from the pool
    # Agent is returned to pool after completion
    # Pool maintains 2-10 agents based on workload
```

### Benefits for Parallel Tasks:
- **Fast startup**: 0.1s from pool vs 2-5s cold start
- **Resource efficiency**: 10 tasks might use only 5 agents
- **Cost savings**: Fewer containers running simultaneously

## Use Case 2: Auto-Scaling for Sequential Workflows

For non-parallel workflows that need to scale based on load:

```python
@workflow(
    name="api_processor",
    scaling_policy={
        "mode": "auto",
        "min_instances": 1,      # Always have 1 ready
        "max_instances": 20,     # Scale up to 20
        "target_utilization": 0.7,
        "scale_up_threshold": 0.8,
        "scale_down_threshold": 0.3,
        "cooldown_period": 60
    }
)
class APIProcessor:
    @step(name="process_request")
    async def process(self, ctx: Context, request: Dict):
        # Single-threaded processing
        # But multiple workflow instances can run in parallel
        result = await heavy_computation(request)
        return result
```

### Auto-Scaling Triggers:

```go
type AutoScaler struct {
    Pool            *AgentPool
    ScalingPolicy   ScalingPolicy
    MetricsWindow   time.Duration
}

func (as *AutoScaler) evaluate() ScalingDecision {
    metrics := as.Pool.Metrics
    
    // Scale UP conditions
    if metrics.UtilizationRate > as.ScalingPolicy.ScaleUpThreshold {
        return ScaleUp
    }
    if metrics.QueueDepth > 0 && metrics.AvgWaitTime > 1*time.Second {
        return ScaleUp
    }
    
    // Scale DOWN conditions
    if metrics.UtilizationRate < as.ScalingPolicy.ScaleDownThreshold {
        return ScaleDown
    }
    
    return NoChange
}

func (as *AutoScaler) scale(decision ScalingDecision) {
    switch decision {
    case ScaleUp:
        newSize := min(as.Pool.Size + 1, as.Pool.MaxSize)
        as.Pool.Resize(newSize)
        
    case ScaleDown:
        newSize := max(as.Pool.Size - 1, as.Pool.MinSize)
        as.Pool.Resize(newSize)
    }
}
```

## Unified Implementation

### 1. Shared Pool Infrastructure

Both use cases share the same pool manager:

```go
// For parallel workflows
parallelPool := &AgentPool{
    Name:    "mapreduce-workers",
    Image:   "processor:latest",
    MinSize: 2,
    MaxSize: 10,
}

// For auto-scaled sequential workflows  
apiPool := &AgentPool{
    Name:    "api-processors",
    Image:   "api-handler:latest",
    MinSize: 1,
    MaxSize: 20,
}

// Both managed by same PoolManager
poolManager.RegisterPool(parallelPool)
poolManager.RegisterPool(apiPool)
```

### 2. Different Scaling Behaviors

```python
# Parallel: Pool shared across parallel executions
@mapreduce(pool_size=5, max_parallel=20)
# 5 agents process 20 tasks in batches

# Sequential: Pool provides multiple workflow instances
@workflow(scaling_policy={"min": 1, "max": 10})
# 1-10 separate workflow instances based on load
```

### 3. Metrics-Driven Scaling

```yaml
# Common metrics for both patterns
pool_metrics:
  - utilization_rate     # Active/Total agents
  - queue_depth         # Waiting requests
  - avg_response_time   # Task completion time
  - throughput          # Tasks/second
  
scaling_rules:
  scale_up:
    - utilization > 80%
    - queue_depth > 5
    - response_time > SLA
    
  scale_down:
    - utilization < 30%
    - queue_depth == 0
    - idle_time > 5min
```

## Configuration Examples

### 1. High-Volume API Processing
```python
@workflow(
    scaling_policy={
        "mode": "aggressive",
        "min_instances": 5,
        "max_instances": 50,
        "scale_up_rate": 5,      # Add 5 at a time
        "scale_down_rate": 1,    # Remove 1 at a time
        "predictor": "ml-based"  # Use ML for prediction
    }
)
```

### 2. Bursty Batch Processing
```python
@workflow(
    scaling_policy={
        "mode": "reactive",
        "min_instances": 0,      # Scale to zero
        "max_instances": 100,
        "burst_capacity": 20,    # Quick scale for bursts
        "idle_terminate": 120    # Terminate after 2min
    }
)
```

### 3. Cost-Optimized Pipeline
```python
@workflow(
    scaling_policy={
        "mode": "conservative",
        "min_instances": 1,
        "max_instances": 5,
        "cost_threshold": 100,   # Max $/hour
        "prefer_spot": True      # Use spot instances
    }
)
```

## Implementation Phases

### Phase 1: Basic Pooling (Week 1)
- Fixed-size pools for parallel tasks
- Simple round-robin assignment
- Basic health checks

### Phase 2: Dynamic Pooling (Week 2)
- Resize pools based on queue depth
- Idle timeout and cleanup
- Connection pooling patterns

### Phase 3: Auto-Scaling (Week 3)
- Metrics collection and analysis
- Rule-based scaling decisions
- Smooth scaling (no thrashing)

### Phase 4: Advanced Features (Future)
- Predictive scaling with ML
- Cross-region pools
- Spot instance integration
- Cost optimization

## Benefits of Unified Approach

1. **Code Reuse**: Same pool infrastructure for both patterns
2. **Consistent Behavior**: Similar configuration and monitoring
3. **Flexibility**: Easy to switch between patterns
4. **Cost Efficiency**: Optimal resource utilization for all workloads

## Example: Real-World Scenario

```python
@workflow(name="data_pipeline")
class DataPipeline:
    # Stage 1: Single API call (auto-scaled)
    @step(
        scaling_policy={"min": 1, "max": 5}
    )
    async def fetch_data_list(self, ctx: Context):
        # Multiple instances if queue builds up
        pass
    
    # Stage 2: Parallel processing (pooled)
    @step(
        parallel=True,
        pool_size=10,
        max_parallel=100
    )
    async def process_item(self, ctx: Context, item: Dict):
        # 10 agents process 100 items
        pass
    
    # Stage 3: Single aggregation (no scaling needed)
    @step()
    async def aggregate(self, ctx: Context, results: List):
        # Single instance is sufficient
        pass
```

This unified approach provides maximum flexibility while maintaining simplicity.