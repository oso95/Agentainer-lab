import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch
from agentainer_flow.context import Context, WorkflowContext, StateProxy, DeployedAgent
from agentainer_flow.models import Agent, AgentStatus


class TestStateProxy:
    """Test StateProxy class"""
    
    def test_state_proxy_get_set(self):
        """Test getting and setting state values"""
        state = {"key1": "value1"}
        update_fn = Mock()
        
        proxy = StateProxy(state, update_fn)
        
        # Test get
        assert proxy["key1"] == "value1"
        assert proxy.get("key2", "default") == "default"
        
        # Test set
        proxy["key2"] = "value2"
        assert proxy["key2"] == "value2"
        update_fn.assert_called_with("key2", "value2")
    
    def test_state_proxy_dict_methods(self):
        """Test dict-like methods"""
        state = {"a": 1, "b": 2}
        proxy = StateProxy(state, Mock())
        
        assert len(proxy) == 2
        assert "a" in proxy
        assert "c" not in proxy
        assert list(proxy.keys()) == ["a", "b"]
        assert list(proxy.values()) == [1, 2]
        assert list(proxy.items()) == [("a", 1), ("b", 2)]
    
    def test_state_proxy_update(self):
        """Test update method"""
        state = {"a": 1}
        update_fn = Mock()
        proxy = StateProxy(state, update_fn)
        
        proxy.update({"b": 2, "c": 3})
        
        assert proxy["b"] == 2
        assert proxy["c"] == 3
        assert update_fn.call_count == 2


class TestDeployedAgent:
    """Test DeployedAgent wrapper"""
    
    def test_deployed_agent_properties(self):
        """Test deployed agent property access"""
        agent = Agent(
            id="agent-123",
            name="test-agent",
            image="test:latest",
            status=AgentStatus.RUNNING,
            created_at=None,
            updated_at=None
        )
        
        client = Mock()
        deployed = DeployedAgent(agent, client)
        
        assert deployed.id == "agent-123"
        assert deployed.name == "test-agent"
        assert deployed.status == AgentStatus.RUNNING
    
    @pytest.mark.asyncio
    async def test_deployed_agent_start(self):
        """Test starting a deployed agent"""
        agent = Agent(
            id="agent-123",
            name="test-agent",
            image="test:latest",
            status=AgentStatus.CREATED,
            created_at=None,
            updated_at=None
        )
        
        client = AsyncMock()
        deployed = DeployedAgent(agent, client)
        
        await deployed.start()
        client.start_agent.assert_called_once_with("agent-123")
    
    @pytest.mark.asyncio
    async def test_deployed_agent_logs(self):
        """Test getting agent logs"""
        agent = Agent(
            id="agent-123",
            name="test-agent",
            image="test:latest",
            status=AgentStatus.RUNNING,
            created_at=None,
            updated_at=None
        )
        
        client = AsyncMock()
        client.get_agent_logs.return_value = "log output"
        deployed = DeployedAgent(agent, client)
        
        logs = await deployed.get_logs()
        assert logs == "log output"
        client.get_agent_logs.assert_called_once_with("agent-123", False)


class TestContext:
    """Test workflow Context"""
    
    def test_context_creation(self):
        """Test creating a context"""
        state = {"input": "data"}
        workflow_ctx = Mock()
        
        ctx = Context(
            workflow_id="wf-123",
            step_id="step-1",
            state=state,
            workflow_context=workflow_ctx
        )
        
        assert ctx.workflow_id == "wf-123"
        assert ctx.step_id == "step-1"
        assert isinstance(ctx.state, StateProxy)
        assert ctx.state["input"] == "data"
    
    @pytest.mark.asyncio
    async def test_deploy_agent(self):
        """Test deploying an agent from context"""
        workflow_ctx = Mock()
        workflow_ctx.client = AsyncMock()
        
        agent = Agent(
            id="agent-456",
            name="deployed",
            image="worker:latest",
            status=AgentStatus.CREATED,
            created_at=None,
            updated_at=None
        )
        workflow_ctx.client.deploy_agent.return_value = agent
        
        ctx = Context(
            workflow_id="wf-123",
            step_id="step-1",
            state={},
            workflow_context=workflow_ctx
        )
        
        deployed = await ctx.deploy_agent(
            name="deployed",
            image="worker:latest",
            env_vars={"KEY": "value"}
        )
        
        assert isinstance(deployed, DeployedAgent)
        assert deployed.id == "agent-456"
        
        workflow_ctx.client.deploy_agent.assert_called_once_with(
            name="deployed",
            image="worker:latest",
            env_vars={"KEY": "value"},
            workflow_id="wf-123",
            step_id="step-1"
        )
    
    def test_parallel_context(self):
        """Test parallel execution context"""
        ctx = Context(
            workflow_id="wf-123",
            step_id="step-1",
            state={},
            workflow_context=Mock(),
            task_id="task-5",
            parallel_index=5
        )
        
        assert ctx.task_id == "task-5"
        assert ctx.parallel_index == 5
        assert ctx.is_parallel is True


class TestWorkflowContext:
    """Test WorkflowContext"""
    
    @pytest.mark.asyncio
    async def test_update_state(self):
        """Test updating workflow state"""
        client = AsyncMock()
        wf_ctx = WorkflowContext("wf-123", client)
        
        await wf_ctx.update_state("key", "value")
        
        client.update_workflow_state.assert_called_once_with(
            "wf-123", "key", "value"
        )
    
    @pytest.mark.asyncio
    async def test_get_state(self):
        """Test getting workflow state"""
        client = AsyncMock()
        client.get_workflow_state.return_value = {"key": "value"}
        
        wf_ctx = WorkflowContext("wf-123", client)
        state = await wf_ctx.get_state()
        
        assert state == {"key": "value"}
        client.get_workflow_state.assert_called_once_with("wf-123")
    
    @pytest.mark.asyncio
    async def test_create_context_for_step(self):
        """Test creating context for a step"""
        client = AsyncMock()
        client.get_workflow_state.return_value = {"input": "data"}
        
        wf_ctx = WorkflowContext("wf-123", client)
        ctx = await wf_ctx.create_context_for_step("step-1")
        
        assert isinstance(ctx, Context)
        assert ctx.workflow_id == "wf-123"
        assert ctx.step_id == "step-1"
        assert ctx.state["input"] == "data"
        assert ctx._workflow_context == wf_ctx


if __name__ == "__main__":
    pytest.main([__file__])
