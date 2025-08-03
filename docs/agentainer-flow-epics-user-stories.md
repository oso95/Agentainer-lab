# Agentainer Flow: EPICs and User Stories

## Overview

This document outlines the EPICs and User Stories for implementing Agentainer Flow, an orchestration-aware execution layer for managing multi-step workflows in Agentainer.

---

## EPIC 1: Multi-Job Awareness and Tagging System

**Goal**: Enable the system to understand and track relationships between jobs that belong to the same workflow.

**Value Statement**: As a platform, Agentainer needs to track which jobs belong together in a workflow to enable orchestration, monitoring, and state management across related jobs.

### User Story 1.1: Tag Jobs with Workflow Metadata
**As a** developer  
**I want to** tag my jobs with workflow_id and step_id  
**So that** the system can track which jobs belong to my workflow

**Acceptance Criteria:**
- [ ] When creating a job, I can specify `workflow_id` and `step_id` in the job metadata
- [ ] The workflow metadata is stored in the Agent struct
- [ ] The metadata persists in Redis with the job information
- [ ] The metadata is returned when querying job details
- [ ] Existing jobs without workflow metadata continue to work normally

**Technical Notes:**
- Extend Agent struct with optional workflow fields
- Update AgentManager.CreateAgent to accept workflow metadata
- Modify Redis storage schema to include workflow fields

### User Story 1.2: Query Jobs by Workflow
**As a** system operator  
**I want to** query all jobs that belong to a specific workflow  
**So that** I can monitor workflow progress and troubleshoot issues

**Acceptance Criteria:**
- [ ] API endpoint `/workflows/{workflow_id}/jobs` returns all jobs for a workflow
- [ ] Jobs are returned in order by step_id
- [ ] Response includes job status, creation time, and execution details
- [ ] Can filter by job status (running, completed, failed)
- [ ] Pagination support for workflows with many jobs

### User Story 1.3: View Workflow Information in CLI
**As a** developer  
**I want to** see workflow information when listing jobs  
**So that** I can understand job relationships at a glance

**Acceptance Criteria:**
- [ ] `agentainer list` shows workflow_id and step_id columns
- [ ] `agentainer workflow list` shows all active workflows
- [ ] `agentainer workflow status <workflow_id>` shows detailed workflow status
- [ ] `agentainer workflow jobs <workflow_id>` lists all jobs in a workflow
- [ ] Output is formatted in a clear, readable table

### User Story 1.4: Automatic Workflow Metadata Propagation
**As a** developer  
**I want** workflow metadata to automatically propagate to child jobs  
**So that** I don't have to manually track relationships

**Acceptance Criteria:**
- [ ] Jobs launched from within a workflow step inherit the workflow_id
- [ ] Child jobs get a new step_id that indicates their relationship
- [ ] Metadata propagation works through the SDK
- [ ] Parent-child relationships are trackable
- [ ] Metadata is available in the job's execution context

---

## EPIC 2: Fan-Out/Fan-In Orchestration Engine

**Goal**: Enable workflows to execute multiple jobs in parallel and wait for their completion before proceeding.

**Value Statement**: As a developer, I need to process data in parallel to improve performance while maintaining coordination between parallel tasks.

### User Story 2.1: Define Parallel Job Groups
**As a** developer  
**I want to** define groups of jobs that should run in parallel  
**So that** I can process data efficiently

**Acceptance Criteria:**
- [ ] SDK provides `@parallel` decorator for workflow steps
- [ ] Can specify the number of parallel jobs to create
- [ ] Each parallel job gets a unique identifier within the group
- [ ] Parallel jobs share the same step_id with different task_ids
- [ ] Can pass different parameters to each parallel job

**Example:**
```python
@step(name="process_chunks", parallel=True, max_workers=5)
async def process_chunk(self, ctx: Context, chunk_id: int):
    # This runs 5 times in parallel with chunk_id 0-4
    pass
```

### User Story 2.2: Wait for Parallel Job Completion
**As a** developer  
**I want** my workflow to wait for all parallel jobs to complete  
**So that** I can aggregate their results

