# Agentainer Flow: Implementation Plan

## Executive Summary

Agentainer Flow is an orchestration-aware execution layer that enables developers to design and run multi-step workflows with automatic coordination of parallel and sequential tasks. It extends Agentainer's current single-agent execution model to support complex workflow patterns including fan-out/fan-in, state persistence across workflow steps, and eventually full DAG capabilities.

## Architecture Overview

### Core Design Principles

1. **Non-Breaking Integration**: Agentainer Flow will be implemented as an optional layer that doesn't affect existing single-agent functionality
2. **Developer-First API**: Simple, intuitive SDK that abstracts complexity while providing power when needed
3. **State-Centric Design**: Built-in workflow state persistence using existing Redis infrastructure
4. **Progressive Enhancement**: Start with fixed parallelism, evolve to auto-scaling and DAG capabilities
5. **Observability by Default**: Full visibility into workflow execution, state, and performance

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agentainer Flow Layer                      │
├─────────────────────────────────────────────────────────────────┤
│  Workflow      │  Orchestration  │  State         │  Developer   │
│  Manager       │  Engine         │  Manager       │  SDK/API     │
├─────────────────────────────────────────────────────────────────┤
│                    Existing Agentainer Core                       │
├─────────────────────────────────────────────────────────────────┤
│  Agent         │  Request        │  Redis         │  Docker      │
│  Manager       │  Manager        │  Storage       │  Runtime     │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation EPICs and User Stories

### EPIC 1: Multi-Job Awareness and Tagging System

**Goal**: Enable the system to understand relationships between jobs in a workflow

#### User Stories:

1. **As a developer**, I want to tag jobs with workflow_id and step_id so that the system knows which jobs belong to the same workflow
   - Acceptance Criteria:
     - Jobs can be tagged with workflow_id and step_id during creation
     - System can query all jobs belonging to a workflow
     - Tags are persisted in Redis with job metadata

2. **As a system operator**, I want to view all jobs grouped by workflow in the UI/CLI
   - Acceptance Criteria:
     - CLI command `agentainer workflow list` shows all workflows
     - CLI command `agentainer workflow status <workflow_id>` shows workflow details
     - Jobs display their workflow association in `agentainer list`

3. **As a developer**, I want workflow metadata to be automatically propagated to child jobs
   - Acceptance Criteria:
     - Child jobs inherit workflow_id from parent
     - Step_id is automatically generated for each workflow step
     - Metadata is available in job context

### EPIC 2: Fan-Out/Fan-In Orchestration

**Goal**: Allow workflows to launch multiple parallel jobs and wait for completion

#### User Stories:

1. **As a developer**, I want to define parallel job execution in my workflow
   - Acceptance Criteria:
     - SDK supports defining parallel job groups
     - System can launch multiple jobs simultaneously
     - Fixed parallelism limit is configurable

2. **As a developer**, I want to wait for all parallel jobs to complete before proceeding
   - Acceptance Criteria:
     - Workflow engine tracks parallel job completion
     - Fan-in mechanism aggregates results
     - Timeout and error handling for parallel groups

3. **As a system**, I want to efficiently manage resources during parallel execution
   - Acceptance Criteria:
     - Parallel jobs respect concurrency limits
     - Queue management for pending parallel jobs
     - Resource allocation is fair across workflows

### EPIC 3: Workflow State Persistence

**Goal**: Provide persistent state storage shared across workflow steps

#### User Stories:

1. **As a developer**, I want to store and retrieve workflow state between steps
   - Acceptance Criteria:
     - Simple key-value API for state storage
     - State is scoped to workflow_id
     - Automatic cleanup after workflow completion

2. **As a developer**, I want to aggregate results from parallel jobs
   - Acceptance Criteria:
     - Aggregation helpers in SDK
     - Thread-safe state updates from parallel jobs
     - Support for common aggregation patterns

3. **As a system operator**, I want workflow state to survive system restarts
   - Acceptance Criteria:
     - State persisted in Redis with TTL
     - State recovery on workflow resume
     - Configurable retention policies

### EPIC 4: Developer SDK and API

**Goal**: Provide intuitive interfaces for defining and managing workflows

