"""
Decorators for defining workflows and steps
"""

import functools
import inspect
from typing import Any, Callable, Dict, List, Optional, Type, Union
from .models import (
    StepConfig, 
    StepType, 
    WorkflowConfig,
    RetryPolicy,
    ResourceLimits,
    PoolConfig,
)
from .exceptions import ValidationError


class WorkflowMeta:
    """Metadata for workflow class"""
    def __init__(self, name: str, config: WorkflowConfig):
        self.name = name
        self.config = config
        self.steps: Dict[str, StepMeta] = {}


class StepMeta:
    """Metadata for workflow step"""
    def __init__(
        self,
        name: str,
        func: Callable,
        step_type: StepType,
        config: StepConfig,
        depends_on: Optional[List[str]] = None,
    ):
        self.name = name
        self.func = func
        self.step_type = step_type
        self.config = config
        self.depends_on = depends_on or []


def workflow(
    name: str,
    description: Optional[str] = None,
    max_parallel: Optional[int] = None,
    timeout: Optional[str] = None,
    retry_policy: Optional[Dict[str, Any]] = None,
    failure_strategy: Optional[str] = "fail_fast",
) -> Callable[[Type], Type]:
    """
    Decorator to define a workflow class.
    
    Args:
        name: Workflow name
        description: Workflow description
        max_parallel: Maximum parallel executions
        timeout: Workflow timeout (e.g., "2h", "30m")
        retry_policy: Retry configuration
        failure_strategy: How to handle failures ("fail_fast" or "continue")
    
    Example:
        @workflow(name="data_pipeline", timeout="2h")
        class DataPipeline:
            pass
    """
    def decorator(cls: Type) -> Type:
        # Create workflow configuration
        config = WorkflowConfig(
            max_parallel=max_parallel,
            timeout=timeout,
            retry_policy=RetryPolicy(**retry_policy) if retry_policy else None,
            failure_strategy=failure_strategy,
        )
        
        # Create workflow metadata
        meta = WorkflowMeta(name=name, config=config)
        
        # Attach metadata to class
        cls._workflow_meta = meta
        cls._workflow_name = name
        cls._workflow_description = description
        
        # Collect step methods
        for attr_name in dir(cls):
            attr = getattr(cls, attr_name)
            if hasattr(attr, "_step_meta"):
                step_meta = attr._step_meta
                meta.steps[step_meta.name] = step_meta
        
        return cls
    
    return decorator


def step(
    name: Optional[str] = None,
    image: Optional[str] = None,
    parallel: bool = False,
    max_workers: Optional[int] = None,
    dynamic: bool = False,
    reduce: bool = False,
    depends_on: Optional[List[str]] = None,
    timeout: Optional[str] = None,
    retry_policy: Optional[Dict[str, Any]] = None,
    resource_limits: Optional[Dict[str, Any]] = None,
    execution_mode: str = "standard",
    pool_config: Optional[Dict[str, Any]] = None,
    env_vars: Optional[Dict[str, str]] = None,
) -> Callable[[Callable], Callable]:
    """
    Decorator to define a workflow step.
    
    Args:
        name: Step name (defaults to function name)
        image: Docker image for the step
        parallel: Enable parallel execution
        max_workers: Maximum parallel workers
        dynamic: Dynamic parallelism based on input
        reduce: Mark as reduce step (aggregates parallel results)
        depends_on: List of step names this depends on
        timeout: Step timeout
        retry_policy: Retry configuration
        resource_limits: CPU/memory limits
        execution_mode: "standard" or "pooled"
        pool_config: Agent pool configuration
        env_vars: Environment variables
    
    Example:
        @step(name="process", parallel=True, max_workers=5)
        async def process_data(self, ctx, item):
            pass
    """
    def decorator(func: Callable) -> Callable:
        # Determine step name
        step_name = name or func.__name__
        
        # Determine step type
        if reduce:
            step_type = StepType.REDUCE
        elif parallel:
            step_type = StepType.PARALLEL
        else:
            step_type = StepType.SEQUENTIAL
        
        # Create step configuration
        config = StepConfig(
            image=image or "default",  # Will be overridden if needed
            env_vars=env_vars,
            parallel=parallel,
            max_workers=max_workers,
            dynamic=dynamic,
            reduce=reduce,
            timeout=timeout,
            retry_policy=RetryPolicy(**retry_policy) if retry_policy else None,
            resource_limits=ResourceLimits(**resource_limits) if resource_limits else None,
            execution_mode=execution_mode,
            pool_config=PoolConfig(**pool_config) if pool_config else None,
        )
        
        # Create step metadata
        step_meta = StepMeta(
            name=step_name,
            func=func,
            step_type=step_type,
            config=config,
            depends_on=depends_on,
        )
        
        # Attach metadata to function
        func._step_meta = step_meta
        
        @functools.wraps(func)
        async def wrapper(self, *args, **kwargs):
            return await func(self, *args, **kwargs)
        
        wrapper._step_meta = step_meta
        
        return wrapper
    
    return decorator


def parallel(
    max_workers: int = 10,
    dynamic: bool = False,
    execution_mode: str = "pooled",
    pool_size: Optional[int] = None,
) -> Callable[[Callable], Callable]:
    """
    Shorthand decorator for parallel steps.
    
    Args:
        max_workers: Maximum parallel workers
        dynamic: Dynamic parallelism
        execution_mode: "standard" or "pooled"
        pool_size: Agent pool size (for pooled mode)
    
    Example:
        @parallel(max_workers=20)
        async def process_item(self, ctx, item):
            pass
    """
    pool_config = None
    if execution_mode == "pooled" and pool_size:
        pool_config = {"max_size": pool_size}
    
    return step(
        parallel=True,
        max_workers=max_workers,
        dynamic=dynamic,
        execution_mode=execution_mode,
        pool_config=pool_config,
    )


def mapreduce(
    mapper: str,
    reducer: str,
    max_parallel: int = 10,
    pool_size: Optional[int] = None,
    timeout: Optional[str] = "30m",
) -> Callable[[Type], Type]:
    """
    Decorator for simplified MapReduce pattern.
    
    Args:
        mapper: Mapper Docker image
        reducer: Reducer Docker image
        max_parallel: Maximum parallel mappers
        pool_size: Agent pool size
        timeout: Workflow timeout
    
    Example:
        @mapreduce(
            mapper="word-mapper:latest",
            reducer="word-reducer:latest",
            max_parallel=20
        )
        class WordCount:
            async def configure(self, ctx):
                return {"input": "data.txt"}
    """
    def decorator(cls: Type) -> Type:
        # Add MapReduce metadata
        cls._mapreduce_config = {
            "mapper_image": mapper,
            "reducer_image": reducer,
            "max_parallel": max_parallel,
            "pool_size": pool_size,
            "timeout": timeout,
        }
        
        # Apply workflow decorator
        workflow_name = cls.__name__.lower()
        cls = workflow(
            name=workflow_name,
            timeout=timeout,
            max_parallel=max_parallel,
        )(cls)
        
        return cls
    
    return decorator