"""
Workflow execution context
"""

import asyncio
from typing import Any, Dict, List, Optional, Union
from .models import Agent, AgentStatus
from .exceptions import StateError, AgentainerError


class StateProxy:
    """Proxy for workflow state with dict-like interface"""
    
    def __init__(self, workflow_id: str, client):
        self.workflow_id = workflow_id
        self.client = client
        self._cache: Dict[str, Any] = {}
    
    def __getitem__(self, key: str) -> Any:
        """Get state value"""
        if key in self._cache:
            return self._cache[key]
        
        # Fetch from server
        value = asyncio.run_coroutine_threadsafe(
            self.client.get_workflow_state(self.workflow_id, key),
            self.client.loop
        ).result()
        
        self._cache[key] = value
        return value
    
    def __setitem__(self, key: str, value: Any):
        """Set state value"""
        asyncio.run_coroutine_threadsafe(
            self.client.update_workflow_state(self.workflow_id, key, value),
            self.client.loop
        ).result()
        
        self._cache[key] = value
    
    def get(self, key: str, default: Any = None) -> Any:
        """Get state value with default"""
        try:
            return self[key]
        except KeyError:
            return default
    
    async def increment(self, key: str, delta: int = 1) -> int:
        """Atomically increment a counter"""
        result = await self.client.increment_state(self.workflow_id, key, delta)
        self._cache[key] = result
        return result
    
    async def append_to_list(self, key: str, value: Any):
        """Append to a list in state"""
        await self.client.append_to_state_list(self.workflow_id, key, value)
        
        # Invalidate cache for this key
        if key in self._cache:
            del self._cache[key]
    
    async def add_to_set(self, key: str, value: Any):
        """Add to a set in state"""
        await self.client.add_to_state_set(self.workflow_id, key, value)
        
        # Invalidate cache
        if key in self._cache:
            del self._cache[key]


class DeployedAgent:
    """Represents a deployed agent in a workflow"""
    
    def __init__(self, agent: Agent, client):
        self.agent = agent
        self.client = client
    
    @property
    def id(self) -> str:
        return self.agent.id
    
    @property
    def name(self) -> str:
        return self.agent.name
    
    @property
    def status(self) -> AgentStatus:
        return self.agent.status
    
    async def wait(self, timeout: Optional[float] = None) -> Dict[str, Any]:
        """Wait for agent to complete and return results"""
        # Poll agent status until completed
        while True:
            agent = await self.client.get_agent(self.agent.id)
            
            if agent.status in [AgentStatus.STOPPED, AgentStatus.FAILED]:
                # In a real implementation, we'd fetch the agent's output
                # For now, return a mock result
                return {
                    "status": "completed" if agent.status == AgentStatus.STOPPED else "failed",
                    "agent_id": agent.id,
                    "output": {}
                }
            
            await asyncio.sleep(1)
    
    async def stop(self):
        """Stop the agent"""
        await self.client.stop_agent(self.agent.id)
    
    async def logs(self, follow: bool = False) -> str:
        """Get agent logs"""
        return await self.client.get_agent_logs(self.agent.id, follow=follow)


class Context:
    """Workflow execution context"""
    
    def __init__(self, workflow_id: str, step_id: str, client):
        self.workflow_id = workflow_id
        self.step_id = step_id
        self.client = client
        self.state = StateProxy(workflow_id, client)
        self._deployed_agents: List[DeployedAgent] = []
    
    async def deploy_agent(
        self,
        name: str,
        image: str,
        env: Optional[Dict[str, str]] = None,
        cpu_limit: Optional[int] = None,
        memory_limit: Optional[int] = None,
        **kwargs
    ) -> DeployedAgent:
        """Deploy an agent for this workflow step"""
        # Add workflow metadata to environment
        if env is None:
            env = {}
        
        env.update({
            "WORKFLOW_ID": self.workflow_id,
            "STEP_ID": self.step_id,
        })
        
        # Deploy agent through client
        agent = await self.client.deploy_agent(
            name=name,
            image=image,
            env_vars=env,
            cpu_limit=cpu_limit,
            memory_limit=memory_limit,
            workflow_id=self.workflow_id,
            step_id=self.step_id,
            **kwargs
        )
        
        # Start the agent
        await self.client.start_agent(agent.id)
        
        # Create deployed agent wrapper
        deployed = DeployedAgent(agent, self.client)
        self._deployed_agents.append(deployed)
        
        return deployed
    
    async def get_previous_results(self, step_name: str) -> Any:
        """Get results from a previous step"""
        key = f"step_{step_name}_results"
        return self.state.get(key)
    
    async def save_results(self, results: Any):
        """Save step results to state"""
        key = f"step_{self.step_id}_results"
        self.state[key] = results
    
    async def cleanup(self):
        """Clean up deployed agents"""
        for agent in self._deployed_agents:
            try:
                await agent.stop()
            except Exception:
                pass  # Best effort cleanup


class WorkflowContext(Context):
    """Extended context for workflow-level operations"""
    
    def __init__(self, workflow_id: str, client):
        super().__init__(workflow_id, "workflow", client)
        self.steps_completed = 0
        self.steps_failed = 0
    
    async def mark_step_completed(self, step_id: str):
        """Mark a step as completed"""
        self.steps_completed += 1
        await self.state.increment("steps_completed")
    
    async def mark_step_failed(self, step_id: str, error: str):
        """Mark a step as failed"""
        self.steps_failed += 1
        await self.state.increment("steps_failed")
        await self.state.append_to_list("step_errors", {
            "step_id": step_id,
            "error": error
        })