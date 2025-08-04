package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// WorkflowStatus represents the current state of a workflow
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusRunning    WorkflowStatus = "running"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusFailed     WorkflowStatus = "failed"
	WorkflowStatusCancelled  WorkflowStatus = "cancelled"
)

// StepStatus represents the current state of a workflow step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// Workflow represents an orchestrated workflow
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Status      WorkflowStatus         `json:"status"`
	Config      WorkflowConfig         `json:"config"`
	Steps       []WorkflowStep         `json:"steps"`
	State       map[string]interface{} `json:"state"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// WorkflowConfig holds workflow-level configuration
type WorkflowConfig struct {
	MaxParallel      int               `json:"max_parallel,omitempty"`
	Timeout          string            `json:"timeout,omitempty"`
	RetryPolicy      *RetryPolicy      `json:"retry_policy,omitempty"`
	FailureStrategy  string            `json:"failure_strategy,omitempty"` // "fail_fast" or "continue"
	ResourceLimits   *ResourceLimits   `json:"resource_limits,omitempty"`
	EnableProfiling  bool              `json:"enable_profiling,omitempty"`
	Schedule         string            `json:"schedule,omitempty"` // Cron schedule
	CleanupPolicy    string            `json:"cleanup_policy,omitempty"` // "always" (default), "on_success", "never"
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        StepType               `json:"type"`
	Status      StepStatus             `json:"status"`
	Config      StepConfig             `json:"config"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Results     interface{}            `json:"results,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// StepType defines the type of workflow step
type StepType string

const (
	StepTypeSequential StepType = "sequential"
	StepTypeParallel   StepType = "parallel"
	StepTypeMap        StepType = "map"         // Map over array/list
	StepTypeReduce     StepType = "reduce"
	StepTypeMapReduce  StepType = "mapreduce"
	StepTypeDecision   StepType = "decision"    // Decision node
	StepTypeBranch     StepType = "branch"      // Conditional branch
	StepTypeSubWorkflow StepType = "subworkflow" // Sub-workflow execution
)

// StepConfig holds step-specific configuration
type StepConfig struct {
	Image           string            `json:"image"`
	Command         []string          `json:"command,omitempty"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	Parallel        bool              `json:"parallel,omitempty"`
	MaxWorkers      int               `json:"max_workers,omitempty"`
	Dynamic         bool              `json:"dynamic,omitempty"`
	Reduce          bool              `json:"reduce,omitempty"`
	Timeout         string            `json:"timeout,omitempty"`
	RetryPolicy     *RetryPolicy      `json:"retry_policy,omitempty"`
	ResourceLimits  *ResourceLimits   `json:"resource_limits,omitempty"`
	
	// Pool configuration for performance optimization
	ExecutionMode   string            `json:"execution_mode,omitempty"` // "standard" or "pooled"
	PoolConfig      *PoolConfig       `json:"pool_config,omitempty"`
	
	// Conditional execution
	Condition       *Condition        `json:"condition,omitempty"`      // Execute only if condition is true
	BranchConfig    *BranchConfig     `json:"branch_config,omitempty"`  // For branch steps
	DecisionNode    *DecisionNode     `json:"decision_node,omitempty"`  // For decision steps
	
	// Sub-workflow
	SubWorkflowID   string            `json:"sub_workflow_id,omitempty"`  // ID of sub-workflow to execute
	SubWorkflowName string            `json:"sub_workflow_name,omitempty"` // Name of sub-workflow template
	
	// Map configuration
	MapConfig       *MapConfig        `json:"map_config,omitempty"`      // For map steps
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff"` // "exponential", "linear", "constant"
	Delay       string `json:"delay"`   // Duration string
}

// ResourceLimits defines resource constraints
type ResourceLimits struct {
	CPULimit    int64 `json:"cpu_limit,omitempty"`    // In nanoseconds
	MemoryLimit int64 `json:"memory_limit,omitempty"` // In bytes
	GPULimit    int   `json:"gpu_limit,omitempty"`    // Number of GPUs
}

// PoolConfig defines agent pool configuration
type PoolConfig struct {
	MinSize       int    `json:"min_size"`
	MaxSize       int    `json:"max_size"`
	IdleTimeout   string `json:"idle_timeout"`
	MaxAgentUses  int    `json:"max_agent_uses"`
	WarmUp        bool   `json:"warm_up"`
}

// MapConfig defines configuration for map steps
type MapConfig struct {
	InputPath      string `json:"input_path"`       // JSON path to array/list to iterate over (e.g., "$.urls")
	ItemAlias      string `json:"item_alias"`       // Variable name for current item (e.g., "url")
	MaxConcurrency int    `json:"max_concurrency"`  // Maximum parallel executions (0 = unlimited)
	ErrorHandling  string `json:"error_handling"`   // "fail_fast" or "continue_on_error"
}