**Acceptance Criteria:**
- [ ] Workflow automatically waits for all parallel jobs in a group
- [ ] Can configure timeout for parallel job groups
- [ ] Failed jobs in the group are reported
- [ ] Can specify failure tolerance (e.g., continue if 80% succeed)
- [ ] Partial results are accessible even if some jobs fail

### User Story 2.3: Aggregate Results from Parallel Jobs
**As a** developer  
**I want to** easily aggregate results from parallel job execution  
**So that** I can combine their outputs

**Acceptance Criteria:**
- [ ] SDK provides aggregation step type with `reduce=True`
- [ ] Aggregation step receives a list of all parallel job results
- [ ] Results maintain order based on job identifiers
- [ ] Can access both successful and failed job results
- [ ] State updates from parallel jobs are merged correctly

**Example:**
```python
@step(name="aggregate", depends_on=["process_chunks"], reduce=True)
async def aggregate_results(self, ctx: Context, chunk_results: List[Dict]):
    # Receives results from all parallel chunks
    total = sum(r["count"] for r in chunk_results)
    return {"total": total}
```

### User Story 2.4: Control Parallel Execution Resources
**As a** system operator  
**I want to** limit parallel job execution  
**So that** the system doesn't get overloaded

**Acceptance Criteria:**
- [ ] Global max parallelism setting in configuration
- [ ] Per-workflow parallelism limits
- [ ] Queue for jobs exceeding parallelism limits
- [ ] Metrics for parallel job queue depth
- [ ] Fair scheduling across multiple workflows

### User Story 2.5: Monitor Parallel Job Progress
**As a** developer  
**I want to** monitor the progress of parallel job execution  
**So that** I can track workflow performance

**Acceptance Criteria:**
- [ ] Real-time progress updates for parallel job groups
- [ ] Can see which jobs are pending/running/completed
- [ ] Estimated completion time based on finished jobs
- [ ] Progress information available via API and CLI
- [ ] Visual progress indicator in workflow status

---

## EPIC 3: Workflow State Persistence Layer

**Goal**: Provide a persistent state store shared across all steps in a workflow.

**Value Statement**: As a developer, I need to share data between workflow steps and preserve state across failures to build reliable data pipelines.

### User Story 3.1: Store and Retrieve Workflow State
**As a** developer  
**I want to** store data that persists across workflow steps  
**So that** later steps can access results from earlier steps

**Acceptance Criteria:**
- [ ] Simple key-value API: `ctx.state[key] = value`
- [ ] Support for common data types (strings, numbers, lists, dicts)
- [ ] State is automatically serialized/deserialized
- [ ] State changes are immediately persisted
- [ ] State is isolated per workflow execution

**Example:**
```python
# In step 1
ctx.state["input_file"] = "s3://bucket/data.csv"
ctx.state["row_count"] = 1000000

# In step 2
input_file = ctx.state["input_file"]
rows = ctx.state["row_count"]
```

### User Story 3.2: Atomic State Operations
**As a** developer  
**I want** thread-safe state operations  
**So that** parallel jobs can safely update shared state

**Acceptance Criteria:**
- [ ] Atomic increment/decrement operations
- [ ] Thread-safe list append operations
- [ ] Thread-safe set operations
- [ ] Atomic compare-and-swap operations
- [ ] No race conditions with parallel updates

**Example:**
```python
# Safe from parallel jobs
await ctx.state.increment("processed_count", 100)
await ctx.state.append_to_list("processed_files", filename)
await ctx.state.add_to_set("unique_users", user_id)
```

### User Story 3.3: State Persistence Across Failures
**As a** developer  
**I want** workflow state to survive failures  
**So that** workflows can resume from where they left off

**Acceptance Criteria:**
- [ ] State persists even if workflow crashes
- [ ] State survives system restarts
- [ ] Can configure state retention period
- [ ] State is recovered when workflow resumes
- [ ] Checkpoint mechanism for long-running steps

