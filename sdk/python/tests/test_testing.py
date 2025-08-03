import pytest
import asyncio
from agentainer_flow import workflow, step
from agentainer_flow.testing import (
    MockAgent, MockContext, WorkflowTestRunner, 
    WorkflowAssertions, mock_agent
)


class TestMockAgent:
    """Test MockAgent functionality"""
    
    @pytest.mark.asyncio
    async def test_mock_agent_basic(self):
        """Test basic mock agent behavior"""
        agent = MockAgent(
            agent_id="mock-123",
            name="test-agent",
            return_value="result"
        )
        
        assert agent.id == "mock-123"
        assert agent.name == "test-agent"
        assert agent.status == "running"
        
        # Test async methods
        await agent.start()
        await agent.stop()
        
        logs = await agent.get_logs()
        assert logs == "Mock logs for test-agent"
    
    @pytest.mark.asyncio
    async def test_mock_agent_with_exception(self):
        """Test mock agent that raises exception"""
        agent = MockAgent(
            agent_id="mock-error",
            name="error-agent",
            side_effect=ValueError("Test error")
        )
        
        with pytest.raises(ValueError, match="Test error"):
            await agent.start()
    
    @pytest.mark.asyncio
    async def test_mock_agent_context_manager(self):
        """Test using mock_agent context manager"""
        async with mock_agent("test:latest", return_value="mocked") as agent:
            assert agent.image == "test:latest"
            assert isinstance(agent, MockAgent)


class TestMockContext:
    """Test MockContext functionality"""
    
    def test_mock_context_state(self):
        """Test mock context state management"""
        ctx = MockContext(
            workflow_id="wf-test",
            step_id="step-1",
            initial_state={"key": "value"}
        )
        
        assert ctx.workflow_id == "wf-test"
        assert ctx.step_id == "step-1"
        assert ctx.state["key"] == "value"
        
        # Test state updates
        ctx.state["new_key"] = "new_value"
        assert ctx.state["new_key"] == "new_value"
    
    @pytest.mark.asyncio
    async def test_mock_context_deploy_agent(self):
        """Test deploying mock agents from context"""
        ctx = MockContext(
            workflow_id="wf-test",
            step_id="step-1"
        )
        
        agent = await ctx.deploy_agent(
            name="worker",
            image="worker:latest"
        )
        
        assert isinstance(agent, MockAgent)
        assert agent.name == "worker"
        assert agent.image == "worker:latest"
        assert agent.id in ctx.deployed_agents
    
    def test_mock_context_with_mocks(self):
        """Test mock context with predefined mocks"""
        mock1 = MockAgent("mock-1", "agent1")
        mock2 = MockAgent("mock-2", "agent2")
        
        ctx = MockContext(
            workflow_id="wf-test",
            step_id="step-1",
            agent_mocks={"worker:latest": [mock1, mock2]}
        )
        
        # Should return predefined mocks in order
        agent1 = asyncio.run(ctx.deploy_agent("first", "worker:latest"))
        agent2 = asyncio.run(ctx.deploy_agent("second", "worker:latest"))
        
        assert agent1 == mock1
        assert agent2 == mock2


