# Agentainer Flow: Agent Isolation Design

## Overview

This document ensures that Agentainer Flow's workflow orchestration maintains the strict agent isolation that exists in the current Agentainer architecture. Each agent must remain isolated and unable to access or be assigned tasks from other agents.

## Current Isolation Mechanisms

### 1. Network Isolation
- All agents run on a dedicated `agentainer-network` bridge network
- Agents cannot directly communicate with each other
- All external access is through the authenticated proxy layer

### 2. Container Isolation  
- Each agent runs in its own Docker container
- Unique container IDs and hostnames (using agent ID)
- Resource limits (CPU, memory) per container

### 3. Request Routing
- Proxy routes requests based on agent ID in the URL path
- No agent can receive requests intended for another agent
- Request queues are agent-specific: `agent:{id}:requests:*`

## Workflow Integration with Isolation

### 1. Workflow Metadata is Non-Routable

The workflow metadata (`workflow_id`, `step_id`) added to agents is purely for tracking and organization. It does NOT affect request routing:

```go
// Extended Agent struct
type Agent struct {
    // ... existing fields
    
    // Workflow metadata - for tracking only
    WorkflowID   string `json:"workflow_id,omitempty"`
    StepID       string `json:"step_id,omitempty"`
    
    // Routing still based solely on Agent.ID
}
```

### 2. Request Routing Remains Unchanged

The proxy routing logic remains exactly the same:

```go
// Proxy handler - NO CHANGES
func (s *Server) proxyHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    agentID := vars["id"]  // ONLY uses agent ID for routing
    
    // Get the specific agent by ID
    agent, err := s.agentMgr.GetAgent(agentID)
    if err != nil {
        s.sendError(w, http.StatusNotFound, "Agent not found")
        return
    }
    
    // Route ONLY to this specific agent's container
    target := fmt.Sprintf("http://%s:8000", agent.ContainerID)
    // ... proxy logic
}
```

### 3. Workflow-Specific Agent Creation

When a workflow creates agents, each agent gets a unique ID and remains isolated:

```python
# In workflow SDK
@step(name="process", parallel=True, max_workers=3)
async def process_data(self, ctx: Context, worker_id: int):
    # Each parallel execution creates a SEPARATE agent
    agent = await ctx.deploy_agent(
        name=f"processor-{ctx.workflow_id}-{worker_id}",  # Unique name
        image="processor:latest",
        env={"WORKER_ID": str(worker_id)}
    )
    # This creates agents with IDs like:
    # - agent-1234567890  (completely unique)
    # - agent-1234567891  (completely unique)
    # - agent-1234567892  (completely unique)
    
    # Each agent is isolated - cannot access others
```

### 4. State Isolation

Workflow state is isolated by workflow execution ID, not shared between agents:

```
# Redis key structure ensures isolation
workflow:{workflow_id}:exec:{execution_id}:state  # Workflow state
agent:{agent_id}:*                                # Agent-specific data

# Agents CANNOT access workflow state directly
# Only the orchestrator can read/write workflow state
```

### 5. Request Queue Isolation

Each agent maintains its own request queues:

```
# Agent-specific queues (NO CHANGE)
agent:{agent_id}:requests:pending
agent:{agent_id}:requests:completed
agent:{agent_id}:requests:failed

# Workflow orchestration uses separate queues
workflow:{workflow_id}:jobs:pending
workflow:{workflow_id}:jobs:completed
```

## Security Boundaries

### 1. Agent-to-Agent Isolation

Agents CANNOT:
- Access another agent's container
- Read another agent's requests or state  
- Communicate directly with other agents
- Share memory or filesystem (unless explicitly mounted)

### 2. Workflow-to-Agent Relationship

Workflows CAN:
- Create new isolated agents
- Track which agents belong to a workflow
- Aggregate results from multiple agents
- Coordinate agent lifecycle

Workflows CANNOT:
- Make agents share resources
- Route requests between agents
- Break container isolation

### 3. Network Security