### User Story 3.4: Manage Large State Objects
**As a** developer  
**I want to** efficiently handle large state objects  
**So that** my workflows can process big datasets

**Acceptance Criteria:**
- [ ] Support for streaming large objects to external storage
- [ ] Automatic handling of objects over size threshold
- [ ] Transparent retrieval of large objects
- [ ] Compression for large state values
- [ ] Metrics for state size monitoring

### User Story 3.5: Query and Debug Workflow State
**As a** developer  
**I want to** inspect workflow state during execution  
**So that** I can debug issues

**Acceptance Criteria:**
- [ ] CLI command to view workflow state: `agentainer workflow state <id>`
- [ ] API endpoint to query state: `/workflows/{id}/state`
- [ ] State history/audit log for debugging
- [ ] Can view state at specific points in time
- [ ] Export state for offline analysis

---

## EPIC 4: Developer SDK and API

**Goal**: Provide an intuitive Python SDK and REST API for defining and managing workflows.

**Value Statement**: As a developer, I need simple yet powerful tools to define workflows without managing low-level orchestration details.

### User Story 4.1: Define Workflows with Python Decorators
**As a** developer  
**I want to** define workflows using familiar Python decorators  
**So that** I can quickly build workflows without learning a new DSL

**Acceptance Criteria:**
- [ ] `@workflow` decorator for workflow classes
- [ ] `@step` decorator for workflow steps
- [ ] Automatic dependency resolution based on method calls
- [ ] Type hints for better IDE support
- [ ] Validation of workflow structure at definition time

**Example:**
```python
@workflow(name="etl_pipeline", timeout="2h")
class ETLWorkflow:
    @step(name="extract")
    async def extract_data(self, ctx: Context):
        # Extract logic
        pass
```

### User Story 4.2: Local Workflow Testing
**As a** developer  
**I want to** test my workflows locally  
**So that** I can develop and debug efficiently

**Acceptance Criteria:**
- [ ] Local execution mode that simulates agent deployment
- [ ] Mock agent responses for unit testing
- [ ] Step-by-step debugging support
- [ ] Ability to inject test data into workflow state
- [ ] Test helpers for common scenarios

**Example:**
```python
async def test_workflow():
    wf = MyWorkflow()
    with mock_agent("processor") as mock:
        mock.returns({"status": "success"})
        result = await run_workflow_locally(wf)
        assert result["status"] == "completed"
```

### User Story 4.3: Programmatic Workflow Management
**As a** developer  
**I want to** manage workflows programmatically  
**So that** I can integrate workflows into my applications

**Acceptance Criteria:**
- [ ] Python client for all workflow operations
- [ ] Async/await support for all operations
- [ ] Proper error handling and exceptions
- [ ] Retry logic for transient failures
- [ ] Connection pooling for performance

**Example:**
```python
client = WorkflowClient("http://localhost:8081")
execution = await client.start_workflow(MyWorkflow(), input_data)
status = await execution.get_status()
await execution.cancel()
```

### User Story 4.4: Stream Workflow Events
**As a** developer  
**I want to** receive real-time workflow events  
**So that** I can monitor and react to workflow progress

**Acceptance Criteria:**
- [ ] Event streaming API for workflow events
- [ ] Filterable event types (step_started, step_completed, etc.)
- [ ] Webhooks for event notifications
- [ ] Event history API for past events
- [ ] SDK support for event handlers

**Example:**
```python
async for event in execution.stream_events():
    if event.type == "step_failed":
        await send_alert(event)
```

### User Story 4.5: Workflow Templates and Reusability
**As a** developer  
**I want to** create reusable workflow components  
**So that** I can build complex workflows from simple building blocks

**Acceptance Criteria:**
- [ ] Support for workflow inheritance
- [ ] Reusable step definitions
- [ ] Parameterized workflows
- [ ] Workflow composition (workflows as steps)
- [ ] Template library for common patterns

---

## EPIC 5: Workflow Scheduling and Triggers

**Goal**: Enable workflows to be triggered automatically based on schedules or events.