class TestWorkflowTestRunner:
    """Test WorkflowTestRunner"""
    
    @pytest.mark.asyncio
    async def test_run_workflow(self):
        """Test running a workflow with test runner"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def step1(self, ctx):
                ctx.state["step1_done"] = True
                return "step1_result"
            
            @step(depends_on=["step1"])
            async def step2(self, ctx):
                ctx.state["step2_done"] = True
                return "step2_result"
        
        runner = WorkflowTestRunner()
        workflow_instance = TestWorkflow()
        
        result = await runner.run_workflow(
            workflow_instance,
            initial_state={"input": "data"}
        )
        
        # Check execution completed
        assert result.success is True
        assert result.completed_steps == ["step1", "step2"]
        assert result.final_state["step1_done"] is True
        assert result.final_state["step2_done"] is True
    
    @pytest.mark.asyncio
    async def test_run_single_step(self):
        """Test running a single step"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def process(self, ctx):
                ctx.state["processed"] = ctx.state.get("input", "").upper()
                return ctx.state["processed"]
        
        runner = WorkflowTestRunner()
        workflow_instance = TestWorkflow()
        
        result = await runner.run_step(
            workflow_instance,
            "process",
            initial_state={"input": "hello"}
        )
        
        assert result == "HELLO"
    
    @pytest.mark.asyncio
    async def test_workflow_with_mock_agents(self):
        """Test workflow with mock agents"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def deploy_workers(self, ctx):
                agents = []
                for i in range(3):
                    agent = await ctx.deploy_agent(
                        name=f"worker-{i}",
                        image="worker:latest"
                    )
                    agents.append(agent)
                ctx.state["agents"] = [a.id for a in agents]
        
        runner = WorkflowTestRunner()
        workflow_instance = TestWorkflow()
        
        # Configure mock agents
        mocks = [
            MockAgent(f"mock-{i}", f"worker-{i}", return_value=f"result-{i}")
            for i in range(3)
        ]
        
        result = await runner.run_workflow(
            workflow_instance,
            agent_mocks={"worker:latest": mocks}
        )
        
        assert result.success is True
        assert len(result.final_state["agents"]) == 3
        assert result.final_state["agents"][0] == "mock-0"


class TestWorkflowAssertions:
    """Test WorkflowAssertions helper"""
    
    def test_assert_step_completed(self):
        """Test step completion assertion"""
        result = MockExecutionResult(
            completed_steps=["step1", "step2"]
        )
        
        assertions = WorkflowAssertions(result)
        assertions.assert_step_completed("step1")
        assertions.assert_step_completed("step2")
        
        with pytest.raises(AssertionError):
            assertions.assert_step_completed("step3")
    
    def test_assert_state_contains(self):
        """Test state assertion"""
        result = MockExecutionResult(
            final_state={"key1": "value1", "key2": 42}
        )
        
        assertions = WorkflowAssertions(result)
        assertions.assert_state_contains("key1", "value1")
        assertions.assert_state_contains("key2", 42)
        
        with pytest.raises(AssertionError):
            assertions.assert_state_contains("key3", "value3")
        
        with pytest.raises(AssertionError):
            assertions.assert_state_contains("key1", "wrong_value")
    
    def test_assert_agents_deployed(self):
        """Test agent deployment assertion"""
        mock_ctx = MockContext("wf-1", "step-1")
        # Simulate deploying agents
        agent1 = asyncio.run(mock_ctx.deploy_agent("agent1", "image1"))
        agent2 = asyncio.run(mock_ctx.deploy_agent("agent2", "image2"))
        
        result = MockExecutionResult(
            contexts={"step1": mock_ctx}
        )
        
        assertions = WorkflowAssertions(result)
        assertions.assert_agents_deployed(2)
        assertions.assert_agents_deployed(2, step="step1")
        
        with pytest.raises(AssertionError):
            assertions.assert_agents_deployed(3)
    
    def test_assert_no_errors(self):
        """Test error assertion"""
        result = MockExecutionResult(
            success=True,
            errors=[]
        )
        
        assertions = WorkflowAssertions(result)
        assertions.assert_no_errors()
        
        result_with_errors = MockExecutionResult(
            success=False,
            errors=["Error 1", "Error 2"]
        )
        
        assertions_with_errors = WorkflowAssertions(result_with_errors)
        with pytest.raises(AssertionError, match="Workflow had 2 errors"):
            assertions_with_errors.assert_no_errors()


# Helper class for testing
class MockExecutionResult:
    def __init__(self, **kwargs):
        self.success = kwargs.get("success", True)
        self.completed_steps = kwargs.get("completed_steps", [])
        self.failed_steps = kwargs.get("failed_steps", [])
        self.final_state = kwargs.get("final_state", {})
        self.errors = kwargs.get("errors", [])
        self.contexts = kwargs.get("contexts", {})
        self.execution_time = kwargs.get("execution_time", 0)


if __name__ == "__main__":
    pytest.main([__file__])
