# Agentainer Flow State Persistence Strategy

## Overview

State persistence is crucial for Agentainer Flow to support long-running workflows, fault tolerance, and data sharing between workflow steps. This document outlines the strategy for implementing a robust, scalable state management system.

## Design Principles

1. **Workflow Isolation**: Each workflow has its own isolated state namespace
2. **Consistency**: Use Redis transactions for atomic state updates
3. **Performance**: Optimize for read-heavy workloads with appropriate caching
4. **Fault Tolerance**: State survives agent crashes and system restarts
5. **Scalability**: Support large state objects and high-throughput workflows

## State Architecture

### 1. State Hierarchy

```
Workflow State
├── Workflow Metadata (immutable after creation)
├── Workflow Runtime State (mutable, workflow-scoped)
├── Step State (mutable, step-scoped)
├── Parallel Task State (mutable, task-scoped)
└── Shared State Store (cross-workflow sharing)
```

### 2. Redis Key Structure

```
# Workflow metadata (hash)
workflow:{workflow_id}:meta
  - id: string
  - name: string
  - version: string
  - created_at: timestamp
  - config: json

# Workflow execution state (hash)
workflow:{workflow_id}:exec:{execution_id}:state
  - status: string (pending|running|completed|failed|paused)
  - started_at: timestamp
  - updated_at: timestamp
  - error: string (optional)
  - result: json (optional)

# Workflow runtime state (hash)
workflow:{workflow_id}:exec:{execution_id}:data
  - user-defined key-value pairs

# Step state (hash)
workflow:{workflow_id}:exec:{execution_id}:step:{step_id}:state
  - status: string
  - started_at: timestamp
  - completed_at: timestamp
  - result: json
  - error: string (optional)

# Parallel task state (hash)
workflow:{workflow_id}:exec:{execution_id}:step:{step_id}:task:{task_id}:state
  - Similar to step state

# State operations log (stream)
workflow:{workflow_id}:exec:{execution_id}:state:log
  - Audit trail of all state changes

# State TTL tracking (sorted set)
workflow:state:ttl
  - Score: expiration timestamp
  - Member: state key
```

### 3. State Management Components

#### StateManager Interface

```go
type StateManager interface {
    // Workflow state operations
    CreateWorkflowState(ctx context.Context, workflowID, executionID string) error
    GetWorkflowState(ctx context.Context, workflowID, executionID string) (*WorkflowState, error)
    UpdateWorkflowState(ctx context.Context, workflowID, executionID string, updates map[string]interface{}) error
    
    // Step state operations
    CreateStepState(ctx context.Context, workflowID, executionID, stepID string) error
    UpdateStepState(ctx context.Context, workflowID, executionID, stepID string, state StepState) error
    
    // Data operations
    SetData(ctx context.Context, workflowID, executionID, key string, value interface{}) error
    GetData(ctx context.Context, workflowID, executionID, key string) (interface{}, error)
    GetAllData(ctx context.Context, workflowID, executionID string) (map[string]interface{}, error)
    
    // Atomic operations
    IncrementCounter(ctx context.Context, workflowID, executionID, key string, delta int64) (int64, error)
    AppendToList(ctx context.Context, workflowID, executionID, key string, values ...interface{}) error
    AddToSet(ctx context.Context, workflowID, executionID, key string, members ...interface{}) error
    
    // Cleanup
    CleanupWorkflowState(ctx context.Context, workflowID, executionID string) error
}
```

#### State Storage Implementation

```go
type RedisStateManager struct {
    client      *redis.Client
    serializer  Serializer
    ttlDuration time.Duration
}

func (m *RedisStateManager) SetData(ctx context.Context, workflowID, executionID, key string, value interface{}) error {
    stateKey := fmt.Sprintf("workflow:%s:exec:%s:data", workflowID, executionID)
    
    // Serialize value
    serialized, err := m.serializer.Serialize(value)
    if err != nil {
        return fmt.Errorf("failed to serialize value: %w", err)
    }
    
    // Use pipeline for atomic operations
    pipe := m.client.Pipeline()
    
    // Set the data
    pipe.HSet(ctx, stateKey, key, serialized)
    
    // Update TTL
    pipe.Expire(ctx, stateKey, m.ttlDuration)
    
    // Log the operation
    logKey := fmt.Sprintf("workflow:%s:exec:%s:state:log", workflowID, executionID)
    logEntry := map[string]interface{}{
        "operation": "set_data",
        "key":       key,
        "timestamp": time.Now().Unix(),
    }
    pipe.XAdd(ctx, &redis.XAddArgs{
        Stream: logKey,
        Values: logEntry,
    })
    
    // Execute pipeline
    _, err = pipe.Exec(ctx)
    return err
}
```

