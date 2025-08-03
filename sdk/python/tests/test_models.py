import pytest
from datetime import datetime
from agentainer_flow.models import (
    Workflow, WorkflowStatus, WorkflowConfig,
    WorkflowStep, StepStatus, StepType, StepConfig,
    Agent, AgentStatus,
    TriggerType, TriggerConfig, WorkflowTrigger,
    MapReduceConfig, RetryPolicy, ResourceLimits, PoolConfig
)


class TestWorkflowModels:
    """Test workflow data models"""
    
    def test_workflow_creation(self):
        """Test creating a workflow model"""
        workflow = Workflow(
            id="wf-123",
            name="test-workflow",
            description="Test workflow",
            status=WorkflowStatus.PENDING,
            config=WorkflowConfig(),
            steps=[],
            state={},
            created_at=datetime.utcnow(),
            updated_at=datetime.utcnow()
        )
        
        assert workflow.id == "wf-123"
        assert workflow.name == "test-workflow"
        assert workflow.status == WorkflowStatus.PENDING
        assert isinstance(workflow.config, WorkflowConfig)
    
    def test_workflow_status_enum(self):
        """Test workflow status values"""
        assert WorkflowStatus.PENDING == "pending"
        assert WorkflowStatus.RUNNING == "running"
        assert WorkflowStatus.COMPLETED == "completed"
        assert WorkflowStatus.FAILED == "failed"
        assert WorkflowStatus.CANCELLED == "cancelled"
    
    def test_workflow_config_defaults(self):
        """Test workflow config default values"""
        config = WorkflowConfig()
        
        assert config.failure_strategy == "fail_fast"
        assert config.max_parallel is None
        assert config.timeout is None
        assert config.retry_policy is None


class TestStepModels:
    """Test workflow step models"""
    
    def test_step_creation(self):
        """Test creating a workflow step"""
        step = WorkflowStep(
            id="step-1",
            name="process-data",
            type=StepType.SEQUENTIAL,
            status=StepStatus.PENDING,
            config=StepConfig(image="processor:latest")
        )
        
        assert step.id == "step-1"
        assert step.name == "process-data"
        assert step.type == StepType.SEQUENTIAL
        assert step.config.image == "processor:latest"
    
    def test_step_type_enum(self):
        """Test step type values"""
        assert StepType.SEQUENTIAL == "sequential"
        assert StepType.PARALLEL == "parallel"
        assert StepType.REDUCE == "reduce"
        assert StepType.MAPREDUCE == "mapreduce"
    
    def test_step_config_with_pooling(self):
        """Test step config with pooling enabled"""
        pool_config = PoolConfig(
            min_size=5,
            max_size=20,
            idle_timeout="10m",
            max_agent_uses=100,
            warm_up=True
        )
        
        config = StepConfig(
            image="worker:latest",
            execution_mode="pooled",
            pool_config=pool_config
        )
        
        assert config.execution_mode == "pooled"
        assert config.pool_config.max_size == 20
        assert config.pool_config.warm_up is True
    
    def test_retry_policy(self):
        """Test retry policy configuration"""
        retry = RetryPolicy(
            max_attempts=5,
            backoff="exponential",
            delay="2s"
        )
        
        assert retry.max_attempts == 5
        assert retry.backoff == "exponential"
        assert retry.delay == "2s"


class TestAgentModels:
    """Test agent models"""
    
    def test_agent_creation(self):
        """Test creating an agent model"""
        agent = Agent(
            id="agent-123",
            name="worker-1",
            image="worker:latest",
            status=AgentStatus.CREATED,
            created_at=datetime.utcnow()
        )
        
        assert agent.id == "agent-123"
        assert agent.name == "worker-1"
        assert agent.status == AgentStatus.CREATED
    
    def test_agent_with_workflow_metadata(self):
        """Test agent with workflow metadata"""
        agent = Agent(
            id="agent-123",
            name="worker-1",
            image="worker:latest",
            status=AgentStatus.RUNNING,
            workflow_id="wf-456",
            step_id="step-789",
            task_id="task-1",
            created_at=datetime.utcnow()
        )
        
        assert agent.workflow_id == "wf-456"
        assert agent.step_id == "step-789"
        assert agent.task_id == "task-1"


class TestTriggerModels:
    """Test trigger models"""
    
    def test_schedule_trigger(self):
        """Test creating a schedule trigger"""
        config = TriggerConfig(
            cron_expression="0 */5 * * * *",
            timezone="UTC",
            skip_if_running=True
        )
        
        trigger = WorkflowTrigger(
            id="trigger-1",
            workflow_id="wf-123",
            type=TriggerType.SCHEDULE,
            config=config,
            enabled=True,
            created_at=datetime.utcnow()
        )
        
        assert trigger.type == TriggerType.SCHEDULE
        assert trigger.config.cron_expression == "0 */5 * * * *"
        assert trigger.config.skip_if_running is True
    
    def test_event_trigger(self):
        """Test creating an event trigger"""
        config = TriggerConfig(
            event_type="data_uploaded",
            event_filter={"bucket": "input-data"},
            input_data={"process_type": "batch"}
        )
        
        trigger = WorkflowTrigger(
            id="trigger-2",
            workflow_id="wf-123",
            type=TriggerType.EVENT,
            config=config,
            enabled=True,
            created_at=datetime.utcnow()
        )
        
        assert trigger.type == TriggerType.EVENT
        assert trigger.config.event_type == "data_uploaded"
        assert trigger.config.event_filter["bucket"] == "input-data"
    
    def test_webhook_trigger(self):
        """Test creating a webhook trigger"""
        config = TriggerConfig(
            webhook_path="/webhooks/github",
            webhook_secret="secret123"
        )
        
        trigger = WorkflowTrigger(
            id="trigger-3",
            workflow_id="wf-123",
            type=TriggerType.WEBHOOK,
            config=config,
            enabled=False,
            created_at=datetime.utcnow()
        )
        
        assert trigger.type == TriggerType.WEBHOOK
        assert trigger.config.webhook_path == "/webhooks/github"
        assert trigger.enabled is False


class TestMapReduceConfig:
    """Test MapReduce configuration"""
    
    def test_mapreduce_config(self):
        """Test MapReduce configuration model"""
        config = MapReduceConfig(
            name="data-processing",
            mapper_image="mapper:latest",
            reducer_image="reducer:latest",
            max_parallel=20,
            pool_size=15,
            timeout="1h",
            error_strategy="continue_on_partial"
        )
        
        assert config.name == "data-processing"
        assert config.mapper_image == "mapper:latest"
        assert config.reducer_image == "reducer:latest"
        assert config.max_parallel == 20
        assert config.pool_size == 15
    
    def test_mapreduce_defaults(self):
        """Test MapReduce default values"""
        config = MapReduceConfig(
            name="test",
            mapper_image="mapper:latest",
            reducer_image="reducer:latest"
        )
        
        assert config.max_parallel == 10
        assert config.timeout == "30m"
        assert config.error_strategy == "fail_fast"


class TestResourceLimits:
    """Test resource limit models"""
    
    def test_resource_limits(self):
        """Test resource limits configuration"""
        limits = ResourceLimits(
            cpu_limit=2000000000,  # 2 CPU cores
            memory_limit=4294967296  # 4GB
        )
        
        assert limits.cpu_limit == 2000000000
        assert limits.memory_limit == 4294967296
    
    def test_resource_limits_optional(self):
        """Test resource limits are optional"""
        limits = ResourceLimits()
        
        assert limits.cpu_limit is None
        assert limits.memory_limit is None


if __name__ == "__main__":
    pytest.main([__file__])