```yaml
# Docker network configuration
agentainer-network:
  driver: bridge
  options:
    com.docker.network.bridge.name: agentainer0
  internal: false  # Allows external access through proxy
  enable_ipv6: false
  ipam:
    driver: default
    config:
      - subnet: 172.20.0.0/16  # Isolated subnet
```

Each agent gets a unique IP in this subnet but cannot initiate connections to other agents.

## Implementation Guidelines

### 1. Workflow Manager MUST:
- Create unique agent IDs for every agent
- Never reuse or share agent IDs
- Track workflow membership without affecting routing
- Maintain separate state stores for workflows and agents

### 2. SDK MUST:
- Generate unique names for workflow-created agents
- Never allow direct agent-to-agent communication
- Use the orchestrator for all coordination

### 3. Proxy MUST:
- Continue routing based solely on agent ID
- Never consider workflow metadata for routing decisions
- Maintain authentication per request

## Example: Parallel Processing with Isolation

```python
@workflow(name="data_processor")
class DataProcessingWorkflow:
    
    @step(name="split_data")
    async def split_data(self, ctx: Context):
        # Create a splitter agent
        splitter = await ctx.deploy_agent(
            name=f"splitter-{ctx.workflow_id}",
            image="splitter:latest"
        )
        # Agent ID: agent-1234567890 (unique)
        
        result = await splitter.wait()
        ctx.state["chunks"] = result.output["chunk_count"]
        return result
    
    @step(name="process_chunks", parallel=True, max_workers=5)
    async def process_chunk(self, ctx: Context, chunk_id: int):
        # Each parallel execution creates an isolated agent
        processor = await ctx.deploy_agent(
            name=f"proc-{ctx.workflow_id}-{chunk_id}",
            image="processor:latest",
            env={
                "CHUNK_ID": str(chunk_id),
                "TOTAL": str(ctx.state["chunks"])
            }
        )
        # Agent IDs: 
        # - agent-1234567891 (chunk 0)
        # - agent-1234567892 (chunk 1)
        # - agent-1234567893 (chunk 2)
        # - agent-1234567894 (chunk 3)
        # - agent-1234567895 (chunk 4)
        
        # Each agent:
        # - Runs in isolated container
        # - Has unique IP on agentainer-network
        # - Cannot access other agents
        # - Receives requests only through its unique proxy path
        
        return await processor.wait()
```

## Testing Isolation

### 1. Security Tests
```python
def test_agent_isolation():
    # Create two agents in same workflow
    agent1 = deploy_agent("test1", workflow_id="wf1")
    agent2 = deploy_agent("test2", workflow_id="wf1")
    
    # Verify they cannot access each other
    assert agent1.id != agent2.id
    assert agent1.container_id != agent2.container_id
    
    # Try to send agent2's request to agent1 (should fail)
    response = proxy_request(agent1.id, agent2_specific_request)
    assert response.status_code == 404
```

### 2. Network Tests
```bash
# From inside agent1 container
$ ping agent2-container  # Should fail - no route
$ curl http://agent2:8000  # Should fail - no access
```

### 3. State Isolation Tests
```python
def test_state_isolation():
    # Agent cannot access workflow state directly
    agent = deploy_agent("test", workflow_id="wf1")
    
    # This should fail - agents don't have workflow state access
    with pytest.raises(Unauthorized):
        agent.access_workflow_state()
```

## Conclusion

Agentainer Flow maintains complete agent isolation by:

1. **Preserving existing isolation mechanisms** - No changes to network, container, or routing isolation
2. **Adding workflow metadata non-invasively** - Workflow IDs are for tracking only, not routing
3. **Creating unique agents for each workflow step** - No sharing or reuse of agents
4. **Separating orchestration from execution** - Workflow manager coordinates but doesn't break boundaries
5. **Maintaining security boundaries** - Agents remain fully isolated from each other

The workflow orchestration layer sits above the agent layer, coordinating isolated agents without compromising their security boundaries.