package workflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
)

// CompensationAction defines a compensation action for a step
type CompensationAction struct {
	ID          string                 `json:"id"`
	StepID      string                 `json:"step_id"`      // Step this compensates for
	Type        CompensationType       `json:"type"`
	Config      CompensationConfig     `json:"config"`
	Status      CompensationStatus     `json:"status"`
	ExecutedAt  *time.Time             `json:"executed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// CompensationType defines the type of compensation
type CompensationType string

const (
	CompensationTypeRollback   CompensationType = "rollback"    // Rollback changes
	CompensationTypeRetry      CompensationType = "retry"       // Retry the operation
	CompensationTypeAlternate  CompensationType = "alternate"   // Try alternate path
	CompensationTypeNotify     CompensationType = "notify"      // Notify and continue
	CompensationTypeCustom     CompensationType = "custom"      // Custom compensation
)

// CompensationStatus tracks compensation execution status
type CompensationStatus string

const (
	CompensationStatusPending   CompensationStatus = "pending"
	CompensationStatusExecuting CompensationStatus = "executing"
	CompensationStatusCompleted CompensationStatus = "completed"
	CompensationStatusFailed    CompensationStatus = "failed"
	CompensationStatusSkipped   CompensationStatus = "skipped"
)

// CompensationConfig holds compensation configuration
type CompensationConfig struct {
	// Rollback configuration
	RollbackImage    string                 `json:"rollback_image,omitempty"`
	RollbackCommand  []string               `json:"rollback_command,omitempty"`
	RollbackEnvVars  map[string]string      `json:"rollback_env_vars,omitempty"`
	
	// Retry configuration
	MaxRetries       int                    `json:"max_retries,omitempty"`
	RetryDelay       string                 `json:"retry_delay,omitempty"`
	RetryBackoff     string                 `json:"retry_backoff,omitempty"`
	
	// Alternate path
	AlternateStepID  string                 `json:"alternate_step_id,omitempty"`
	AlternateWorkflow string                `json:"alternate_workflow,omitempty"`
	
	// Notification
	NotifyEndpoint   string                 `json:"notify_endpoint,omitempty"`
	NotifyData       map[string]interface{} `json:"notify_data,omitempty"`
	
	// Custom handler
	CustomHandler    string                 `json:"custom_handler,omitempty"`
	CustomConfig     map[string]interface{} `json:"custom_config,omitempty"`
}

// ErrorHandler manages workflow error handling and compensation
type ErrorHandler struct {
	workflowManager  *Manager
	agentManager     *agent.Manager
	stateManager     *StateManager
	orchestrator     *Orchestrator
	compensations    map[string][]CompensationAction // workflow ID -> compensations
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(workflowManager *Manager, agentManager *agent.Manager, stateManager *StateManager) *ErrorHandler {
	return &ErrorHandler{
		workflowManager: workflowManager,
		agentManager:    agentManager,
		stateManager:    stateManager,
		compensations:   make(map[string][]CompensationAction),
	}
}

// SetOrchestrator sets the orchestrator reference
func (eh *ErrorHandler) SetOrchestrator(orchestrator *Orchestrator) {
	eh.orchestrator = orchestrator
}

// HandleStepError handles an error in a workflow step
func (eh *ErrorHandler) HandleStepError(ctx context.Context, workflow *Workflow, step *WorkflowStep, err error) error {
	log.Printf("Handling error for step %s: %v", step.ID, err)
	
	// Record error in step
	step.Error = err.Error()
	step.Status = StepStatusFailed
	now := time.Now()
	step.CompletedAt = &now
	
	// Check if step has compensation configured
	if step.Config.RetryPolicy != nil && step.Config.RetryPolicy.MaxAttempts > 0 {
		// Try retry compensation first
		if retryErr := eh.executeRetryCompensation(ctx, workflow, step); retryErr == nil {
			return nil // Retry succeeded
		}
	}
	
	// Check workflow failure strategy
	switch workflow.Config.FailureStrategy {
	case "fail_fast":
		// Stop workflow and initiate rollback
		return eh.initiateRollback(ctx, workflow)
		
	case "continue_on_partial":
		// Mark step as failed but continue workflow
		log.Printf("Step %s failed, continuing with partial success", step.ID)
		return nil
		
	case "compensate":
		// Execute compensation for this step
		return eh.executeStepCompensation(ctx, workflow, step)
		
	default:
		return fmt.Errorf("unknown failure strategy: %s", workflow.Config.FailureStrategy)
	}
}

// executeRetryCompensation attempts to retry a failed step
func (eh *ErrorHandler) executeRetryCompensation(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	retryPolicy := step.Config.RetryPolicy
	if retryPolicy == nil {
		return fmt.Errorf("no retry policy configured")
	}
	
	// Calculate retry attempts
	retryCount := 0
	if countStr, exists := step.Metadata["retry_count"]; exists {
		fmt.Sscanf(countStr, "%d", &retryCount)
	}
	
	if retryCount >= retryPolicy.MaxAttempts {
		return fmt.Errorf("max retries (%d) exceeded", retryPolicy.MaxAttempts)
	}
	
	// Wait based on backoff strategy
	delay := calculateBackoffDelay(retryCount, retryPolicy)
	log.Printf("Retrying step %s after %v (attempt %d/%d)", step.ID, delay, retryCount+1, retryPolicy.MaxAttempts)
	time.Sleep(delay)
	
	// Update retry count
	if step.Metadata == nil {
		step.Metadata = make(map[string]string)
	}
	step.Metadata["retry_count"] = fmt.Sprintf("%d", retryCount+1)
	
	// Reset step status
	step.Status = StepStatusPending
	step.Error = ""
	step.StartedAt = nil
	step.CompletedAt = nil
	
	// Re-execute step
	if eh.orchestrator != nil {
		return eh.orchestrator.executeStep(ctx, workflow, step)
	}
	
	return fmt.Errorf("orchestrator not set")
}

// initiateRollback starts the rollback process for a workflow
func (eh *ErrorHandler) initiateRollback(ctx context.Context, workflow *Workflow) error {
	log.Printf("Initiating rollback for workflow %s", workflow.ID)
	
	// Get completed steps in reverse order
	completedSteps := []*WorkflowStep{}
	for i := range workflow.Steps {
		step := &workflow.Steps[i]
		if step.Status == StepStatusCompleted {
			completedSteps = append([]*WorkflowStep{step}, completedSteps...)
		}
	}
	
	// Execute rollback for each completed step
	for _, step := range completedSteps {
		if err := eh.rollbackStep(ctx, workflow, step); err != nil {
			log.Printf("Failed to rollback step %s: %v", step.ID, err)
			// Continue with other rollbacks
		}
	}
	
	// Update workflow status
	workflow.Status = WorkflowStatusFailed
	workflow.Metadata["rollback_completed"] = "true"
	
	return nil
}

// rollbackStep executes rollback for a specific step
func (eh *ErrorHandler) rollbackStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	log.Printf("Rolling back step %s", step.ID)
	
	// Create compensation action
	compensation := CompensationAction{
		ID:     fmt.Sprintf("comp-%s-%d", step.ID, time.Now().UnixNano()),
		StepID: step.ID,
		Type:   CompensationTypeRollback,
		Status: CompensationStatusPending,
	}
	
	// Check if step has rollback configuration
	if step.Config.Condition != nil && step.Config.Condition.Metadata != nil {
		if rollbackImage, exists := step.Config.Condition.Metadata["rollback_image"]; exists {
			compensation.Config.RollbackImage = rollbackImage
		}
	}
	
	// If no specific rollback config, use generic rollback
	if compensation.Config.RollbackImage == "" {
		compensation.Config.RollbackImage = "busybox:latest"
		compensation.Config.RollbackCommand = []string{"echo", "Rollback for step " + step.ID}
	}
	
	// Execute rollback
	return eh.executeCompensation(ctx, workflow, &compensation)
}

// executeStepCompensation executes compensation for a failed step
func (eh *ErrorHandler) executeStepCompensation(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Check if step has compensation actions defined
	compensations := eh.getStepCompensations(workflow.ID, step.ID)
	
	if len(compensations) == 0 {
		// No compensation defined, use default
		compensation := CompensationAction{
			ID:     fmt.Sprintf("comp-%s-%d", step.ID, time.Now().UnixNano()),
			StepID: step.ID,
			Type:   CompensationTypeNotify,
			Status: CompensationStatusPending,
			Config: CompensationConfig{
				NotifyData: map[string]interface{}{
					"workflow_id": workflow.ID,
					"step_id":     step.ID,
					"error":       step.Error,
				},
			},
		}
		compensations = []CompensationAction{compensation}
	}
	
	// Execute each compensation
	for _, comp := range compensations {
		if err := eh.executeCompensation(ctx, workflow, &comp); err != nil {
			log.Printf("Failed to execute compensation %s: %v", comp.ID, err)
		}
	}
	
	return nil
}

// executeCompensation executes a single compensation action
func (eh *ErrorHandler) executeCompensation(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	comp.Status = CompensationStatusExecuting
	now := time.Now()
	comp.ExecutedAt = &now
	
	var err error
	switch comp.Type {
	case CompensationTypeRollback:
		err = eh.executeRollbackAction(ctx, workflow, comp)
	case CompensationTypeRetry:
		err = eh.executeRetryAction(ctx, workflow, comp)
	case CompensationTypeAlternate:
		err = eh.executeAlternateAction(ctx, workflow, comp)
	case CompensationTypeNotify:
		err = eh.executeNotifyAction(ctx, workflow, comp)
	case CompensationTypeCustom:
		err = eh.executeCustomAction(ctx, workflow, comp)
	default:
		err = fmt.Errorf("unknown compensation type: %s", comp.Type)
	}
	
	if err != nil {
		comp.Status = CompensationStatusFailed
		comp.Error = err.Error()
	} else {
		comp.Status = CompensationStatusCompleted
	}
	
	// Store compensation result
	eh.storeCompensation(workflow.ID, comp)
	
	return err
}

// executeRollbackAction executes a rollback compensation
func (eh *ErrorHandler) executeRollbackAction(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	// Deploy rollback agent
	agent, err := eh.agentManager.Deploy(
		ctx,
		fmt.Sprintf("rollback-%s", comp.ID),
		comp.Config.RollbackImage,
		comp.Config.RollbackEnvVars,
		0, // default CPU
		0, // default memory
		false,
		"",
		nil,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy rollback agent: %w", err)
	}
	
	// Start agent
	if err := eh.agentManager.Start(ctx, agent.ID); err != nil {
		return fmt.Errorf("failed to start rollback agent: %w", err)
	}
	
	// Wait for completion (simplified)
	time.Sleep(5 * time.Second)
	
	return nil
}

// executeRetryAction executes a retry compensation
func (eh *ErrorHandler) executeRetryAction(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	// Find the step to retry
	var stepToRetry *WorkflowStep
	for i := range workflow.Steps {
		if workflow.Steps[i].ID == comp.StepID {
			stepToRetry = &workflow.Steps[i]
			break
		}
	}
	
	if stepToRetry == nil {
		return fmt.Errorf("step %s not found", comp.StepID)
	}
	
	// Re-execute the step
	if eh.orchestrator != nil {
		return eh.orchestrator.executeStep(ctx, workflow, stepToRetry)
	}
	
	return fmt.Errorf("orchestrator not set")
}

// executeAlternateAction executes an alternate path compensation
func (eh *ErrorHandler) executeAlternateAction(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	if comp.Config.AlternateWorkflow != "" {
		// Execute alternate workflow
		if eh.orchestrator != nil && eh.orchestrator.subWorkflowExecutor != nil {
			_, err := eh.orchestrator.subWorkflowExecutor.ExecuteSubWorkflow(
				ctx,
				workflow.ID,
				comp.StepID,
				comp.Config.AlternateWorkflow,
				workflow.State,
			)
			return err
		}
	}
	
	return fmt.Errorf("no alternate path configured")
}

// executeNotifyAction executes a notification compensation
func (eh *ErrorHandler) executeNotifyAction(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	// Log notification (in production, would send to external system)
	log.Printf("Compensation notification for workflow %s, step %s: %v",
		workflow.ID, comp.StepID, comp.Config.NotifyData)
	
	// Store notification in workflow state
	notificationKey := fmt.Sprintf("compensation_notification_%s", comp.ID)
	if err := eh.stateManager.SetValue(ctx, workflow.ID, notificationKey, comp.Config.NotifyData); err != nil {
		return fmt.Errorf("failed to store notification: %w", err)
	}
	
	return nil
}

// executeCustomAction executes a custom compensation handler
func (eh *ErrorHandler) executeCustomAction(ctx context.Context, workflow *Workflow, comp *CompensationAction) error {
	// In a real implementation, this would call a custom handler
	log.Printf("Executing custom compensation handler: %s", comp.Config.CustomHandler)
	return nil
}

// getStepCompensations retrieves compensation actions for a step
func (eh *ErrorHandler) getStepCompensations(workflowID, stepID string) []CompensationAction {
	if compensations, exists := eh.compensations[workflowID]; exists {
		var stepComps []CompensationAction
		for _, comp := range compensations {
			if comp.StepID == stepID {
				stepComps = append(stepComps, comp)
			}
		}
		return stepComps
	}
	return nil
}

// storeCompensation stores a compensation action
func (eh *ErrorHandler) storeCompensation(workflowID string, comp *CompensationAction) {
	if _, exists := eh.compensations[workflowID]; !exists {
		eh.compensations[workflowID] = []CompensationAction{}
	}
	eh.compensations[workflowID] = append(eh.compensations[workflowID], *comp)
}

// calculateBackoffDelay calculates retry delay based on backoff strategy
func calculateBackoffDelay(retryCount int, policy *RetryPolicy) time.Duration {
	baseDelay, _ := time.ParseDuration(policy.Delay)
	if baseDelay == 0 {
		baseDelay = time.Second
	}
	
	switch policy.Backoff {
	case "exponential":
		return baseDelay * time.Duration(1<<uint(retryCount))
	case "linear":
		return baseDelay * time.Duration(retryCount+1)
	default:
		return baseDelay
	}
}

// CompensationPlan defines a complete compensation plan for a workflow
type CompensationPlan struct {
	WorkflowID    string                   `json:"workflow_id"`
	Actions       []CompensationAction     `json:"actions"`
	Strategy      CompensationStrategy     `json:"strategy"`
	CreatedAt     time.Time                `json:"created_at"`
}

// CompensationStrategy defines how compensations are executed
type CompensationStrategy string

const (
	CompensationStrategySequential CompensationStrategy = "sequential"  // Execute in order
	CompensationStrategyParallel   CompensationStrategy = "parallel"    // Execute in parallel
	CompensationStrategyBestEffort CompensationStrategy = "best_effort" // Continue on failure
)
