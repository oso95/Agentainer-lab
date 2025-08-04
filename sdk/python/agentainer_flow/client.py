"""
Client for Agentainer Flow API
"""

import asyncio
import json
from typing import Any, Dict, List, Optional, Type, Union
from datetime import datetime

import aiohttp
from aiohttp import ClientTimeout

from .models import (
    Agent,
    Workflow,
    WorkflowStatus,
    MapReduceConfig,
    WorkflowTrigger,
    TriggerType,
    TriggerConfig,
)
from .context import Context, WorkflowContext
from .decorators import StepType
from .exceptions import ClientError, WorkflowError


class WorkflowExecution:
    """Handle for a running workflow"""
    
    def __init__(self, workflow: Workflow, client: "WorkflowClient"):
        self.workflow = workflow
        self.client = client
        self._context = WorkflowContext(workflow.id, client)
    
    @property
    def id(self) -> str:
        return self.workflow.id
    
    @property
    def status(self) -> WorkflowStatus:
        return self.workflow.status
    
    async def wait_for_completion(self, timeout: Optional[float] = None) -> Dict[str, Any]:
        """Wait for workflow to complete"""
        start_time = asyncio.get_event_loop().time()
        
        while True:
            # Refresh workflow status
            self.workflow = await self.client.get_workflow(self.workflow.id)
            
            if self.workflow.status in [
                WorkflowStatus.COMPLETED,
                WorkflowStatus.FAILED,
                WorkflowStatus.CANCELLED
            ]:
                return {
                    "status": self.workflow.status,
                    "workflow_id": self.workflow.id,
                    "state": self.workflow.state,
                    "duration": datetime.utcnow() - self.workflow.created_at
                }
            
            # Check timeout
            if timeout and (asyncio.get_event_loop().time() - start_time) > timeout:
                raise TimeoutError(f"Workflow {self.workflow.id} did not complete within {timeout}s")
            
            await asyncio.sleep(2)
    
    async def cancel(self):
        """Cancel the workflow"""
        # In a real implementation, would call cancel endpoint
        raise NotImplementedError("Workflow cancellation not yet implemented")
    
    async def get_jobs(self) -> List[Agent]:
        """Get all jobs (agents) for this workflow"""
        return await self.client.get_workflow_jobs(self.workflow.id)
    
    async def get_state(self) -> Dict[str, Any]:
        """Get workflow state"""
        workflow = await self.client.get_workflow(self.workflow.id)
        return workflow.state
    
    async def get_metrics(self) -> Dict[str, Any]:
        """Get execution metrics"""
        return await self.client.get_workflow_metrics(self.workflow.id)
    
    async def monitor(self, callback=None, interval: float = 2.0):
        """Monitor workflow execution with optional callback
        
        Args:
            callback: Optional function to call with metrics on each update
            interval: Update interval in seconds
        """
        while True:
            # Refresh workflow status
            self.workflow = await self.client.get_workflow(self.workflow.id)
            
            # Get current metrics
            metrics = await self.get_metrics()
            
            # Call callback if provided
            if callback:
                callback(self.workflow, metrics)
            
            # Check if workflow is done
            if self.workflow.status in [
                WorkflowStatus.COMPLETED,
                WorkflowStatus.FAILED,
                WorkflowStatus.CANCELLED
            ]:
                return metrics
            
            await asyncio.sleep(interval)