// Manager handles workflow orchestration
type Manager struct {
	redisClient *redis.Client
}

// NewManager creates a new workflow manager
func NewManager(redisClient *redis.Client) *Manager {
	return &Manager{
		redisClient: redisClient,
	}
}

// CreateWorkflow creates a new workflow
func (m *Manager) CreateWorkflow(ctx context.Context, name, description string, config WorkflowConfig) (*Workflow, error) {
	workflow := &Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Status:      WorkflowStatusPending,
		Config:      config,
		Steps:       []WorkflowStep{},
		State:       make(map[string]interface{}),
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := m.SaveWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	return workflow, nil
}

// GetWorkflow retrieves a workflow by ID
func (m *Manager) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	key := fmt.Sprintf("workflow:%s", workflowID)
	data, err := m.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("workflow not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	var workflow Workflow
	if err := json.Unmarshal([]byte(data), &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	return &workflow, nil
}

// ListWorkflows lists all workflows with optional filtering
func (m *Manager) ListWorkflows(ctx context.Context, status WorkflowStatus) ([]Workflow, error) {
	// Get all workflow IDs
	workflowIDs, err := m.redisClient.SMembers(ctx, "workflows:list").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow list: %w", err)
	}

	workflows := make([]Workflow, 0, len(workflowIDs))
	for _, id := range workflowIDs {
		workflow, err := m.GetWorkflow(ctx, id)
		if err != nil {
			continue // Skip invalid workflows
		}

		// Filter by status if specified
		if status != "" && workflow.Status != status {
			continue
		}

		workflows = append(workflows, *workflow)
	}

	return workflows, nil
}

// GetWorkflowJobs returns all jobs (agents) for a workflow
func (m *Manager) GetWorkflowJobs(ctx context.Context, workflowID string) ([]string, error) {
	key := fmt.Sprintf("workflow:%s:jobs", workflowID)
	return m.redisClient.SMembers(ctx, key).Result()
}

// AddJobToWorkflow associates a job (agent) with a workflow
func (m *Manager) AddJobToWorkflow(ctx context.Context, workflowID, agentID string) error {
	key := fmt.Sprintf("workflow:%s:jobs", workflowID)
	return m.redisClient.SAdd(ctx, key, agentID).Err()
}

// UpdateWorkflowStatus updates the status of a workflow
func (m *Manager) UpdateWorkflowStatus(ctx context.Context, workflowID string, status WorkflowStatus) error {
	workflow, err := m.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	workflow.Status = status
	workflow.UpdatedAt = time.Now()

	if status == WorkflowStatusRunning && workflow.StartedAt == nil {
		now := time.Now()
		workflow.StartedAt = &now
	} else if (status == WorkflowStatusCompleted || status == WorkflowStatusFailed || status == WorkflowStatusCancelled) && workflow.CompletedAt == nil {
		now := time.Now()
		workflow.CompletedAt = &now
	}

	return m.SaveWorkflow(ctx, workflow)
}

// UpdateWorkflowState updates the shared state of a workflow
func (m *Manager) UpdateWorkflowState(ctx context.Context, workflowID string, key string, value interface{}) error {
	workflow, err := m.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	if workflow.State == nil {
		workflow.State = make(map[string]interface{})
	}

	workflow.State[key] = value
	workflow.UpdatedAt = time.Now()

	return m.SaveWorkflow(ctx, workflow)
}

// GetWorkflowState retrieves a value from workflow state
func (m *Manager) GetWorkflowState(ctx context.Context, workflowID string, key string) (interface{}, error) {
	workflow, err := m.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	if workflow.State == nil {
		return nil, fmt.Errorf("state key not found")
	}

	value, exists := workflow.State[key]
	if !exists {
		return nil, fmt.Errorf("state key not found")
	}

	return value, nil
}

// SaveWorkflow persists a workflow to Redis
func (m *Manager) SaveWorkflow(ctx context.Context, workflow *Workflow) error {
	data, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	key := fmt.Sprintf("workflow:%s", workflow.ID)
	if err := m.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save workflow to Redis: %w", err)
	}

	// Add to workflows list
	if err := m.redisClient.SAdd(ctx, "workflows:list", workflow.ID).Err(); err != nil {
		return fmt.Errorf("failed to add workflow to list: %w", err)
	}
	
	// Publish workflow update event for dashboard
	go func() {
		update := map[string]interface{}{
			"type":        "workflow_update",
			"workflow_id": workflow.ID,
			"status":      string(workflow.Status),
			"timestamp":   time.Now(),
			"workflow":    workflow,
		}
		if updateData, err := json.Marshal(update); err == nil {
			m.redisClient.Publish(ctx, "workflow:updates", updateData)
		}
	}()

	return nil
}