package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// SubWorkflowExecutor handles sub-workflow execution
type SubWorkflowExecutor struct {
	workflowManager *Manager
	orchestrator    *Orchestrator
	stateManager    *StateManager
}

// NewSubWorkflowExecutor creates a new sub-workflow executor
func NewSubWorkflowExecutor(workflowManager *Manager, orchestrator *Orchestrator, stateManager *StateManager) *SubWorkflowExecutor {
	return &SubWorkflowExecutor{
		workflowManager: workflowManager,
		orchestrator:    orchestrator,
		stateManager:    stateManager,
	}
}

// ExecuteSubWorkflow executes a sub-workflow within a parent workflow
func (se *SubWorkflowExecutor) ExecuteSubWorkflow(ctx context.Context, parentWorkflowID, parentStepID, subWorkflowID string, inputState map[string]interface{}) (*Workflow, error) {
	// Get sub-workflow definition
	subWorkflow, err := se.workflowManager.GetWorkflow(ctx, subWorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-workflow: %w", err)
	}

	// Create new instance of sub-workflow
	instanceID := fmt.Sprintf("%s-sub-%d", parentWorkflowID, time.Now().UnixNano())
	newWorkflow := &Workflow{
		ID:          instanceID,
		Name:        fmt.Sprintf("%s (sub)", subWorkflow.Name),
		Description: subWorkflow.Description,
		Status:      WorkflowStatusPending,
		Config:      subWorkflow.Config,
		Steps:       subWorkflow.Steps,
		State:       make(map[string]interface{}),
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Set parent workflow metadata
	newWorkflow.Metadata["parent_workflow_id"] = parentWorkflowID
	newWorkflow.Metadata["parent_step_id"] = parentStepID
	newWorkflow.Metadata["is_subworkflow"] = "true"

	// Copy parent metadata with prefix
	if subWorkflow.Metadata != nil {
		for k, v := range subWorkflow.Metadata {
			newWorkflow.Metadata[k] = v
		}
	}

	// Initialize state with input
	if inputState != nil {
		for k, v := range inputState {
			newWorkflow.State[k] = v
		}
	}

	// Save the new workflow instance
	if err := se.workflowManager.SaveWorkflow(ctx, newWorkflow); err != nil {
		return nil, fmt.Errorf("failed to save sub-workflow instance: %w", err)
	}

	// Execute the sub-workflow
	if err := se.orchestrator.ExecuteWorkflow(ctx, newWorkflow.ID); err != nil {
		return nil, fmt.Errorf("failed to execute sub-workflow: %w", err)
	}

	// Get the final state
	finalWorkflow, err := se.workflowManager.GetWorkflow(ctx, newWorkflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get final sub-workflow state: %w", err)
	}

	return finalWorkflow, nil
}

// CreateWorkflowTemplate creates a reusable workflow template
func (se *SubWorkflowExecutor) CreateWorkflowTemplate(ctx context.Context, template *WorkflowTemplate) error {
	// Validate template
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if len(template.Steps) == 0 {
		return fmt.Errorf("template must have at least one step")
	}

	// Create workflow from template
	workflow := &Workflow{
		ID:          fmt.Sprintf("template-%s", template.Name),
		Name:        template.Name,
		Description: template.Description,
		Status:      WorkflowStatusPending,
		Config:      template.Config,
		Steps:       template.Steps,
		State:       make(map[string]interface{}),
		Metadata: map[string]string{
			"is_template": "true",
			"category":    template.Category,
			"version":     template.Version,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add input schema to metadata
	if template.InputSchema != nil {
		workflow.Metadata["input_schema"] = string(mustMarshalJSON(template.InputSchema))
	}

	// Add output schema to metadata  
	if template.OutputSchema != nil {
		workflow.Metadata["output_schema"] = string(mustMarshalJSON(template.OutputSchema))
	}

	// Save as template
	return se.workflowManager.SaveWorkflow(ctx, workflow)
}

// InstantiateTemplate creates a new workflow instance from a template
func (se *SubWorkflowExecutor) InstantiateTemplate(ctx context.Context, templateName string, instanceName string, inputData map[string]interface{}) (*Workflow, error) {
	// Get template
	templateID := fmt.Sprintf("template-%s", templateName)
	template, err := se.workflowManager.GetWorkflow(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Verify it's a template
	if template.Metadata["is_template"] != "true" {
		return nil, fmt.Errorf("workflow %s is not a template", templateName)
	}

	// Validate input against schema if available
	if schemaStr, ok := template.Metadata["input_schema"]; ok && schemaStr != "" {
		// TODO: Implement JSON schema validation
		log.Printf("Input validation against schema not yet implemented")
	}

	// Create new instance
	instanceID := fmt.Sprintf("%s-%d", instanceName, time.Now().UnixNano())
	instance := &Workflow{
		ID:          instanceID,
		Name:        instanceName,
		Description: fmt.Sprintf("Instance of %s", template.Description),
		Status:      WorkflowStatusPending,
		Config:      template.Config,
		Steps:       make([]WorkflowStep, len(template.Steps)),
		State:       inputData,
		Metadata: map[string]string{
			"template_name": templateName,
			"template_id":   templateID,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Deep copy steps
	copy(instance.Steps, template.Steps)

	// Save instance
	if err := se.workflowManager.SaveWorkflow(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to save workflow instance: %w", err)
	}

	return instance, nil
}

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Category     string                 `json:"category"`      // e.g., "data-processing", "ml-pipeline"
	Version      string                 `json:"version"`       // Semantic versioning
	Config       WorkflowConfig         `json:"config"`
	Steps        []WorkflowStep         `json:"steps"`
	InputSchema  map[string]interface{} `json:"input_schema,omitempty"`  // JSON schema for input validation
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"` // JSON schema for output
	Examples     []WorkflowExample      `json:"examples,omitempty"`      // Usage examples
	Tags         []string               `json:"tags,omitempty"`          // Searchable tags
}

// WorkflowExample shows how to use a workflow template
type WorkflowExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputData   map[string]interface{} `json:"input_data"`
	ExpectedOutput map[string]interface{} `json:"expected_output,omitempty"`
}

// NestedExecution tracks nested workflow execution
type NestedExecution struct {
	ParentWorkflowID string    `json:"parent_workflow_id"`
	ParentStepID     string    `json:"parent_step_id"`
	SubWorkflowID    string    `json:"sub_workflow_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          *time.Time `json:"end_time,omitempty"`
	Status           WorkflowStatus `json:"status"`
	Depth            int        `json:"depth"` // Nesting depth
}

// GetWorkflowHierarchy returns the full hierarchy of a workflow execution
func (se *SubWorkflowExecutor) GetWorkflowHierarchy(ctx context.Context, workflowID string) (*WorkflowHierarchy, error) {
	hierarchy := &WorkflowHierarchy{
		WorkflowID: workflowID,
		Children:   make([]*WorkflowHierarchy, 0),
	}

	// Get workflow
	workflow, err := se.workflowManager.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	hierarchy.Name = workflow.Name
	hierarchy.Status = workflow.Status

	// Find all sub-workflows
	for _, step := range workflow.Steps {
		if step.Type == StepTypeSubWorkflow && step.Config.SubWorkflowID != "" {
			subHierarchy, err := se.GetWorkflowHierarchy(ctx, step.Config.SubWorkflowID)
			if err != nil {
				log.Printf("Failed to get sub-workflow hierarchy: %v", err)
				continue
			}
			hierarchy.Children = append(hierarchy.Children, subHierarchy)
		}
	}

	return hierarchy, nil
}

// WorkflowHierarchy represents the execution hierarchy
type WorkflowHierarchy struct {
	WorkflowID string               `json:"workflow_id"`
	Name       string               `json:"name"`
	Status     WorkflowStatus       `json:"status"`
	Children   []*WorkflowHierarchy `json:"children,omitempty"`
}

// mustMarshalJSON marshals to JSON or panics
func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