class WorkflowClient:
    """Client for Agentainer Flow API"""
    
    def __init__(
        self,
        base_url: str = "http://localhost:8081",
        token: Optional[str] = None,
        timeout: float = 30.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.token = token or "default-token"
        self.timeout = ClientTimeout(total=timeout)
        self._session: Optional[aiohttp.ClientSession] = None
        self.loop = asyncio.get_event_loop()
    
    async def __aenter__(self):
        """Async context manager entry"""
        self._session = aiohttp.ClientSession(
            timeout=self.timeout,
            headers={"Authorization": f"Bearer {self.token}"}
        )
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit"""
        if self._session:
            await self._session.close()
    
    @property
    def session(self) -> aiohttp.ClientSession:
        """Get or create session"""
        if not self._session:
            self._session = aiohttp.ClientSession(
                timeout=self.timeout,
                headers={"Authorization": f"Bearer {self.token}"}
            )
        return self._session
    
    async def _request(
        self,
        method: str,
        endpoint: str,
        json_data: Optional[Dict[str, Any]] = None,
        **kwargs
    ) -> Dict[str, Any]:
        """Make HTTP request to API"""
        url = f"{self.base_url}{endpoint}"
        
        try:
            async with self.session.request(
                method,
                url,
                json=json_data,
                **kwargs
            ) as response:
                data = await response.json()
                
                if response.status >= 400:
                    error_msg = data.get("message", "Unknown error")
                    raise ClientError(f"API error ({response.status}): {error_msg}")
                
                return data
                
        except aiohttp.ClientError as e:
            raise ClientError(f"Network error: {str(e)}")
    
    # Workflow management
    
    async def create_workflow(
        self,
        name: str,
        description: Optional[str] = None,
        config: Optional[Dict[str, Any]] = None,
    ) -> Workflow:
        """Create a new workflow"""
        data = await self._request(
            "POST",
            "/workflows",
            {
                "name": name,
                "description": description,
                "config": config or {}
            }
        )
        return Workflow(**data["data"])
    
    async def get_workflow(self, workflow_id: str) -> Workflow:
        """Get workflow details"""
        data = await self._request("GET", f"/workflows/{workflow_id}")
        return Workflow(**data["data"])
    
    async def list_workflows(
        self,
        status: Optional[WorkflowStatus] = None
    ) -> List[Workflow]:
        """List workflows"""
        params = {}
        if status:
            params["status"] = status.value
        
        data = await self._request("GET", "/workflows", params=params)
        return [Workflow(**w) for w in data["data"]]
    
    async def start_workflow(self, workflow_id: str) -> Dict[str, Any]:
        """Start workflow execution"""
        data = await self._request("POST", f"/workflows/{workflow_id}/start")
        return data["data"]
    
    async def get_workflow_jobs(self, workflow_id: str) -> List[Agent]:
        """Get all jobs for a workflow"""
        data = await self._request("GET", f"/workflows/{workflow_id}/jobs")
        return [Agent(**a) for a in data["data"]]
    
    # State management
    
    async def get_workflow_state(
        self,
        workflow_id: str,
        key: Optional[str] = None
    ) -> Union[Dict[str, Any], Any]:
        """Get workflow state"""
        endpoint = f"/workflows/{workflow_id}/state"
        if key:
            endpoint += f"?key={key}"
        
        data = await self._request("GET", endpoint)
        return data["data"]
    
    async def update_workflow_state(
        self,
        workflow_id: str,
        key: str,
        value: Any
    ):
        """Update workflow state"""
        await self._request(
            "PUT",
            f"/workflows/{workflow_id}/state",
            {"key": key, "value": value}
        )
    
    # Agent management
    
    async def deploy_agent(
        self,
        name: str,
        image: str,
        env_vars: Optional[Dict[str, str]] = None,
        cpu_limit: Optional[int] = None,
        memory_limit: Optional[int] = None,
        workflow_id: Optional[str] = None,
        step_id: Optional[str] = None,
        **kwargs
    ) -> Agent:
        """Deploy an agent"""
        payload = {
            "name": name,
            "image": image,
            "env_vars": env_vars or {},
            "cpu_limit": cpu_limit,
            "memory_limit": memory_limit,
        }
        
        # Add workflow metadata if provided
        if workflow_id:
            payload["env_vars"]["WORKFLOW_ID"] = workflow_id
        if step_id:
            payload["env_vars"]["STEP_ID"] = step_id
        
        data = await self._request("POST", "/agents", payload)
        return Agent(**data["data"])
    
    async def start_agent(self, agent_id: str):
        """Start an agent"""
        await self._request("POST", f"/agents/{agent_id}/start")
    
    async def stop_agent(self, agent_id: str):
        """Stop an agent"""
        await self._request("POST", f"/agents/{agent_id}/stop")
    
    async def get_agent(self, agent_id: str) -> Agent:
        """Get agent details"""
        data = await self._request("GET", f"/agents/{agent_id}")
        return Agent(**data["data"])
    
    async def get_agent_logs(
        self,
        agent_id: str,
        follow: bool = False
    ) -> str:
        """Get agent logs"""
        params = {"follow": str(follow).lower()}
        data = await self._request("GET", f"/agents/{agent_id}/logs", params=params)
        return data["data"]
    
    # High-level workflow execution
    
    async def run_workflow(
        self,
        workflow_instance: Any,
        input_data: Optional[Dict[str, Any]] = None
    ) -> WorkflowExecution:
        """Run a workflow defined with decorators"""
        # Extract workflow metadata
        if not hasattr(workflow_instance.__class__, "_workflow_meta"):
            raise WorkflowError("Class is not decorated with @workflow")
        
        meta = workflow_instance.__class__._workflow_meta
        
        # Create workflow
        workflow = await self.create_workflow(
            name=meta.name,
            description=getattr(workflow_instance.__class__, "_workflow_description", None),
            config=meta.config.dict(exclude_none=True)
        )
        
        # Add input data to state if provided
        if input_data:
            for key, value in input_data.items():
                await self.update_workflow_state(workflow.id, key, value)
        
        # Build and add steps
        for step_name, step_meta in meta.steps.items():
            step_config = step_meta.config.dict(exclude_none=True)
            
            # Add step via API (would need an endpoint for this)
            # For now, we'll start the workflow and let the orchestrator handle it
        
        # Start workflow
        await self.start_workflow(workflow.id)
        
        return WorkflowExecution(workflow, self)
    
    async def run_mapreduce(
        self,
        config: MapReduceConfig
    ) -> WorkflowExecution:
        """Run a MapReduce workflow"""
        data = await self._request(
            "POST",
            "/workflows/mapreduce",
            config.dict(exclude_none=True)
        )
        
        workflow = Workflow(**data["data"])
        return WorkflowExecution(workflow, self)
    
    # Trigger management
    
    async def create_trigger(
        self,
        workflow_id: str,
        trigger_type: TriggerType,
        config: TriggerConfig
    ) -> WorkflowTrigger:
        """Create a workflow trigger"""
        data = await self._request(
            "POST",
            f"/workflows/{workflow_id}/triggers",
            {
                "type": trigger_type.value,
                "config": config.dict(exclude_none=True)
            }
        )
        return WorkflowTrigger(**data["data"])
    
    async def list_triggers(
        self,
        workflow_id: str
    ) -> List[WorkflowTrigger]:
        """List all triggers for a workflow"""
        data = await self._request("GET", f"/workflows/{workflow_id}/triggers")
        return [WorkflowTrigger(**t) for t in data["data"]]
    
    async def enable_trigger(self, trigger_id: str):
        """Enable a trigger"""
        await self._request("PUT", f"/triggers/{trigger_id}/enable")
    
    async def disable_trigger(self, trigger_id: str):
        """Disable a trigger"""
        await self._request("PUT", f"/triggers/{trigger_id}/disable")
    
    async def trigger_workflow(self, workflow_id: str) -> Dict[str, str]:
        """Manually trigger a workflow execution"""
        data = await self._request("POST", f"/workflows/{workflow_id}/trigger")
        return data["data"]
    
    async def schedule_workflow(
        self,
        workflow_id: str,
        cron_expression: str,
        timezone: Optional[str] = None,
        input_data: Optional[Dict[str, Any]] = None
    ) -> WorkflowTrigger:
        """Schedule a workflow with cron expression"""
        config = TriggerConfig(
            cron_expression=cron_expression,
            timezone=timezone,
            input_data=input_data
        )
        
        return await self.create_trigger(
            workflow_id,
            TriggerType.SCHEDULE,
            config
        )
    
    # Metrics
    
    async def get_workflow_metrics(self, workflow_id: str) -> Dict[str, Any]:
        """Get metrics for a specific workflow"""
        data = await self._request("GET", f"/workflows/{workflow_id}/metrics")
        return data["data"]
    
    async def get_workflow_history(
        self,
        duration: str = "1h"
    ) -> List[Dict[str, Any]]:
        """Get historical workflow metrics
        
        Args:
            duration: Duration string (e.g., "1h", "24h", "7d")
        """
        params = {"duration": duration}
        data = await self._request("GET", "/workflows/metrics/history", params=params)
        return data["data"]["workflows"]
    
    async def get_aggregate_metrics(
        self,
        duration: str = "1h"
    ) -> Dict[str, Any]:
        """Get aggregate workflow metrics
        
        Args:
            duration: Duration string (e.g., "1h", "24h", "7d")
        """
        params = {"duration": duration}
        data = await self._request("GET", "/workflows/metrics/aggregate", params=params)
        return data["data"]