**Value Statement**: As a developer, I need my workflows to run automatically on schedules or in response to events without manual intervention.

### User Story 5.1: Schedule-Based Workflow Execution
**As a** developer  
**I want to** schedule my workflows to run periodically  
**So that** I can automate recurring tasks

**Acceptance Criteria:**
- [ ] Cron-style scheduling support
- [ ] Simple interval scheduling (hourly, daily, etc.)
- [ ] Timezone-aware scheduling
- [ ] Skip execution if previous run still active
- [ ] Catch-up runs for missed schedules

**Example:**
```python
@workflow(
    name="daily_report",
    schedule="0 2 * * *",  # Run at 2 AM daily
    timezone="US/Pacific"
)
class DailyReportWorkflow:
    pass
```

### User Story 5.2: Event-Based Workflow Triggers
**As a** developer  
**I want** workflows to start when specific events occur  
**So that** I can build reactive data pipelines

**Acceptance Criteria:**
- [ ] Define event triggers in workflow definition
- [ ] Support for multiple trigger types
- [ ] Event filtering and conditions
- [ ] Pass event data to workflow input
- [ ] Deduplication of duplicate events

**Example:**
```python
@workflow(
    name="file_processor",
    triggers=[
        S3Trigger(bucket="data-bucket", prefix="incoming/"),
        WebhookTrigger(endpoint="/process-file")
    ]
)
class FileProcessorWorkflow:
    pass
```

### User Story 5.3: Manual Workflow Triggers with Parameters
**As a** developer  
**I want to** manually trigger workflows with custom parameters  
**So that** I can run ad-hoc processing tasks

**Acceptance Criteria:**
- [ ] REST API endpoint for manual triggers
- [ ] CLI command for manual triggers
- [ ] Parameter validation before execution
- [ ] Queue position for manual triggers
- [ ] Trigger history and audit log

### User Story 5.4: Workflow Dependencies and Chains
**As a** developer  
**I want** workflows to trigger other workflows  
**So that** I can build complex pipeline dependencies

**Acceptance Criteria:**
- [ ] Define downstream workflow dependencies
- [ ] Pass data between chained workflows
- [ ] Conditional workflow triggers
- [ ] Fan-out to multiple downstream workflows
- [ ] Dependency visualization

### User Story 5.5: Monitor and Manage Scheduled Workflows
**As a** system operator  
**I want to** view and manage all scheduled workflows  
**So that** I can ensure workflows run as expected

**Acceptance Criteria:**
- [ ] View all scheduled workflows and next run times
- [ ] Pause/resume scheduled workflows
- [ ] Modify schedule without redeploying
- [ ] View schedule execution history
- [ ] Alerts for failed scheduled runs

---

## EPIC 6: Agent Pooling and Performance Optimization

**Goal**: Implement agent pooling for dramatic performance improvements in parallel workflows.

**Value Statement**: As a developer, I need my parallel workflows to execute quickly without the overhead of container startup times, achieving 20-50x performance improvements.

### User Story 6.1: Create Reusable Agent Pools
**As a** developer  
**I want** agents to be reused across parallel task executions  
**So that** I avoid container startup overhead

**Acceptance Criteria:**
- [ ] Agent pools maintain warm, idle agents ready for use
- [ ] Pool size is configurable (min/max agents)
- [ ] Agents are health-checked before reuse
- [ ] Failed agents are automatically replaced
- [ ] Pools are isolated per workflow/image

**Example:**
```python
@step(
    parallel=True,
    execution_mode="pooled",
    pool_size=5,
    max_parallel=20
)
async def process_item(self, ctx: Context, item: Dict):
    # 5 agents process 20 items with instant startup
    pass
```

### User Story 6.2: Agent Lifecycle Management
**As a** system operator  
**I want** pooled agents to be terminated based on usage policies  
**So that** resources are used efficiently

**Acceptance Criteria:**
- [ ] Agents terminate after configurable idle timeout
- [ ] Agents retire after maximum number of uses
- [ ] Unhealthy agents are terminated immediately
- [ ] Graceful shutdown with cleanup time
- [ ] Metrics track termination reasons

