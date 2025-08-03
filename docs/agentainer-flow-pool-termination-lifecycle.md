# Agent Pool Termination and Lifecycle Management

## Overview

Pooled agents have a complete lifecycle including termination. Smart termination is crucial for resource efficiency, cost control, and system health.

## Agent Lifecycle in Pool

```
┌─────────┐      ┌─────────┐      ┌─────────┐      ┌───────────┐
│ Created │ ───> │  Idle   │ ───> │ Active  │ ───> │Terminated │
└─────────┘      └─────────┘      └─────────┘      └───────────┘
                      ↑                 │
                      └─────────────────┘
                         (Reuse Cycle)
```

## Termination Triggers

### 1. **Idle Timeout**
Agents that sit idle too long are terminated to save resources:

```go
type PooledAgent struct {
    Agent        *Agent
    LastUsed     time.Time
    IdleTimeout  time.Duration  // e.g., 5 minutes
    UsageCount   int
    MaxUsages    int            // Terminate after N uses
}

func (p *AgentPool) idleCleanupWorker() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        now := time.Now()
        
        // Check each idle agent
        for _, agent := range p.getIdleAgents() {
            idleTime := now.Sub(agent.LastUsed)
            
            if idleTime > agent.IdleTimeout {
                log.Printf("Terminating agent %s due to idle timeout (%v)", 
                    agent.Agent.ID, idleTime)
                p.terminateAgent(agent)
            }
        }
    }
}
```

### 2. **Usage Limit**
Terminate agents after a certain number of uses to prevent degradation:

```go
func (p *AgentPool) releaseAgent(agent *PooledAgent) {
    agent.UsageCount++
    
    // Check if agent should be retired
    if agent.UsageCount >= agent.MaxUsages {
        log.Printf("Retiring agent %s after %d uses", 
            agent.Agent.ID, agent.UsageCount)
        p.terminateAgent(agent)
        return
    }
    
    // Otherwise return to pool
    agent.State = StateIdle
    agent.LastUsed = time.Now()
    p.IdleAgents <- agent
}
```

### 3. **Health Check Failures**
Unhealthy agents are terminated immediately:

```go
func (p *AgentPool) healthCheckWorker() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        for _, agent := range p.getAllAgents() {
            if !p.isHealthy(agent) {
                log.Printf("Terminating unhealthy agent %s", agent.Agent.ID)
                p.terminateAgent(agent)
                
                // Replace with new agent if below min size
                if p.Size() < p.MinSize {
                    p.createNewAgent()
                }
            }
        }
    }
}

func (p *AgentPool) isHealthy(agent *PooledAgent) bool {
    // Check container is running
    if agent.Agent.Status != agent.StatusRunning {
        return false
    }
    
    // Check memory usage
    stats, _ := p.dockerClient.ContainerStats(agent.Agent.ContainerID)
    if stats.MemoryUsage > p.MaxMemoryThreshold {
        return false
    }
    
    // Check responsiveness
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    resp, err := p.pingAgent(ctx, agent)
    return err == nil && resp.StatusCode == 200
}
```

### 4. **Scale-Down Events**
When auto-scaling reduces pool size:

```go
func (p *AgentPool) scaleDown(targetSize int) {
    currentSize := p.Size()
    toRemove := currentSize - targetSize
    
    if toRemove <= 0 {
        return
    }
    
    // Terminate idle agents first (LIFO for cache efficiency)
    removed := 0
    for removed < toRemove {
        select {
        case agent := <-p.IdleAgents:
            p.terminateAgent(agent)
            removed++
        default:
            // No idle agents, wait for active ones to finish
            log.Printf("Waiting for active agents to become idle for termination")
            time.Sleep(1 * time.Second)
        }
    }
}
```

### 5. **Resource Pressure**
System-wide resource constraints trigger termination:

```go
func (p *AgentPool) resourceMonitor() {
    for {
        sysInfo := p.getSystemInfo()
        
        if sysInfo.MemoryPressure > 0.9 {
            // Terminate least recently used agents
            p.terminateLRUAgents(p.Size() / 4)  // Remove 25%
        }
        
        if sysInfo.DiskPressure > 0.95 {
            // Emergency termination
            p.terminateAllIdleAgents()
        }
        
        time.Sleep(5 * time.Second)
    }
}
```

## Termination Process

### 1. **Graceful Termination**
```go
func (p *AgentPool) terminateAgent(agent *PooledAgent) error {
    // Step 1: Mark as terminating
    agent.State = StateTerminating
    
    // Step 2: Remove from available pools
    p.removeFromPools(agent)
    
    // Step 3: Send graceful shutdown signal
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Send SIGTERM to allow cleanup
    if err := p.dockerClient.ContainerKill(ctx, agent.Agent.ContainerID, "SIGTERM"); err != nil {
        log.Printf("Failed to send SIGTERM: %v", err)
    }
    
    // Step 4: Wait for graceful shutdown
    gracePeriod := 10 * time.Second
    select {
    case <-time.After(gracePeriod):
        // Force kill if not stopped
        p.dockerClient.ContainerKill(ctx, agent.Agent.ContainerID, "SIGKILL")
    case <-p.waitForStop(agent.Agent.ContainerID):
        // Stopped gracefully
    }
    
    // Step 5: Remove container
    err := p.dockerClient.ContainerRemove(ctx, agent.Agent.ContainerID, types.ContainerRemoveOptions{
        Force: true,
        RemoveVolumes: true,
    })
    
    // Step 6: Clean up metadata
    p.cleanupAgent(agent)
    
    // Step 7: Update metrics
    p.metrics.TerminatedAgents.Inc()
    
    return err
}
```

