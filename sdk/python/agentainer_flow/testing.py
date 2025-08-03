"""
Testing utilities for Agentainer Flow workflows
"""

import asyncio
from typing import Any, Dict, List, Optional, Callable
from unittest.mock import MagicMock, AsyncMock
from contextlib import asynccontextmanager

from .models import Agent, AgentStatus, Workflow, WorkflowStatus
from .context import Context, DeployedAgent
from .exceptions import AgentainerError


class MockAgent:
    """Mock agent for testing"""
    
    def __init__(self, name: str, image: str, returns: Any = None):
        self.name = name
        self.image = image
        self.id = f"mock-{name}-{id(self)}"
        self.status = AgentStatus.RUNNING
        self.returns = returns or {"status": "success"}
        self.env_vars: Dict[str, str] = {}
        self.logs: List[str] = []
    
    async def wait(self, timeout: Optional[float] = None) -> Dict[str, Any]:
        """Simulate agent completion"""
        await asyncio.sleep(0.1)  # Simulate some work
        self.status = AgentStatus.STOPPED
        return self.returns
    
    async def stop(self):
        """Stop the agent"""
        self.status = AgentStatus.STOPPED
    
    def add_log(self, message: str):
        """Add a log message"""
        self.logs.append(message)


class MockContext(Context):
    """Mock context for testing"""
    
    def __init__(self, workflow_id: str = "test-workflow", step_id: str = "test-step"):
        self.workflow_id = workflow_id
        self.step_id = step_id
        self.state = {}
        self._deployed_agents: List[MockAgent] = []
        self._agent_configs: Dict[str, Dict[str, Any]] = {}
    
    async def deploy_agent(
        self,
        name: str,
        image: str,
        env: Optional[Dict[str, str]] = None,
        **kwargs
    ) -> DeployedAgent:
        """Deploy a mock agent"""
        # Check if we have a mock configured for this image
        mock_config = self._agent_configs.get(image, {})
        
        agent = MockAgent(
            name=name,
            image=image,
            returns=mock_config.get("returns", {"status": "success"})
        )
        
        if env:
            agent.env_vars = env
        
        self._deployed_agents.append(agent)
        
        # Create a mock deployed agent
        deployed = MagicMock(spec=DeployedAgent)
        deployed.id = agent.id
        deployed.name = agent.name
        deployed.status = agent.status
        deployed.wait = agent.wait
        deployed.stop = agent.stop
        deployed.agent = agent
        
        return deployed
    
    def configure_agent(self, image: str, returns: Any = None):
        """Configure mock behavior for an agent image"""
        self._agent_configs[image] = {"returns": returns}


class WorkflowTestRunner:
    """Test runner for workflows"""
    
    def __init__(self):
        self.executed_steps: List[Dict[str, Any]] = []
        self.state: Dict[str, Any] = {}
    
    async def run_workflow(
        self,
        workflow_instance: Any,
        input_data: Optional[Dict[str, Any]] = None,
        mock_agents: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Run a workflow locally for testing"""
        # Get workflow metadata
        if not hasattr(workflow_instance.__class__, "_workflow_meta"):
            raise AgentainerError("Class is not decorated with @workflow")
        
        meta = workflow_instance.__class__._workflow_meta
        
        # Initialize state with input data
        if input_data:
            self.state.update(input_data)
        
        # Create mock context
        context = MockContext()
        context.state = self.state
        
        # Configure mock agents
        if mock_agents:
            for image, config in mock_agents.items():
                context.configure_agent(image, config)
        
        # Execute steps in order (simplified - no dependency resolution)
        results = {}
        for step_name, step_meta in meta.steps.items():
            # Record step execution
            self.executed_steps.append({
                "name": step_name,
                "type": step_meta.step_type,
                "config": step_meta.config,
            })
            
            # Execute step
            try:
                # Call the step method
                if asyncio.iscoroutinefunction(step_meta.func):
                    result = await step_meta.func(workflow_instance, context)
                else:
                    result = step_meta.func(workflow_instance, context)
                
                results[step_name] = result
                
                # Save result to state
                self.state[f"step_{step_name}_result"] = result
                
            except Exception as e:
                results[step_name] = {"error": str(e)}
                raise
        
        return {
            "status": "completed",
            "state": self.state,
            "results": results,
            "executed_steps": self.executed_steps,
        }
    
    async def run_step(
        self,
        workflow_instance: Any,
        step_name: str,
        context: Optional[Context] = None,
        **kwargs
    ) -> Any:
        """Run a single workflow step"""
        meta = workflow_instance.__class__._workflow_meta
        
        if step_name not in meta.steps:
            raise ValueError(f"Step '{step_name}' not found in workflow")
        
        step_meta = meta.steps[step_name]
        
        if context is None:
            context = MockContext()
        
        # Execute step
        if asyncio.iscoroutinefunction(step_meta.func):
            return await step_meta.func(workflow_instance, context, **kwargs)
        else:
            return step_meta.func(workflow_instance, context, **kwargs)


@asynccontextmanager
async def mock_agent(image: str, returns: Any = None):
    """Context manager for mocking agents"""
    mock = MockAgent(image, image, returns)
    yield mock


async def run_workflow_locally(
    workflow_instance: Any,
    input_data: Optional[Dict[str, Any]] = None,
    mock_agents: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """Convenience function to run a workflow locally"""
    runner = WorkflowTestRunner()
    return await runner.run_workflow(workflow_instance, input_data, mock_agents)


class WorkflowAssertions:
    """Assertions for workflow testing"""
    
    @staticmethod
    def assert_step_executed(results: Dict[str, Any], step_name: str):
        """Assert that a step was executed"""
        executed_steps = results.get("executed_steps", [])
        step_names = [step["name"] for step in executed_steps]
        
        if step_name not in step_names:
            raise AssertionError(f"Step '{step_name}' was not executed. Executed: {step_names}")
    
    @staticmethod
    def assert_state_contains(results: Dict[str, Any], key: str, expected_value: Any = None):
        """Assert that workflow state contains a key"""
        state = results.get("state", {})
        
        if key not in state:
            raise AssertionError(f"State does not contain key '{key}'. State: {state}")
        
        if expected_value is not None and state[key] != expected_value:
            raise AssertionError(
                f"State['{key}'] = {state[key]}, expected {expected_value}"
            )
    
    @staticmethod
    def assert_step_result(results: Dict[str, Any], step_name: str, expected: Any):
        """Assert step result matches expected value"""
        step_results = results.get("results", {})
        
        if step_name not in step_results:
            raise AssertionError(f"No result found for step '{step_name}'")
        
        actual = step_results[step_name]
        if actual != expected:
            raise AssertionError(
                f"Step '{step_name}' returned {actual}, expected {expected}"
            )