package workflow

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MapReduceConfig defines configuration for MapReduce pattern
type MapReduceConfig struct {
	Name          string                 `json:"name"`
	MapperImage   string                 `json:"mapper_image"`
	ReducerImage  string                 `json:"reducer_image"`
	MaxParallel   int                    `json:"max_parallel"`
	PoolSize      int                    `json:"pool_size,omitempty"`
	Timeout       string                 `json:"timeout,omitempty"`
	ErrorStrategy string                 `json:"error_strategy,omitempty"` // "fail_fast" or "continue_on_partial"
	InputConfig   map[string]interface{} `json:"input_config,omitempty"`
}

// ExecuteMapReduce provides a simplified API for MapReduce workflows
func (o *Orchestrator) ExecuteMapReduce(ctx context.Context, config MapReduceConfig) (*Workflow, error) {
	// Create workflow
	workflowConfig := WorkflowConfig{
		MaxParallel:     config.MaxParallel,
		Timeout:         config.Timeout,
		FailureStrategy: config.ErrorStrategy,
	}
	
	if workflowConfig.FailureStrategy == "" {
		workflowConfig.FailureStrategy = "fail_fast"
	}

	workflow, err := o.workflowManager.CreateWorkflow(ctx, config.Name, "MapReduce workflow", workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Create list step
	listStep := WorkflowStep{
		ID:     uuid.New().String(),
		Name:   "list",
		Type:   StepTypeSequential,
		Status: StepStatusPending,
		Config: StepConfig{
			Image:   config.MapperImage,
			EnvVars: map[string]string{"STEP_TYPE": "list"},
		},
	}

	// Create map step
	poolConfig := &PoolConfig{
		MinSize:      2,
		MaxSize:      config.PoolSize,
		IdleTimeout:  "5m",
		MaxAgentUses: 100,
		WarmUp:       true,
	}
	
	if config.PoolSize == 0 {
		poolConfig.MaxSize = 5
	}

	mapStep := WorkflowStep{
		ID:        uuid.New().String(),
		Name:      "map",
		Type:      StepTypeParallel,
		Status:    StepStatusPending,
		DependsOn: []string{listStep.ID},
		Config: StepConfig{
			Image:         config.MapperImage,
			EnvVars:       map[string]string{"STEP_TYPE": "map"},
			Parallel:      true,
			MaxWorkers:    config.MaxParallel,
			Dynamic:       true,
			ExecutionMode: "pooled",
			PoolConfig:    poolConfig,
		},
	}

	// Create reduce step
	reduceStep := WorkflowStep{
		ID:        uuid.New().String(),
		Name:      "reduce",
		Type:      StepTypeReduce,
		Status:    StepStatusPending,
		DependsOn: []string{mapStep.ID},
		Config: StepConfig{
			Image:   config.ReducerImage,
			EnvVars: map[string]string{"STEP_TYPE": "reduce"},
			Reduce:  true,
		},
	}

	// Add steps to workflow
	workflow.Steps = []WorkflowStep{listStep, mapStep, reduceStep}
	
	// Save input config to state
	if config.InputConfig != nil {
		for k, v := range config.InputConfig {
			if err := o.workflowManager.UpdateWorkflowState(ctx, workflow.ID, k, v); err != nil {
				return nil, fmt.Errorf("failed to save input config: %w", err)
			}
		}
	}

	// Save workflow
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	// Start execution
	go o.ExecuteWorkflow(ctx, workflow.ID)

	return workflow, nil
}

// ScatterGatherConfig defines configuration for Scatter-Gather pattern
type ScatterGatherConfig struct {
	Name           string                 `json:"name"`
	ScatterImage   string                 `json:"scatter_image"`
	GatherImage    string                 `json:"gather_image"`
	Workers        int                    `json:"workers"`
	InputBatchSize int                    `json:"input_batch_size,omitempty"`
	Timeout        string                 `json:"timeout,omitempty"`
}

// ExecuteScatterGather provides a simplified API for Scatter-Gather workflows
func (o *Orchestrator) ExecuteScatterGather(ctx context.Context, config ScatterGatherConfig) (*Workflow, error) {
	// Create workflow
	workflowConfig := WorkflowConfig{
		MaxParallel:     config.Workers,
		Timeout:         config.Timeout,
		FailureStrategy: "continue",
	}

	workflow, err := o.workflowManager.CreateWorkflow(ctx, config.Name, "Scatter-Gather workflow", workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Create scatter steps
	var scatterSteps []WorkflowStep
	for i := 0; i < config.Workers; i++ {
		step := WorkflowStep{
			ID:     fmt.Sprintf("scatter-%d", i),
			Name:   fmt.Sprintf("scatter-%d", i),
			Type:   StepTypeSequential,
			Status: StepStatusPending,
			Config: StepConfig{
				Image: config.ScatterImage,
				EnvVars: map[string]string{
					"WORKER_ID":    fmt.Sprintf("%d", i),
					"TOTAL_WORKERS": fmt.Sprintf("%d", config.Workers),
				},
			},
		}
		scatterSteps = append(scatterSteps, step)
	}

	// Create gather step
	var dependsOn []string
	for _, step := range scatterSteps {
		dependsOn = append(dependsOn, step.ID)
	}

	gatherStep := WorkflowStep{
		ID:        "gather",
		Name:      "gather",
		Type:      StepTypeReduce,
		Status:    StepStatusPending,
		DependsOn: dependsOn,
		Config: StepConfig{
			Image:  config.GatherImage,
			Reduce: true,
		},
	}

	// Add all steps
	workflow.Steps = append(scatterSteps, gatherStep)

	// Save workflow
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	// Start execution
	go o.ExecuteWorkflow(ctx, workflow.ID)

	return workflow, nil
}

// PipelineConfig defines configuration for Pipeline pattern
type PipelineConfig struct {
	Name   string        `json:"name"`
	Stages []StageConfig `json:"stages"`
}

// StageConfig defines a pipeline stage
type StageConfig struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	EnvVars map[string]string `json:"env_vars,omitempty"`
	Timeout string            `json:"timeout,omitempty"`
}

// ExecutePipeline provides a simplified API for Pipeline workflows
func (o *Orchestrator) ExecutePipeline(ctx context.Context, config PipelineConfig) (*Workflow, error) {
	// Create workflow
	workflowConfig := WorkflowConfig{
		FailureStrategy: "fail_fast",
	}

	workflow, err := o.workflowManager.CreateWorkflow(ctx, config.Name, "Pipeline workflow", workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Create pipeline steps
	var previousStepID string
	for i, stage := range config.Stages {
		step := WorkflowStep{
			ID:     fmt.Sprintf("stage-%d-%s", i, stage.Name),
			Name:   stage.Name,
			Type:   StepTypeSequential,
			Status: StepStatusPending,
			Config: StepConfig{
				Image:   stage.Image,
				EnvVars: stage.EnvVars,
				Timeout: stage.Timeout,
			},
		}

		if previousStepID != "" {
			step.DependsOn = []string{previousStepID}
		}

		workflow.Steps = append(workflow.Steps, step)
		previousStepID = step.ID
	}

	// Save workflow
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	// Start execution
	go o.ExecuteWorkflow(ctx, workflow.ID)

	return workflow, nil
}

// Helper function to create a MapReduce workflow from code
func CreateMapReduceWorkflow(name, mapperImage, reducerImage string, parallelism int) MapReduceConfig {
	return MapReduceConfig{
		Name:         name,
		MapperImage:  mapperImage,
		ReducerImage: reducerImage,
		MaxParallel:  parallelism,
		PoolSize:     5,
		Timeout:      "30m",
	}
}