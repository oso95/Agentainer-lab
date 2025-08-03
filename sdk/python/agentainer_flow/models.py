"""
Data models for Agentainer Flow SDK
"""

from enum import Enum
from typing import Dict, List, Optional, Any, Union
from datetime import datetime
from pydantic import BaseModel, Field


class WorkflowStatus(str, Enum):
    """Workflow execution status"""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class StepStatus(str, Enum):
    """Workflow step status"""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"


class StepType(str, Enum):
    """Workflow step type"""
    SEQUENTIAL = "sequential"
    PARALLEL = "parallel"
    REDUCE = "reduce"
    MAPREDUCE = "mapreduce"


class AgentStatus(str, Enum):
    """Agent status"""
    CREATED = "created"
    RUNNING = "running"
    STOPPED = "stopped"
    PAUSED = "paused"
    FAILED = "failed"


class ResourceLimits(BaseModel):
    """Resource limits for agents"""
    cpu_limit: Optional[int] = Field(None, description="CPU limit in nanoseconds")
    memory_limit: Optional[int] = Field(None, description="Memory limit in bytes")


class RetryPolicy(BaseModel):
    """Retry policy for steps"""
    max_attempts: int = Field(3, description="Maximum retry attempts")
    backoff: str = Field("exponential", description="Backoff strategy")
    delay: str = Field("1s", description="Initial delay between retries")


class PoolConfig(BaseModel):
    """Agent pool configuration"""
    min_size: int = Field(1, description="Minimum pool size")
    max_size: int = Field(10, description="Maximum pool size")
    idle_timeout: str = Field("5m", description="Idle agent timeout")
    max_agent_uses: int = Field(100, description="Max uses before agent retirement")
    warm_up: bool = Field(False, description="Pre-warm pool on startup")


class StepConfig(BaseModel):
    """Step configuration"""
    image: str = Field(..., description="Docker image for the step")
    command: Optional[List[str]] = Field(None, description="Override command")
    env_vars: Optional[Dict[str, str]] = Field(None, description="Environment variables")
    parallel: Optional[bool] = Field(False, description="Enable parallel execution")
    max_workers: Optional[int] = Field(None, description="Max parallel workers")
    dynamic: Optional[bool] = Field(False, description="Dynamic parallelism")
    reduce: Optional[bool] = Field(False, description="Is reduce step")
    timeout: Optional[str] = Field(None, description="Step timeout")
    retry_policy: Optional[RetryPolicy] = None
    resource_limits: Optional[ResourceLimits] = None
    execution_mode: Optional[str] = Field("standard", description="Execution mode")
    pool_config: Optional[PoolConfig] = None


class WorkflowConfig(BaseModel):
    """Workflow configuration"""
    max_parallel: Optional[int] = Field(None, description="Max parallel executions")
    timeout: Optional[str] = Field(None, description="Workflow timeout")
    retry_policy: Optional[RetryPolicy] = None
    failure_strategy: Optional[str] = Field("fail_fast", description="Failure handling")
    resource_limits: Optional[ResourceLimits] = None


class WorkflowStep(BaseModel):
    """Workflow step definition"""
    id: str = Field(..., description="Step ID")
    name: str = Field(..., description="Step name")
    type: StepType = Field(..., description="Step type")
    status: StepStatus = Field(StepStatus.PENDING, description="Step status")
    config: StepConfig = Field(..., description="Step configuration")
    depends_on: Optional[List[str]] = Field(None, description="Step dependencies")
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    error: Optional[str] = None
    results: Optional[Any] = None


class Workflow(BaseModel):
    """Workflow definition"""
    id: str = Field(..., description="Workflow ID")
    name: str = Field(..., description="Workflow name")
    description: Optional[str] = Field(None, description="Workflow description")
    status: WorkflowStatus = Field(WorkflowStatus.PENDING, description="Workflow status")
    config: WorkflowConfig = Field(..., description="Workflow configuration")
    steps: List[WorkflowStep] = Field(default_factory=list, description="Workflow steps")
    state: Dict[str, Any] = Field(default_factory=dict, description="Workflow state")
    metadata: Optional[Dict[str, str]] = Field(None, description="Additional metadata")
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None


class Agent(BaseModel):
    """Agent information"""
    id: str = Field(..., description="Agent ID")
    name: str = Field(..., description="Agent name")
    image: str = Field(..., description="Docker image")
    container_id: Optional[str] = Field(None, description="Container ID")
    status: AgentStatus = Field(..., description="Agent status")
    env_vars: Optional[Dict[str, str]] = Field(None, description="Environment variables")
    workflow_id: Optional[str] = Field(None, description="Associated workflow")
    step_id: Optional[str] = Field(None, description="Associated step")
    task_id: Optional[str] = Field(None, description="Task ID for parallel execution")
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)


class MapReduceConfig(BaseModel):
    """MapReduce pattern configuration"""
    name: str = Field(..., description="Workflow name")
    mapper_image: str = Field(..., description="Mapper Docker image")
    reducer_image: str = Field(..., description="Reducer Docker image")
    max_parallel: int = Field(10, description="Maximum parallel mappers")
    pool_size: Optional[int] = Field(None, description="Agent pool size")
    timeout: Optional[str] = Field("30m", description="Workflow timeout")
    error_strategy: Optional[str] = Field("fail_fast", description="Error handling")
    input_config: Optional[Dict[str, Any]] = Field(None, description="Input configuration")


class TriggerType(str, Enum):
    """Workflow trigger type"""
    SCHEDULE = "schedule"
    EVENT = "event"
    MANUAL = "manual"
    WEBHOOK = "webhook"


class TriggerConfig(BaseModel):
    """Trigger configuration"""
    # Schedule triggers
    cron_expression: Optional[str] = Field(None, description="Cron expression")
    timezone: Optional[str] = Field(None, description="Timezone for schedule")
    
    # Event triggers
    event_type: Optional[str] = Field(None, description="Event type to match")
    event_filter: Optional[Dict[str, Any]] = Field(None, description="Event filters")
    
    # Webhook triggers
    webhook_path: Optional[str] = Field(None, description="Webhook path")
    webhook_secret: Optional[str] = Field(None, description="Webhook secret")
    
    # Common
    skip_if_running: bool = Field(False, description="Skip if workflow already running")
    catch_up: bool = Field(False, description="Catch up missed runs")
    input_data: Optional[Dict[str, Any]] = Field(None, description="Input data for triggered runs")


class WorkflowTrigger(BaseModel):
    """Workflow trigger definition"""
    id: str = Field(..., description="Trigger ID")
    workflow_id: str = Field(..., description="Associated workflow ID")
    type: TriggerType = Field(..., description="Trigger type")
    config: TriggerConfig = Field(..., description="Trigger configuration")
    enabled: bool = Field(True, description="Whether trigger is enabled")
    last_run: Optional[datetime] = Field(None, description="Last execution time")
    next_run: Optional[datetime] = Field(None, description="Next scheduled run")
    run_count: int = Field(0, description="Number of executions")
    metadata: Optional[Dict[str, str]] = Field(None, description="Additional metadata")
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)