#### User Stories:

1. **As a developer**, I want to define workflows using Python decorators
   - Acceptance Criteria:
     - `@workflow` decorator for workflow definition
     - `@step` decorator for workflow steps
     - Type hints and validation support

2. **As a developer**, I want to test workflows locally before deployment
   - Acceptance Criteria:
     - Local workflow execution mode
     - Mocked agent execution for testing
     - Debug logging and step inspection

3. **As a developer**, I want programmatic workflow management
   - Acceptance Criteria:
     - Start/stop/pause/resume workflows via API
     - Query workflow status and history
     - Handle workflow events and callbacks

### EPIC 5: Fixed Parallelism Implementation

**Goal**: Implement configurable fixed parallelism for workflow execution

#### User Stories:

1. **As a developer**, I want to configure parallelism limits for my workflow
   - Acceptance Criteria:
     - Per-workflow parallelism configuration
     - Global parallelism limits respected
     - Queue overflow handling

2. **As a system operator**, I want to monitor parallel execution performance
   - Acceptance Criteria:
     - Metrics for parallel job execution time
     - Queue depth monitoring
     - Resource utilization tracking

### EPIC 6: DAG Execution Engine (Future)

**Goal**: Extend to full DAG capabilities with branching and conditional logic

#### User Stories:

1. **As a developer**, I want to define conditional workflow branches
   - Acceptance Criteria:
     - Conditional step execution based on previous results
     - Dynamic workflow graph construction
     - Branch merge capabilities

2. **As a developer**, I want to implement retry and error handling strategies
   - Acceptance Criteria:
     - Per-step retry configuration
     - Error propagation and handling
     - Compensating transactions support

## Technical Implementation Details

### 1. Data Model Extensions

```go
// Workflow represents a multi-step workflow
type Workflow struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Status      WorkflowStatus         `json:"status"`
    CreatedAt   time.Time             `json:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at"`
    Config      WorkflowConfig        `json:"config"`
    Steps       []WorkflowStep        `json:"steps"`
    State       map[string]interface{} `json:"state"`
}

// WorkflowStep represents a step in the workflow
type WorkflowStep struct {
    ID           string              `json:"id"`
    Name         string              `json:"name"`
    Type         StepType            `json:"type"` // sequential, parallel
    Status       StepStatus          `json:"status"`
    Jobs         []string            `json:"jobs"` // job IDs
    Dependencies []string            `json:"dependencies"`
    Config       StepConfig          `json:"config"`
}

// Extend existing Agent struct
type Agent struct {
    // ... existing fields
    WorkflowID   string              `json:"workflow_id,omitempty"`
    StepID       string              `json:"step_id,omitempty"`
    WorkflowMeta map[string]string   `json:"workflow_meta,omitempty"`
}
```

### 2. Key Components

#### WorkflowManager
- Manages workflow lifecycle (create, start, stop, resume)
- Coordinates with AgentManager for job execution
- Handles workflow state persistence

#### OrchestrationEngine
- Executes workflow steps according to dependencies
- Manages parallel job groups and fan-in operations
- Handles workflow state transitions

#### StateManager
- Provides workflow-scoped state storage
- Manages state TTL and cleanup
- Ensures thread-safe state updates

#### WorkflowSDK
- Python SDK for workflow definition
- REST API client for workflow management
- Local testing utilities

### 3. Storage Schema

```
# Workflow metadata
workflow:{id} -> JSON (Workflow struct)

# Workflow state
workflow:{id}:state -> HASH (key-value state)

# Workflow steps
workflow:{id}:steps -> LIST (ordered steps)

# Step jobs
workflow:{id}:step:{step_id}:jobs -> SET (job IDs)

# Workflow events
workflow:{id}:events -> STREAM (event log)
```

### 4. API Endpoints

```
# Workflow Management
POST   /workflows                    - Create workflow
GET    /workflows                    - List workflows
GET    /workflows/{id}               - Get workflow details
PUT    /workflows/{id}/start         - Start workflow
PUT    /workflows/{id}/pause         - Pause workflow
PUT    /workflows/{id}/resume        - Resume workflow
DELETE /workflows/{id}               - Delete workflow