### User Story 6.3: Pool Performance Metrics
**As a** developer  
**I want to** monitor pool performance and utilization  
**So that** I can optimize pool configuration

**Acceptance Criteria:**
- [ ] Track pool utilization rate
- [ ] Measure queue wait times
- [ ] Monitor agent reuse count
- [ ] Show performance improvement metrics
- [ ] Alert on pool exhaustion

### User Story 6.4: Simplified MapReduce API
**As a** developer  
**I want** a simple decorator for common MapReduce patterns  
**So that** I can write less boilerplate code

**Acceptance Criteria:**
- [ ] Single decorator for map-reduce workflows
- [ ] Automatic handling of list/detail/aggregate pattern
- [ ] Built-in error handling and retries
- [ ] Progress tracking out of the box
- [ ] 70% less code than manual implementation

**Example:**
```python
@mapreduce(
    mapper="scraper:latest",
    reducer="analyzer:latest",
    pool_size=5,
    max_parallel=10
)
async def analyze_documents(self, ctx: Context, doc_list_url: str):
    return {"source": doc_list_url}
```

### User Story 6.5: Pool Warm-up and Pre-scaling
**As a** developer  
**I want** pools to pre-warm before workflow execution  
**So that** first tasks don't experience cold starts

**Acceptance Criteria:**
- [ ] Pools can be pre-warmed on workflow start
- [ ] Predictive scaling based on historical data
- [ ] Scheduled pool warm-up for known workloads
- [ ] Zero cold starts for properly configured pools
- [ ] Cost tracking for pre-warmed agents

---

## EPIC 7: Auto-Scaling for Dynamic Workloads

**Goal**: Enable workflows to automatically scale based on load and demand.

**Value Statement**: As a developer, I need my workflows to handle variable load efficiently, scaling up during peaks and down during quiet periods to optimize performance and cost.

### User Story 7.1: Metrics-Based Auto-Scaling
**As a** developer  
**I want** my workflow to scale automatically based on load  
**So that** it handles traffic spikes without manual intervention

**Acceptance Criteria:**
- [ ] Scale based on queue depth, utilization, response time
- [ ] Configurable scale-up/down thresholds
- [ ] Smooth scaling without thrashing
- [ ] Respect min/max instance limits
- [ ] Cool-down periods prevent rapid changes

**Example:**
```python
@workflow(
    scaling_policy={
        "min_instances": 1,
        "max_instances": 20,
        "target_utilization": 0.7,
        "scale_up_threshold": 0.8,
        "scale_down_threshold": 0.3
    }
)
```

### User Story 7.2: Predictive Auto-Scaling
**As a** developer  
**I want** the system to predict load patterns  
**So that** scaling happens before demand spikes

**Acceptance Criteria:**
- [ ] Learn from historical patterns
- [ ] Pre-scale for known busy periods
- [ ] Integrate with external calendars/events
- [ ] Override predictions manually
- [ ] A/B test scaling strategies

### User Story 7.3: Cost-Aware Scaling
**As a** system operator  
**I want** auto-scaling to consider cost constraints  
**So that** we don't exceed budget limits

**Acceptance Criteria:**
- [ ] Set cost caps for scaling
- [ ] Prefer spot/preemptible instances
- [ ] Scale down aggressively during expensive periods
- [ ] Cost tracking per workflow
- [ ] Budget alerts and limits

### User Story 7.4: Cross-Workflow Resource Sharing
**As a** system operator  
**I want** workflows to share resource pools  
**So that** overall resource utilization is optimized

**Acceptance Criteria:**
- [ ] Global resource pools across workflows
- [ ] Fair sharing algorithms
- [ ] Priority-based allocation
- [ ] Burst capacity handling
- [ ] Resource isolation options

### User Story 7.5: Auto-Scaling Observability
**As a** developer  
**I want to** understand auto-scaling decisions  
**So that** I can optimize scaling policies

