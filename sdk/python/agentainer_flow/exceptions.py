"""
Exception classes for Agentainer Flow SDK
"""


class AgentainerError(Exception):
    """Base exception for all Agentainer errors"""
    pass


class WorkflowError(AgentainerError):
    """Workflow-related errors"""
    pass


class StepError(AgentainerError):
    """Step execution errors"""
    pass


class StateError(AgentainerError):
    """State management errors"""
    pass


class ClientError(AgentainerError):
    """Client communication errors"""
    pass


class ValidationError(AgentainerError):
    """Validation errors"""
    pass


class TimeoutError(AgentainerError):
    """Timeout errors"""
    pass


class PoolError(AgentainerError):
    """Agent pool errors"""
    pass