# Workflow State
GET    /workflows/{id}/state         - Get workflow state
PUT    /workflows/{id}/state         - Update workflow state
GET    /workflows/{id}/state/{key}   - Get specific state key

# Workflow Events
GET    /workflows/{id}/events        - Get workflow events stream
```

### 5. SDK Example

```python
from agentainer_flow import workflow, step, parallel

@workflow(name="data_processing_pipeline")
class DataProcessingWorkflow:
    
    @step(name="fetch_data")
    def fetch_data(self, context):
        # Deploy agent to fetch data
        agent = context.deploy_agent(
            name="data_fetcher",
            image="data-fetcher:latest",
            env={"SOURCE": "s3://bucket/data"}
        )
        result = agent.wait()
        context.state["raw_data_path"] = result["output_path"]
        return result
    
    @step(name="process_data", depends_on=["fetch_data"])
    @parallel(max_workers=5)
    def process_data(self, context, chunk_id):
        # This step will run in parallel for each chunk
        agent = context.deploy_agent(
            name=f"processor_{chunk_id}",
            image="data-processor:latest",
            env={
                "INPUT": context.state["raw_data_path"],
                "CHUNK": str(chunk_id)
            }
        )
        return agent.wait()
    
    @step(name="aggregate_results", depends_on=["process_data"])
    def aggregate_results(self, context, process_results):
        # Fan-in: aggregate results from parallel processing
        agent = context.deploy_agent(
            name="aggregator",
            image="aggregator:latest",
            env={"RESULTS": json.dumps(process_results)}
        )
        final_result = agent.wait()
        context.state["final_output"] = final_result["output_path"]
        return final_result

# Usage
wf = DataProcessingWorkflow()
wf.run(chunks=10)  # Process data in 10 parallel chunks
```

## Implementation Phases

### Phase 1: Foundation (Weeks 1-3)
- Implement workflow and step data models
- Extend Agent struct with workflow metadata
- Add workflow tagging to existing agent operations
- Create WorkflowManager component

### Phase 2: Orchestration (Weeks 4-6)
- Implement OrchestrationEngine
- Add fan-out/fan-in capabilities
- Implement workflow state persistence
- Create workflow API endpoints

### Phase 3: Developer Experience (Weeks 7-9)
- Build Python SDK
- Implement workflow decorators
- Add CLI commands for workflow management
- Create testing utilities

### Phase 4: Production Features (Weeks 10-12)
- Add monitoring and metrics
- Implement workflow event streaming
- Add retry and error handling
- Performance optimization

### Phase 5: Advanced Features (Future)
- DAG execution engine
- Conditional branching
- Dynamic workflow composition
- Auto-scaling capabilities

## Success Metrics

1. **Developer Adoption**
   - Number of workflows created
   - SDK usage and feedback
   - Time to first workflow deployment

2. **System Performance**
   - Workflow execution time
   - Parallel job throughput
   - State operation latency

3. **Reliability**
   - Workflow completion rate
   - Error recovery success rate
   - State consistency metrics

4. **Resource Efficiency**
   - CPU/Memory utilization during parallel execution
   - Queue management efficiency
   - Network overhead

## Risk Mitigation

1. **Backward Compatibility**: All changes are additive; existing functionality remains unchanged
2. **Performance Impact**: Workflow metadata adds minimal overhead to existing operations
3. **Complexity Management**: Progressive disclosure in SDK - simple things simple, complex things possible
4. **Resource Constraints**: Fixed parallelism prevents resource exhaustion
5. **State Consistency**: Redis transactions ensure atomic state updates

## Conclusion

Agentainer Flow transforms Agentainer from a single-agent runtime into a full workflow orchestration platform. By building on existing infrastructure and following established patterns, we can deliver powerful orchestration capabilities while maintaining the simplicity that makes Agentainer attractive to developers.

The phased implementation approach ensures we can deliver value incrementally while gathering feedback to guide future development. The end result will be a system that handles orchestration use cases requiring tools like Airflow or Temporal, but with built-in state persistence and easier deployment tailored specifically for LLM agent workflows.