### 4. State Persistence Patterns

#### Pattern 1: Accumulator Pattern

For aggregating results from parallel tasks:

```python
# In workflow step
async def aggregate_results(self, ctx: Context, parallel_results: List[Dict]):
    # Initialize accumulator if needed
    if "total_processed" not in ctx.state:
        ctx.state["total_processed"] = 0
    
    # Aggregate results
    for result in parallel_results:
        ctx.state["total_processed"] += result["count"]
        
        # Thread-safe list append
        await ctx.state.append_to_list("processed_files", result["file"])
        
        # Thread-safe set operations
        await ctx.state.add_to_set("unique_errors", *result.get("errors", []))
```

#### Pattern 2: Checkpoint Pattern

For long-running operations with progress tracking:

```python
async def process_large_dataset(self, ctx: Context):
    total_items = ctx.input["total_items"]
    batch_size = 1000
    
    # Resume from checkpoint if exists
    processed = ctx.state.get("items_processed", 0)
    
    for batch_start in range(processed, total_items, batch_size):
        # Process batch
        batch_end = min(batch_start + batch_size, total_items)
        await process_batch(batch_start, batch_end)
        
        # Update checkpoint
        ctx.state["items_processed"] = batch_end
        ctx.state["last_checkpoint"] = datetime.now().isoformat()
        
        # Persist state immediately
        await ctx.state.flush()
```

#### Pattern 3: Shared State Pattern

For workflows that need to share state:

```python
# Producer workflow
@workflow(name="data_producer")
class ProducerWorkflow:
    @step(name="produce")
    async def produce_data(self, ctx: Context):
        result = await generate_data()
        
        # Share state with other workflows
        await ctx.shared_state.set(
            "dataset:latest",
            result,
            ttl="1h"
        )

# Consumer workflow
@workflow(name="data_consumer")
class ConsumerWorkflow:
    @step(name="consume")
    async def consume_data(self, ctx: Context):
        # Read shared state
        data = await ctx.shared_state.get("dataset:latest")
        if data:
            await process_data(data)
```

### 5. State Lifecycle Management

#### State Creation

```go
func (m *WorkflowManager) StartWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*WorkflowExecution, error) {
    executionID := uuid.New().String()
    
    // Create workflow state atomically
    pipe := m.redis.Pipeline()
    
    // Set workflow metadata
    metaKey := fmt.Sprintf("workflow:%s:meta", workflowID)
    pipe.HSet(ctx, metaKey, map[string]interface{}{
        "id":         workflowID,
        "created_at": time.Now().Unix(),
        // ... other metadata
    })
    
    // Initialize execution state
    execKey := fmt.Sprintf("workflow:%s:exec:%s:state", workflowID, executionID)
    pipe.HSet(ctx, execKey, map[string]interface{}{
        "status":     "running",
        "started_at": time.Now().Unix(),
    })
    
    // Initialize data store
    dataKey := fmt.Sprintf("workflow:%s:exec:%s:data", workflowID, executionID)
    pipe.HSet(ctx, dataKey, "initialized", "true")
    
    // Set TTL
    pipe.Expire(ctx, execKey, 24*time.Hour)
    pipe.Expire(ctx, dataKey, 24*time.Hour)
    
    _, err := pipe.Exec(ctx)
    return &WorkflowExecution{ID: executionID}, err
}
```

#### State Cleanup

```go
func (m *WorkflowManager) CleanupWorkflow(ctx context.Context, workflowID, executionID string) error {
    pattern := fmt.Sprintf("workflow:%s:exec:%s:*", workflowID, executionID)
    
    // Get all keys for this execution
    keys, err := m.redis.Keys(ctx, pattern).Result()
    if err != nil {
        return err
    }
    
    // Archive state before deletion (optional)
    if m.config.ArchiveEnabled {
        if err := m.archiveState(ctx, keys); err != nil {
            log.Printf("Failed to archive state: %v", err)
        }
    }
    
    // Delete all keys
    if len(keys) > 0 {
        return m.redis.Del(ctx, keys...).Err()
    }
    
    return nil
}
```

