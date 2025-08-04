import pytest
from agentainer_flow import workflow, step, parallel, mapreduce
from agentainer_flow.decorators import WorkflowMeta, StepMeta


class TestWorkflowDecorator:
    """Test @workflow decorator"""
    
    def test_basic_workflow_decorator(self):
        """Test basic workflow decoration"""
        @workflow("test-workflow")
        class TestWorkflow:
            pass
        
        assert hasattr(TestWorkflow, "_workflow_meta")
        assert isinstance(TestWorkflow._workflow_meta, WorkflowMeta)
        assert TestWorkflow._workflow_meta.name == "test-workflow"
    
    def test_workflow_with_config(self):
        """Test workflow with configuration"""
        @workflow(
            "test-workflow",
            max_parallel=10,
            timeout="30m",
            failure_strategy="continue_on_partial"
        )
        class TestWorkflow:
            pass
        
        meta = TestWorkflow._workflow_meta
        assert meta.config.max_parallel == 10
        assert meta.config.timeout == "30m"
        assert meta.config.failure_strategy == "continue_on_partial"
    
    def test_workflow_with_description(self):
        """Test workflow with description"""
        @workflow("test-workflow", description="Test description")
        class TestWorkflow:
            """Class docstring"""
            pass
        
        assert TestWorkflow._workflow_description == "Test description"
    
    def test_workflow_inherits_docstring(self):
        """Test workflow inherits class docstring as description"""
        @workflow("test-workflow")
        class TestWorkflow:
            """This is a test workflow"""
            pass
        
        assert TestWorkflow._workflow_description == "This is a test workflow"


class TestStepDecorator:
    """Test @step decorator"""
    
    def test_basic_step_decorator(self):
        """Test basic step decoration"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def process_data(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        assert "process_data" in meta.steps
        assert isinstance(meta.steps["process_data"], StepMeta)
        assert meta.steps["process_data"].name == "process_data"
    
    def test_step_with_custom_name(self):
        """Test step with custom name"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step(name="custom-step")
            async def process_data(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        assert "process_data" in meta.steps
        assert meta.steps["process_data"].name == "custom-step"
    
    def test_parallel_step(self):
        """Test parallel step"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step(parallel=True, max_workers=5)
            async def process_batch(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        step_meta = meta.steps["process_batch"]
        assert step_meta.config.parallel is True
        assert step_meta.config.max_workers == 5
    
    def test_step_with_dependencies(self):
        """Test step with dependencies"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def step1(self, ctx):
                pass
            
            @step(depends_on=["step1"])
            async def step2(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        assert meta.steps["step2"].depends_on == ["step1"]
    
    def test_step_with_pooling(self):
        """Test step with agent pooling"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step(
                pooled=True,
                pool_size=10,
                max_agent_uses=50
            )
            async def pooled_step(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        step_meta = meta.steps["pooled_step"]
        assert step_meta.config.execution_mode == "pooled"
        assert step_meta.config.pool_config.max_size == 10
        assert step_meta.config.pool_config.max_agent_uses == 50


class TestParallelDecorator:
    """Test @parallel decorator"""
    
    def test_parallel_decorator(self):
        """Test @parallel shorthand decorator"""
        @workflow("test-workflow")
        class TestWorkflow:
            @parallel(max_workers=10)
            async def parallel_task(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        step_meta = meta.steps["parallel_task"]
        assert step_meta.config.parallel is True
        assert step_meta.config.max_workers == 10
    
    def test_parallel_with_dynamic(self):
        """Test parallel with dynamic scaling"""
        @workflow("test-workflow")
        class TestWorkflow:
            @parallel(max_workers=10, dynamic=True)
            async def dynamic_parallel(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        step_meta = meta.steps["dynamic_parallel"]
        assert step_meta.config.dynamic is True


class TestMapReduceDecorator:
    """Test @mapreduce decorator"""
    
    def test_mapreduce_decorator(self):
        """Test @mapreduce decorator"""
        @workflow("test-workflow")
        class TestWorkflow:
            @mapreduce(
                mapper_image="mapper:latest",
                reducer_image="reducer:latest",
                max_parallel=20
            )
            async def process_dataset(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        
        # Should create two steps: mapper and reducer
        assert "process_dataset_map" in meta.steps
        assert "process_dataset_reduce" in meta.steps
        
        # Check mapper configuration
        mapper = meta.steps["process_dataset_map"]
        assert mapper.config.image == "mapper:latest"
        assert mapper.config.parallel is True
        assert mapper.config.max_workers == 20
        
        # Check reducer configuration
        reducer = meta.steps["process_dataset_reduce"]
        assert reducer.config.image == "reducer:latest"
        assert reducer.config.reduce is True
        assert reducer.depends_on == ["process_dataset_map"]
    
    def test_mapreduce_with_pool(self):
        """Test MapReduce with pooling"""
        @workflow("test-workflow")
        class TestWorkflow:
            @mapreduce(
                mapper_image="mapper:latest",
                reducer_image="reducer:latest",
                pool_size=15
            )
            async def pooled_mapreduce(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        mapper = meta.steps["pooled_mapreduce_map"]
        assert mapper.config.execution_mode == "pooled"
        assert mapper.config.pool_config.max_size == 15


class TestStepExecution:
    """Test step execution order and validation"""
    
    def test_step_registration_order(self):
        """Test steps are registered in definition order"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def step1(self, ctx):
                pass
            
            @step()
            async def step2(self, ctx):
                pass
            
            @step()
            async def step3(self, ctx):
                pass
        
        meta = TestWorkflow._workflow_meta
        step_names = list(meta.steps.keys())
        assert step_names == ["step1", "step2", "step3"]
    
    def test_invalid_decorator_usage(self):
        """Test invalid decorator usage"""
        with pytest.raises(ValueError, match="@step can only be used"):
            @step()
            async def standalone_step(ctx):
                pass
    
    def test_method_wrapping(self):
        """Test that decorated methods remain callable"""
        @workflow("test-workflow")
        class TestWorkflow:
            @step()
            async def process(self, ctx):
                return "processed"
        
        # Method should still be callable
        workflow_instance = TestWorkflow()
        assert asyncio.iscoroutinefunction(workflow_instance.process)


if __name__ == "__main__":
    pytest.main([__file__])