**Acceptance Criteria:**
- [ ] Log all scaling decisions with reasons
- [ ] Visualize scaling events on timeline
- [ ] Show cost impact of scaling
- [ ] Recommend policy improvements
- [ ] Export scaling data for analysis

---

## EPIC 8: Workflow Monitoring and Observability

### User Story 6.1: Real-Time Workflow Metrics
**As a** system operator  
**I want** real-time metrics on workflow execution  
**So that** I can monitor system health

**Acceptance Criteria:**
- [ ] Workflow start/completion rates
- [ ] Average workflow duration by type
- [ ] Step execution times and success rates
- [ ] Resource utilization per workflow
- [ ] Queue depths and wait times

### User Story 6.2: Workflow Execution Timeline
**As a** developer  
**I want to** see a visual timeline of workflow execution  
**So that** I can identify bottlenecks and failures

**Acceptance Criteria:**
- [ ] Visual timeline showing all steps
- [ ] Parallel execution visualization
- [ ] Duration for each step
- [ ] Status indicators (success/failure/running)
- [ ] Drill-down to step details

### User Story 6.3: Distributed Tracing for Workflows
**As a** developer  
**I want** distributed tracing across workflow steps  
**So that** I can debug complex workflows

**Acceptance Criteria:**
- [ ] OpenTelemetry integration
- [ ] Trace ID propagation across steps
- [ ] Span creation for each step
- [ ] Custom span attributes
- [ ] Integration with tracing backends

### User Story 6.4: Workflow Error Tracking
**As a** developer  
**I want** detailed error information when workflows fail  
**So that** I can quickly fix issues

**Acceptance Criteria:**
- [ ] Detailed error messages with stack traces
- [ ] Error context (state at time of error)
- [ ] Error categorization and grouping
- [ ] Error trends and patterns
- [ ] Integration with error tracking services

### User Story 6.5: Workflow Performance Analytics
**As a** system operator  
**I want** analytics on workflow performance  
**So that** I can optimize resource usage

**Acceptance Criteria:**
- [ ] Historical performance trends
- [ ] Resource usage patterns
- [ ] Cost analysis per workflow
- [ ] Optimization recommendations
- [ ] Capacity planning insights

---

## Implementation Priority

### Phase 1 (MVP) - Weeks 1-4
1. EPIC 1: Multi-Job Awareness (Critical foundation)
2. EPIC 3: State Persistence (Story 3.1, 3.2, 3.3)
3. EPIC 6: Basic Agent Pooling (Story 6.1, 6.2) - **Critical for performance**

### Phase 2 (Core Features) - Weeks 5-8
1. EPIC 2: Fan-Out/Fan-In Orchestration
2. EPIC 4: Developer SDK (Story 4.1, 4.2, 4.3)
3. EPIC 6: Simplified MapReduce API (Story 6.4) - **Major usability win**

### Phase 3 (Production Ready) - Weeks 9-12
1. EPIC 4: Complete Developer SDK
2. EPIC 7: Auto-Scaling (Story 7.1, 7.3) - **Production scalability**
3. EPIC 8: Basic Monitoring (Story 8.1, 8.4)

### Phase 4 (Advanced Features) - Future
1. EPIC 5: Scheduling and Triggers
2. EPIC 7: Predictive Scaling (Story 7.2)
3. EPIC 8: Advanced Monitoring
4. DAG capabilities and conditional logic

---

## Success Criteria

### Technical Success
- All acceptance criteria met for implemented stories
- Test coverage > 80% for new code
- Performance benchmarks met:
  - < 100ms orchestration overhead per step
  - < 200ms agent acquisition from pool (vs 2-5s cold start)
  - 20-50x improvement in parallel task execution
- No regression in existing Agentainer functionality
- Pool utilization > 70% during peak loads

### User Success
- Developers can create a workflow in < 30 minutes
- 90% of workflows complete successfully
- Average time to debug failures < 15 minutes
- Positive developer feedback score > 4/5

### Business Success
- 50+ workflows created in first month
- 80% reduction in manual orchestration code
- 60% improvement in pipeline reliability
- Adoption by 3+ internal teams