### 2. **Emergency Termination**
For critical situations:

```go
func (p *AgentPool) emergencyTerminate(agent *PooledAgent) {
    // Skip graceful shutdown, force kill immediately
    ctx := context.Background()
    
    p.dockerClient.ContainerKill(ctx, agent.Agent.ContainerID, "SIGKILL")
    p.dockerClient.ContainerRemove(ctx, agent.Agent.ContainerID, types.ContainerRemoveOptions{
        Force: true,
    })
    
    p.cleanupAgent(agent)
}
```

## Configuration Options

### 1. **Pool-Level Configuration**
```python
@workflow(
    pool_config={
        "idle_timeout": 300,        # 5 minutes
        "max_agent_uses": 100,      # Retire after 100 tasks
        "health_check_interval": 10, # Every 10 seconds
        "graceful_shutdown": 30,    # 30 second grace period
        "aggressive_cleanup": False  # Keep agents longer
    }
)
```

### 2. **Termination Policies**

```go
type TerminationPolicy struct {
    IdleTimeout      time.Duration
    MaxUsages        int
    MaxAge           time.Duration  // Terminate after age
    MemoryThreshold  float64        // Terminate if memory > threshold
    ErrorThreshold   int            // Terminate after N errors
}

// Different policies for different workloads
var (
    // Aggressive: For cost optimization
    AggressivePolicy = TerminationPolicy{
        IdleTimeout:     1 * time.Minute,
        MaxUsages:       50,
        MaxAge:          1 * time.Hour,
        MemoryThreshold: 0.8,
        ErrorThreshold:  3,
    }
    
    // Balanced: Default
    BalancedPolicy = TerminationPolicy{
        IdleTimeout:     5 * time.Minute,
        MaxUsages:       100,
        MaxAge:          4 * time.Hour,
        MemoryThreshold: 0.9,
        ErrorThreshold:  5,
    }
    
    // Performance: Keep agents warm
    PerformancePolicy = TerminationPolicy{
        IdleTimeout:     30 * time.Minute,
        MaxUsages:       500,
        MaxAge:          24 * time.Hour,
        MemoryThreshold: 0.95,
        ErrorThreshold:  10,
    }
)
```

### 3. **Auto-Scaling Integration**

```go
func (as *AutoScaler) applyScalingDecision(decision ScalingDecision) {
    switch decision {
    case ScaleUp:
        // Create new agents
        for i := 0; i < as.ScaleUpBatch; i++ {
            as.Pool.createNewAgent()
        }
        
    case ScaleDown:
        // Terminate agents based on policy
        switch as.TerminationStrategy {
        case "LIFO":
            // Terminate newest first (better cache locality)
            as.Pool.terminateNewestAgents(as.ScaleDownBatch)
            
        case "FIFO":
            // Terminate oldest first (prevent staleness)
            as.Pool.terminateOldestAgents(as.ScaleDownBatch)
            
        case "LRU":
            // Terminate least recently used
            as.Pool.terminateLRUAgents(as.ScaleDownBatch)
        }
    }
}
```

## Monitoring and Metrics

```go
type PoolMetrics struct {
    // Termination metrics
    TerminationsByReason map[string]int64
    AvgAgentLifetime     time.Duration
    TerminationRate      float64
    
    // Health metrics
    HealthCheckFailures  int64
    RecycleRate         float64
}

// Prometheus metrics
pool_agent_terminations_total{reason="idle_timeout", pool="mapreduce"}
pool_agent_lifetime_seconds{pool="mapreduce", quantile="0.95"}
pool_agent_health_failures_total{pool="mapreduce", reason="memory"}
```

## Best Practices

### 1. **Gradual Termination**
Don't terminate all at once:
```go
// Spread terminations to avoid thundering herd
for i := 0; i < toTerminate; i++ {
    p.terminateAgent(agents[i])
    time.Sleep(100 * time.Millisecond)
}
```

### 2. **Predictive Termination**
Terminate before problems occur:
```go
// Terminate agents approaching limits
if agent.MemoryUsage > 0.8 * limit {
    p.scheduleTermination(agent, 1*time.Minute)
}
```

### 3. **Cost-Aware Termination**
Consider billing cycles:
```go
// If billed hourly, keep agents until next billing cycle
timeToNextBilling := time.Until(agent.NextBillingCycle())
if timeToNextBilling < 5*time.Minute {
    agent.IdleTimeout = timeToNextBilling
}
```

## Summary

Agent termination in pools is:
- **Automatic** - Based on configurable policies
- **Graceful** - Allows agents to clean up
- **Efficient** - Removes unhealthy/unused agents
- **Flexible** - Different policies for different workloads
- **Observable** - Full metrics and monitoring

This ensures pools remain healthy, cost-effective, and performant while providing the warm-start benefits.