### 6. Performance Optimizations

#### Batch Operations

```go
// Batch read for multiple keys
func (m *RedisStateManager) GetMultipleData(ctx context.Context, workflowID, executionID string, keys []string) (map[string]interface{}, error) {
    stateKey := fmt.Sprintf("workflow:%s:exec:%s:data", workflowID, executionID)
    
    // Use HMGET for batch read
    values, err := m.client.HMGet(ctx, stateKey, keys...).Result()
    if err != nil {
        return nil, err
    }
    
    result := make(map[string]interface{})
    for i, key := range keys {
        if values[i] != nil {
            var decoded interface{}
            if err := m.serializer.Deserialize(values[i].(string), &decoded); err == nil {
                result[key] = decoded
            }
        }
    }
    
    return result, nil
}
```

#### State Caching

```python
class CachedStateManager:
    """In-memory cache for frequently accessed state."""
    
    def __init__(self, redis_manager, cache_size=1000, ttl=60):
        self.redis = redis_manager
        self.cache = LRUCache(maxsize=cache_size)
        self.ttl = ttl
    
    async def get(self, workflow_id: str, execution_id: str, key: str):
        cache_key = f"{workflow_id}:{execution_id}:{key}"
        
        # Check cache first
        if cache_key in self.cache:
            entry = self.cache[cache_key]
            if time.time() < entry["expires"]:
                return entry["value"]
        
        # Fetch from Redis
        value = await self.redis.get(workflow_id, execution_id, key)
        
        # Update cache
        self.cache[cache_key] = {
            "value": value,
            "expires": time.time() + self.ttl
        }
        
        return value
```

### 7. Fault Tolerance

#### State Replication

```yaml
# Redis configuration for state persistence
redis:
  mode: sentinel  # or cluster
  master_name: agentainer-state
  sentinels:
    - host: sentinel1
      port: 26379
    - host: sentinel2
      port: 26379
    - host: sentinel3
      port: 26379
  
  # Enable persistence
  save:
    - 900 1     # Save after 900 sec if at least 1 key changed
    - 300 10    # Save after 300 sec if at least 10 keys changed
    - 60 10000  # Save after 60 sec if at least 10000 keys changed
  
  # Enable AOF for better durability
  appendonly: yes
  appendfsync: everysec
```

#### State Recovery

```go
func (m *WorkflowManager) RecoverWorkflow(ctx context.Context, workflowID, executionID string) error {
    // Check if workflow state exists
    stateKey := fmt.Sprintf("workflow:%s:exec:%s:state", workflowID, executionID)
    exists, err := m.redis.Exists(ctx, stateKey).Result()
    if err != nil {
        return err
    }
    
    if exists == 0 {
        return ErrWorkflowNotFound
    }
    
    // Get current state
    state, err := m.redis.HGetAll(ctx, stateKey).Result()
    if err != nil {
        return err
    }
    
    // Determine recovery point
    lastStep := state["last_completed_step"]
    
    // Resume from last completed step
    return m.ResumeFromStep(ctx, workflowID, executionID, lastStep)
}
```

### 8. Monitoring and Metrics

```go
// State metrics collector
type StateMetrics struct {
    StateSize        prometheus.GaugeVec
    StateOperations  prometheus.CounterVec
    StateLatency     prometheus.HistogramVec
}

func (m *RedisStateManager) recordMetrics(op string, workflowID string, duration time.Duration) {
    m.metrics.StateOperations.WithLabelValues(op, workflowID).Inc()
    m.metrics.StateLatency.WithLabelValues(op).Observe(duration.Seconds())
}
```

## Best Practices

1. **Keep State Minimal**: Store only essential data in workflow state
2. **Use Appropriate TTLs**: Set TTLs based on workflow duration and recovery requirements
3. **Handle Large Objects**: Store large objects in external storage (S3) and keep references in state
4. **Version State Schema**: Include version info for backward compatibility
5. **Monitor State Growth**: Track state size and implement cleanup policies
6. **Test State Recovery**: Regularly test workflow recovery from persisted state

## Summary

The state persistence strategy for Agentainer Flow provides a robust foundation for building reliable, scalable workflows. By leveraging Redis's capabilities and implementing proper patterns, we ensure that workflows can handle failures gracefully and maintain consistency across distributed execution.