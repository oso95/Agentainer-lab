package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/go-redis/redis/v8"
)

// Orchestrator manages workflow execution
type Orchestrator struct {
	workflowManager     *Manager
	agentManager        *agent.Manager
	redisClient         *redis.Client
	poolManager         *PoolManager
	metricsCollector    *MetricsCollector
	conditionEvaluator  *ConditionEvaluator
	subWorkflowExecutor *SubWorkflowExecutor
	stateManager        *StateManager
	performanceProfiler *PerformanceProfiler
	agentMonitor        *AgentMonitor
}

// NewOrchestrator creates a new workflow orchestrator
func NewOrchestrator(workflowManager *Manager, agentManager *agent.Manager, redisClient *redis.Client) *Orchestrator {
	stateManager := NewStateManager(redisClient)
	metricsCollector := NewMetricsCollector(redisClient)
	o := &Orchestrator{
		workflowManager:     workflowManager,
		agentManager:        agentManager,
		redisClient:         redisClient,
		poolManager:         NewPoolManager(agentManager, redisClient),
		metricsCollector:    metricsCollector,
		stateManager:        stateManager,
		performanceProfiler: NewPerformanceProfiler(redisClient, metricsCollector),
		agentMonitor:        NewAgentMonitor(agentManager),
	}
	o.conditionEvaluator = NewConditionEvaluator(stateManager)
	o.subWorkflowExecutor = NewSubWorkflowExecutor(workflowManager, o, stateManager)
	return o
}

// NewOrchestratorWithMetrics creates a new workflow orchestrator with a shared metrics collector
func NewOrchestratorWithMetrics(workflowManager *Manager, agentManager *agent.Manager, redisClient *redis.Client, metricsCollector *MetricsCollector) *Orchestrator {
	stateManager := NewStateManager(redisClient)
	o := &Orchestrator{
		workflowManager:     workflowManager,
		agentManager:        agentManager,
		redisClient:         redisClient,
		poolManager:         NewPoolManager(agentManager, redisClient),
		metricsCollector:    metricsCollector,
		stateManager:        stateManager,
		performanceProfiler: NewPerformanceProfiler(redisClient, metricsCollector),
		agentMonitor:        NewAgentMonitor(agentManager),
	}
	o.conditionEvaluator = NewConditionEvaluator(stateManager)
	o.subWorkflowExecutor = NewSubWorkflowExecutor(workflowManager, o, stateManager)
	return o
}

// mergeEnvVars merges two maps of environment variables
func mergeEnvVars(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// ExecuteWorkflow executes a workflow
func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflowID string) error {
	log.Printf("[Orchestrator] Starting workflow execution for ID: %s", workflowID)
	
	workflow, err := o.workflowManager.GetWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}
	
	log.Printf("[Orchestrator] Retrieved workflow: %s with %d steps", workflow.Name, len(workflow.Steps))

	// Start performance profiling if enabled
	if workflow.Config.EnableProfiling {
		o.performanceProfiler.StartProfiling(workflowID)
		defer func() {
			if profile, err := o.performanceProfiler.StopProfiling(workflowID); err == nil {
				log.Printf("Performance profile saved for workflow %s", workflowID)
				if len(profile.Recommendations) > 0 {
					log.Printf("Performance recommendations: %v", profile.Recommendations)
				}
			}
		}()
	}

	// Record workflow start
	o.metricsCollector.RecordWorkflowStart(workflowID, workflow.Name)

	// Update workflow status to running
	if err := o.workflowManager.UpdateWorkflowStatus(ctx, workflowID, WorkflowStatusRunning); err != nil {
		return fmt.Errorf("failed to update workflow status: %w", err)
	}

	// Execute steps in dependency order
	for i := range workflow.Steps {
		if err := o.executeStep(ctx, workflow, &workflow.Steps[i]); err != nil {
			// Handle failure based on strategy
			if workflow.Config.FailureStrategy == "fail_fast" {
				// Update workflow status directly instead of reloading
				workflow.Status = WorkflowStatusFailed
				workflow.UpdatedAt = time.Now()
				now := time.Now()
				workflow.CompletedAt = &now
				
				// Save the workflow with current step statuses
				if saveErr := o.workflowManager.SaveWorkflow(ctx, workflow); saveErr != nil {
					log.Printf("Warning: failed to save workflow after failure: %v", saveErr)
				}
				
				o.metricsCollector.RecordWorkflowComplete(workflowID, WorkflowStatusFailed)
				return fmt.Errorf("step %s failed: %w", workflow.Steps[i].ID, err)
			}
			// Continue on failure
			log.Printf("Step %s failed but continuing: %v", workflow.Steps[i].ID, err)
			o.metricsCollector.RecordStepError(workflow.ID, workflow.Steps[i].ID, err)
			o.metricsCollector.RecordStepComplete(workflow.ID, workflow.Steps[i].ID, StepStatusFailed, "", false)
		}
	}

	// Update workflow status to completed
	// Note: We need to update the workflow we've been working with, not reload from Redis
	workflow.Status = WorkflowStatusCompleted
	workflow.UpdatedAt = time.Now()
	now := time.Now()
	workflow.CompletedAt = &now
	
	// Save the workflow with all the updated step statuses
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		return fmt.Errorf("failed to save completed workflow: %w", err)
	}
	
	log.Printf("Workflow %s completed with all step statuses preserved", workflowID)

	// Record workflow completion
	o.metricsCollector.RecordWorkflowComplete(workflowID, WorkflowStatusCompleted)

	return nil
}

// executeStep executes a single workflow step
func (o *Orchestrator) executeStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Check dependencies
	if err := o.waitForDependencies(ctx, workflow, step); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Check condition if specified
	if step.Config.Condition != nil {
		canExecute, err := o.conditionEvaluator.Evaluate(ctx, workflow.ID, *step.Config.Condition)
		if err != nil {
			return fmt.Errorf("failed to evaluate condition: %w", err)
		}
		if !canExecute {
			// Skip this step
			step.Status = StepStatusSkipped
			now := time.Now()
			step.CompletedAt = &now
			log.Printf("Skipping step %s due to condition", step.ID)
			return nil
		}
	}

	// Update step status to running
	step.Status = StepStatusRunning
	now := time.Now()
	step.StartedAt = &now

	// Record step start
	o.metricsCollector.RecordStepStart(workflow.ID, step.ID, step.Name)

	// Start step profiling
	if workflow.Config.EnableProfiling {
		o.performanceProfiler.ProfileStep(workflow.ID, step.ID, step.Name)
		defer o.performanceProfiler.CompleteStepProfile(workflow.ID, step.ID)
	}

	switch step.Type {
	case StepTypeSequential:
		return o.executeSequentialStep(ctx, workflow, step)
	case StepTypeParallel:
		return o.executeParallelStep(ctx, workflow, step)
	case StepTypeMap:
		return o.executeMapStep(ctx, workflow, step)
	case StepTypeReduce:
		return o.executeReduceStep(ctx, workflow, step)
	case StepTypeDecision:
		return o.executeDecisionStep(ctx, workflow, step)
	case StepTypeBranch:
		return o.executeBranchStep(ctx, workflow, step)
	case StepTypeSubWorkflow:
		return o.executeSubWorkflowStep(ctx, workflow, step)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

// waitForTaskCompletion waits for a task to complete using Redis Pub/Sub
func (o *Orchestrator) waitForTaskCompletion(ctx context.Context, taskID string, agentID string, timeout time.Duration) (map[string]interface{}, error) {
	// Subscribe to task completion channel
	completionChannel := fmt.Sprintf("task:%s:complete", taskID)
	pubsub := o.redisClient.Subscribe(ctx, completionChannel)
	defer pubsub.Close()
	
	// Also monitor agent status periodically (less frequently)
	agentCheckTicker := time.NewTicker(2 * time.Second)
	defer agentCheckTicker.Stop()
	
	timeoutChan := time.After(timeout)
	resultKey := fmt.Sprintf("task:%s:result", taskID)
	
	for {
		select {
		case msg := <-pubsub.Channel():
			// Task completed notification received
			log.Printf("Received task completion notification for task %s: %s", taskID, msg.Payload)
			
			// Get the actual result
			resultData, err := o.redisClient.Get(ctx, resultKey).Result()
			if err != nil {
				log.Printf("Error getting task result after completion notification: %v", err)
				// Try once more after a short delay
				time.Sleep(100 * time.Millisecond)
				resultData, err = o.redisClient.Get(ctx, resultKey).Result()
				if err != nil {
					return nil, fmt.Errorf("failed to get task result: %w", err)
				}
			}
			
			// Parse result
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(resultData), &result); err != nil {
				log.Printf("Warning: failed to unmarshal task result: %v", err)
			}
			
			// Check if it's an error completion
			if msg.Payload == "error" {
				errorKey := fmt.Sprintf("task:%s:error", taskID)
				if errorMsg, err := o.redisClient.Get(ctx, errorKey).Result(); err == nil {
					return nil, fmt.Errorf("task failed: %s", errorMsg)
				}
			}
			
			return result, nil
			
		case <-agentCheckTicker.C:
			// Periodically check if agent is still running
			agent, err := o.agentManager.GetAgent(agentID)
			if err != nil {
				log.Printf("Warning: failed to check agent status: %v", err)
				continue
			}
			
			// If agent exited without publishing completion
			if agent.Status == "failed" || agent.Status == "stopped" {
				// Check if result was written but notification failed
				if resultData, err := o.redisClient.Get(ctx, resultKey).Result(); err == nil {
					// Process as completed
					var result map[string]interface{}
					if err := json.Unmarshal([]byte(resultData), &result); err != nil {
						log.Printf("Warning: failed to unmarshal task result: %v", err)
					}
					return result, nil
				}
				
				// Check for error message
				errorKey := fmt.Sprintf("task:%s:error", taskID)
				if errorMsg, err := o.redisClient.Get(ctx, errorKey).Result(); err == nil {
					return nil, fmt.Errorf("task failed: %s", errorMsg)
				}
				
				return nil, fmt.Errorf("agent stopped without completing task")
			}
			
		case <-timeoutChan:
			return nil, fmt.Errorf("task timeout after %v", timeout)
		}
	}
}

// executeSequentialStep executes a sequential step
func (o *Orchestrator) executeSequentialStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Create task for this step
	taskID := fmt.Sprintf("task-%s-%s-%d", workflow.ID, step.ID, time.Now().UnixNano())
	task := map[string]interface{}{
		"task_id":     taskID,
		"workflow_id": workflow.ID,
		"step_id":     step.ID,
		"step_name":   step.Name,
		"task_type":   step.Config.EnvVars["TASK_TYPE"], // Get task type from env vars
		"input":       workflow.State,                    // Pass workflow state as input
		"created_at":  time.Now().Unix(),
	}
	
	// Write task to Redis
	taskKey := fmt.Sprintf("task:%s", taskID)
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	
	if err := o.redisClient.Set(ctx, taskKey, taskData, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to write task to Redis: %w", err)
	}
	
	// Also add to workflow's task list
	if err := o.redisClient.RPush(ctx, fmt.Sprintf("workflow:%s:tasks", workflow.ID), taskID).Err(); err != nil {
		log.Printf("Warning: failed to add task to workflow list: %v", err)
	}
	
	// Prepare environment variables with Redis connection info
	envVars := make(map[string]string)
	for k, v := range step.Config.EnvVars {
		envVars[k] = v
	}
	
	// Add task ID to environment
	envVars["TASK_ID"] = taskID
	
	// Add Redis connection info if not already present
	if _, ok := envVars["REDIS_HOST"]; !ok {
		// Use the same Redis host that Agentainer is configured with
		redisHost := os.Getenv("AGENTAINER_REDIS_HOST")
		if redisHost == "" {
			// Default to host.docker.internal for Docker Desktop (Mac/Windows)
			// On Linux, this might need to be the Docker bridge IP (172.17.0.1)
			redisHost = "host.docker.internal"
		}
		envVars["REDIS_HOST"] = redisHost
	}
	if _, ok := envVars["REDIS_PORT"]; !ok {
		envVars["REDIS_PORT"] = "6379"
	}
	
	// Deploy agent with workflow metadata
	agent, err := o.agentManager.DeployWithWorkflow(
		ctx,
		fmt.Sprintf("%s-%s", workflow.Name, step.Name),
		step.Config.Image,
		envVars,
		step.Config.ResourceLimits.CPULimit,
		step.Config.ResourceLimits.MemoryLimit,
		false, // auto-restart
		"",    // token
		nil,   // ports
		nil,   // volumes
		nil,   // health check
		workflow.ID,
		step.ID,
		taskID,
		workflow.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy agent: %w", err)
	}

	// Add job to workflow
	if err := o.workflowManager.AddJobToWorkflow(ctx, workflow.ID, agent.ID); err != nil {
		return fmt.Errorf("failed to add job to workflow: %w", err)
	}

	// Start the agent
	if err := o.agentManager.Start(ctx, agent.ID); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Mark step as running
	step.Status = StepStatusRunning
	stepStartTime := time.Now()
	step.StartedAt = &stepStartTime
	
	// Save updated workflow to persist step status
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after marking step as running: %v", err)
	}
	
	o.metricsCollector.RecordStepStart(workflow.ID, step.ID, step.Name)

	// Wait for task completion using Pub/Sub
	timeout := 5 * time.Minute // Default timeout
	if step.Config.Timeout != "" {
		if parsed, err := time.ParseDuration(step.Config.Timeout); err == nil {
			timeout = parsed
		}
	} else if workflow.Config.Timeout != "" {
		if parsed, err := time.ParseDuration(workflow.Config.Timeout); err == nil {
			timeout = parsed
		}
	}

	// Wait for task completion
	result, err := o.waitForTaskCompletion(ctx, taskID, agent.ID, timeout)
	if err != nil {
		step.Status = StepStatusFailed
		now := time.Now()
		step.CompletedAt = &now
		o.workflowManager.SaveWorkflow(ctx, workflow)
		o.metricsCollector.RecordStepError(workflow.ID, step.ID, err)
		o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusFailed, agent.ID, false)
		return err
	}
	
	// Update workflow state with result
	if result != nil {
		for k, v := range result {
			workflow.State[k] = v
		}
	}
	
	// Mark step as completed
	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now
	
	// Save updated workflow to persist step status
	log.Printf("Saving workflow %s after step %s completion", workflow.ID, step.ID)
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after step completion: %v", err)
	} else {
		log.Printf("Successfully saved workflow %s with step %s status: %s", workflow.ID, step.ID, step.Status)
	}
	
	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, agent.ID, false)
	
	return nil
}

// executeParallelStep executes a parallel step with multiple workers
func (o *Orchestrator) executeParallelStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Determine number of parallel tasks
	maxWorkers := step.Config.MaxWorkers
	if maxWorkers == 0 {
		maxWorkers = 5 // Default
	}

	// Check if we should use pooled execution
	if step.Config.ExecutionMode == "pooled" && step.Config.PoolConfig != nil {
		return o.executePooledParallelStep(ctx, workflow, step, maxWorkers)
	}

	// Standard parallel execution (without pooling)
	var wg sync.WaitGroup
	errors := make(chan error, maxWorkers)
	agentIDs := make([]string, 0, maxWorkers)
	agentIDsMutex := &sync.Mutex{}

	// Mark step as running
	step.Status = StepStatusRunning
	stepStartTime := time.Now()
	step.StartedAt = &stepStartTime
	
	// Save updated workflow to persist step status
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after marking step as running: %v", err)
	}
	
	o.metricsCollector.RecordStepStart(workflow.ID, step.ID, step.Name)

	// Deploy and start all agents
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Create task for this worker
			taskID := fmt.Sprintf("task-%s-%s-%d-%d", workflow.ID, step.ID, workerID, time.Now().UnixNano())
			task := map[string]interface{}{
				"task_id":     taskID,
				"workflow_id": workflow.ID,
				"step_id":     step.ID,
				"step_name":   step.Name,
				"worker_id":   workerID,
				"task_type":   step.Config.EnvVars["TASK_TYPE"],
				"input":       workflow.State,
				"created_at":  time.Now().Unix(),
			}
			
			// Write task to Redis
			taskKey := fmt.Sprintf("task:%s", taskID)
			taskData, err := json.Marshal(task)
			if err != nil {
				errors <- fmt.Errorf("failed to marshal task for worker %d: %w", workerID, err)
				return
			}
			
			if err := o.redisClient.Set(ctx, taskKey, taskData, 1*time.Hour).Err(); err != nil {
				errors <- fmt.Errorf("failed to write task to Redis for worker %d: %w", workerID, err)
				return
			}

			// Prepare environment variables with Redis connection info
			envVars := make(map[string]string)
			for k, v := range step.Config.EnvVars {
				envVars[k] = v
			}
			
			// Add task ID to environment
			envVars["TASK_ID"] = taskID
			envVars["WORKER_ID"] = fmt.Sprintf("%d", workerID)
			
			// Add Redis connection info if not already present
			if _, ok := envVars["REDIS_HOST"]; !ok {
				redisHost := os.Getenv("AGENTAINER_REDIS_HOST")
				if redisHost == "" {
					redisHost = "host.docker.internal"
				}
				envVars["REDIS_HOST"] = redisHost
			}
			if _, ok := envVars["REDIS_PORT"]; !ok {
				envVars["REDIS_PORT"] = "6379"
			}

			// Deploy agent with workflow metadata
			agent, err := o.agentManager.DeployWithWorkflow(
				ctx,
				fmt.Sprintf("%s-%s-%d", workflow.Name, step.Name, workerID),
				step.Config.Image,
				envVars,
				step.Config.ResourceLimits.CPULimit,
				step.Config.ResourceLimits.MemoryLimit,
				false, // auto-restart
				"",    // token
				nil,   // ports
				nil,   // volumes
				nil,   // health check
				workflow.ID,
				step.ID,
				taskID,
				workflow.Metadata,
			)
			if err != nil {
				errors <- fmt.Errorf("failed to deploy agent for task %d: %w", workerID, err)
				return
			}

			// Add job to workflow
			if err := o.workflowManager.AddJobToWorkflow(ctx, workflow.ID, agent.ID); err != nil {
				errors <- fmt.Errorf("failed to add job to workflow: %w", err)
				return
			}

			// Start the agent
			if err := o.agentManager.Start(ctx, agent.ID); err != nil {
				errors <- fmt.Errorf("failed to start agent for task %d: %w", workerID, err)
				return
			}

			// Collect agent ID with task ID
			agentIDsMutex.Lock()
			agentIDs = append(agentIDs, agent.ID)
			agentIDsMutex.Unlock()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for deployment errors
	for err := range errors {
		if err != nil {
			step.Status = StepStatusFailed
			o.metricsCollector.RecordStepError(workflow.ID, step.ID, err)
			o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusFailed, "", false)
			return err
		}
	}

	// Wait for all agents to complete
	timeout := 10 * time.Minute // Default timeout for parallel steps
	if workflow.Config.Timeout != "" {
		if parsed, err := time.ParseDuration(workflow.Config.Timeout); err == nil {
			timeout = parsed
		}
	}

	log.Printf("Waiting for %d agents to complete for step %s", len(agentIDs), step.ID)
	completedAgents, err := o.agentMonitor.WaitForMultipleAgents(ctx, agentIDs, timeout)
	if err != nil {
		step.Status = StepStatusFailed
		o.metricsCollector.RecordStepError(workflow.ID, step.ID, err)
		o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusFailed, "", false)
		return fmt.Errorf("parallel execution failed: %w", err)
	}

	// Check if any agents failed
	failedAgents := 0
	for _, agent := range completedAgents {
		if agent.Status == "failed" {
			failedAgents++
		}
	}

	if failedAgents > 0 {
		step.Status = StepStatusFailed
		o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusFailed, "", false)
		return fmt.Errorf("%d out of %d agents failed", failedAgents, len(agentIDs))
	}

	// All agents completed successfully
	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now

	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, "", false)

	return nil
}

// executePooledParallelStep executes a parallel step using agent pooling
func (o *Orchestrator) executePooledParallelStep(ctx context.Context, workflow *Workflow, step *WorkflowStep, maxWorkers int) error {
	log.Printf("Executing pooled parallel step %s for workflow %s", step.ID, workflow.ID)
	
	// Mark step as running first
	step.Status = StepStatusRunning
	stepStartTime := time.Now()
	step.StartedAt = &stepStartTime
	
	// Save updated workflow to persist step status
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after marking step as running: %v", err)
	}
	
	o.metricsCollector.RecordStepStart(workflow.ID, step.ID, step.Name)
	
	// For dynamic parallel steps, determine tasks from workflow state
	var tasks []map[string]interface{}
	if step.Config.Dynamic {
		// Extract tasks from workflow state
		if tasksData, ok := workflow.State["tasks"].([]interface{}); ok {
			for _, t := range tasksData {
				if taskMap, ok := t.(map[string]interface{}); ok {
					tasks = append(tasks, taskMap)
				}
			}
		}
		log.Printf("Found %d dynamic tasks for parallel execution", len(tasks))
	}
	
	// If no dynamic tasks, create default tasks based on maxWorkers
	if len(tasks) == 0 {
		for i := 0; i < maxWorkers; i++ {
			tasks = append(tasks, map[string]interface{}{
				"worker_id": i,
				"task_type": step.Config.EnvVars["TASK_TYPE"],
			})
		}
	}
	
	// Get or create pool for this image
	pool, err := o.poolManager.GetOrCreatePool(ctx, step.Config.Image, step.Config.PoolConfig)
	if err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}

	// Pre-warm pool if configured
	if step.Config.PoolConfig.WarmUp {
		if err := pool.WarmUp(ctx); err != nil {
			log.Printf("Warning: failed to warm up pool: %v", err)
		}
	}

	// Execute tasks using the pool - but actually do the task-based execution
	// Since our workflow agents use task-based pattern, we'll fall back to standard execution
	log.Printf("Pooled agents created, falling back to standard parallel execution for task-based workflow")
	
	// Use standard parallel execution for now since task-based pattern requires actual task creation
	var wg sync.WaitGroup
	errors := make(chan error, len(tasks))
	agentIDs := make([]string, 0, len(tasks))
	agentIDsMutex := &sync.Mutex{}
	
	// Deploy and start all agents for tasks
	for i, taskData := range tasks {
		wg.Add(1)
		go func(workerID int, task map[string]interface{}) {
			defer wg.Done()

			// Create task ID
			taskID := fmt.Sprintf("task-%s-%s-%d-%d", workflow.ID, step.ID, workerID, time.Now().UnixNano())
			
			// Merge task data with workflow context
			fullTask := map[string]interface{}{
				"task_id":     taskID,
				"workflow_id": workflow.ID,
				"step_id":     step.ID,
				"step_name":   step.Name,
				"worker_id":   workerID,
				"input":       workflow.State,
				"created_at":  time.Now().Unix(),
			}
			
			// Merge in task-specific data
			for k, v := range task {
				fullTask[k] = v
			}
			
			// Add step env vars
			for k, v := range step.Config.EnvVars {
				fullTask[k] = v
			}
			
			// Write task to Redis
			taskKey := fmt.Sprintf("task:%s", taskID)
			taskData, err := json.Marshal(fullTask)
			if err != nil {
				errors <- fmt.Errorf("failed to marshal task: %w", err)
				return
			}
			
			if err := o.redisClient.Set(ctx, taskKey, taskData, 1*time.Hour).Err(); err != nil {
				errors <- fmt.Errorf("failed to write task to Redis: %w", err)
				return
			}
			
			log.Printf("Created task %s for parallel worker %d", taskID, workerID)

			// Deploy agent for this task
			envVars := mergeEnvVars(step.Config.EnvVars, map[string]string{
				"TASK_ID": taskID,
				"WORKFLOW_ID": workflow.ID,
				"STEP_ID": step.ID,
				"WORKER_ID": fmt.Sprintf("%d", workerID),
			})
			
			// Add Redis connection info if not already present
			if _, ok := envVars["REDIS_HOST"]; !ok {
				redisHost := os.Getenv("AGENTAINER_REDIS_HOST")
				if redisHost == "" {
					redisHost = "host.docker.internal"
				}
				envVars["REDIS_HOST"] = redisHost
			}
			if _, ok := envVars["REDIS_PORT"]; !ok {
				envVars["REDIS_PORT"] = "6379"
			}

			cpuLimit := int64(1000000000) // Default 1 CPU
			memLimit := int64(536870912)  // Default 512MB
			if step.Config.ResourceLimits != nil {
				cpuLimit = step.Config.ResourceLimits.CPULimit
				memLimit = step.Config.ResourceLimits.MemoryLimit
			}

			agentObj, err := o.agentManager.DeployWithWorkflow(
				ctx,
				fmt.Sprintf("%s-%s-worker-%d", workflow.Name, step.Name, workerID),
				step.Config.Image,
				envVars,
				cpuLimit,
				memLimit,
				false, // auto-restart
				"",    // token
				nil,   // ports
				nil,   // volumes
				nil,   // health check
				workflow.ID,
				step.ID,
				taskID,
				workflow.Metadata,
			)
			if err != nil {
				errors <- fmt.Errorf("failed to deploy agent: %w", err)
				return
			}
			
			// Add job to workflow
			if err := o.workflowManager.AddJobToWorkflow(ctx, workflow.ID, agentObj.ID); err != nil {
				log.Printf("Warning: failed to add job to workflow: %v", err)
			}
			
			// Start the agent
			if err := o.agentManager.Start(ctx, agentObj.ID); err != nil {
				errors <- fmt.Errorf("failed to start agent: %w", err)
				return
			}
			
			agentIDsMutex.Lock()
			agentIDs = append(agentIDs, agentObj.ID)
			agentIDsMutex.Unlock()

			// Wait for task completion
			taskTimeout := 10 * time.Minute
			if step.Config.Timeout != "" {
				if parsed, err := time.ParseDuration(step.Config.Timeout); err == nil {
					taskTimeout = parsed
				}
			}

			result, err := o.waitForTaskCompletion(ctx, taskID, agentObj.ID, taskTimeout)
			if err != nil {
				errors <- fmt.Errorf("task %s failed: %w", taskID, err)
				return
			}

			// Store result in workflow state
			if result != nil {
				stateKey := fmt.Sprintf("%s_result_%d", step.ID, workerID)
				workflow.State[stateKey] = result
			}
		}(i, taskData)
	}

	wg.Wait()
	close(errors)

	// Collect all errors
	var allErrors []error
	for err := range errors {
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Clean up agents - agents will auto-terminate after task completion
	for _, agentID := range agentIDs {
		if err := o.agentManager.Stop(ctx, agentID); err != nil {
			log.Printf("Warning: failed to stop agent %s: %v", agentID, err)
		}
	}

	if len(allErrors) > 0 {
		step.Status = StepStatusFailed
		now := time.Now()
		step.CompletedAt = &now
		
		// Save workflow state before returning error
		if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
			log.Printf("Warning: failed to save workflow after parallel step failure: %v", err)
		}
		
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(allErrors), allErrors[0])
	}

	// Merge all results into workflow state
	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now
	
	// Save updated workflow to persist step status and results
	log.Printf("Saving workflow %s after parallel step %s completion", workflow.ID, step.ID)
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after step completion: %v", err)
	} else {
		log.Printf("Successfully saved workflow %s with step %s status: %s", workflow.ID, step.ID, step.Status)
	}

	// Record step completion with pooling metrics
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, "", true)

	// Get pool stats and record resource usage
	stats := pool.GetStats()
	usage := ResourceUsage{
		AgentsDeployed: stats.TotalAgents,
		AgentsReused:   int(stats.TotalUses) - stats.TotalAgents,
		PoolUtilization: float64(stats.ActiveAgents) / float64(stats.TotalAgents),
	}
	o.metricsCollector.RecordResourceUsage(workflow.ID, step.ID, usage)

	return nil
}

// executeMapStep executes a map step that creates parallel tasks for each item in an array
func (o *Orchestrator) executeMapStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	if step.Config.MapConfig == nil {
		return fmt.Errorf("map step requires map_config")
	}

	mapConfig := step.Config.MapConfig
	
	// Extract the array to map over from workflow state
	inputData, err := o.extractMapInput(workflow.State, mapConfig.InputPath)
	if err != nil {
		return fmt.Errorf("failed to extract map input: %w", err)
	}

	// Validate input is an array
	items, ok := inputData.([]interface{})
	if !ok {
		return fmt.Errorf("map input must be an array, got %T", inputData)
	}

	if len(items) == 0 {
		log.Printf("No items to map over for step %s", step.ID)
		step.Status = StepStatusCompleted
		return nil
	}

	log.Printf("Executing map step %s with %d items", step.ID, len(items))
	
	// Update step status
	step.Status = StepStatusRunning
	step.StartedAt = timePtr(time.Now())
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}

	// Track map execution state
	mapState := &MapStepState{
		TotalItems:     len(items),
		CompletedItems: 0,
		FailedItems:    0,
		Results:        make([]interface{}, len(items)),
	}
	
	// Save map state
	if err := o.stateManager.SetMapState(ctx, workflow.ID, step.ID, mapState); err != nil {
		return fmt.Errorf("failed to save map state: %w", err)
	}

	// Determine concurrency
	maxConcurrency := mapConfig.MaxConcurrency
	if maxConcurrency == 0 || maxConcurrency > len(items) {
		maxConcurrency = len(items)
	}

	// Create tasks for each item
	taskChan := make(chan int, len(items))
	for i := range items {
		taskChan <- i
	}
	close(taskChan)

	// Launch workers
	var wg sync.WaitGroup
	errorChan := make(chan error, maxConcurrency)
	
	for w := 0; w < maxConcurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for itemIndex := range taskChan {
				// Create task for this item
				taskID := fmt.Sprintf("task-%s-%s-map-%d-%d", workflow.ID, step.ID, itemIndex, time.Now().UnixNano())
				
				// Create input for this specific item
				taskInput := make(map[string]interface{})
				// Copy workflow state
				for k, v := range workflow.State {
					taskInput[k] = v
				}
				// Add the current item
				taskInput[mapConfig.ItemAlias] = items[itemIndex]
				taskInput["_map_index"] = itemIndex
				
				task := map[string]interface{}{
					"task_id":     taskID,
					"workflow_id": workflow.ID,
					"step_id":     step.ID,
					"step_name":   fmt.Sprintf("%s[%d]", step.Name, itemIndex),
					"task_type":   step.Config.EnvVars["TASK_TYPE"],
					"input":       taskInput,
					"created_at":  time.Now().Unix(),
				}
				
				// Store task in Redis
				taskKey := fmt.Sprintf("task:%s", taskID)
				if err := o.redisClient.Set(ctx, taskKey, mustMarshal(task), 1*time.Hour).Err(); err != nil {
					log.Printf("Failed to store task %s: %v", taskID, err)
					if mapConfig.ErrorHandling == "fail_fast" {
						errorChan <- fmt.Errorf("failed to store task: %w", err)
						return
					}
					continue
				}
				
				// Deploy agent for this task
				envVars := make(map[string]string)
				for k, v := range step.Config.EnvVars {
					envVars[k] = v
				}
				envVars["TASK_ID"] = taskID
				envVars["WORKFLOW_ID"] = workflow.ID
				envVars["STEP_ID"] = step.ID
				envVars["MAP_INDEX"] = fmt.Sprintf("%d", itemIndex)
				
				// Set Redis host
				redisHost := os.Getenv("AGENTAINER_REDIS_HOST")
				if redisHost == "" {
					redisHost = "host.docker.internal"
				}
				envVars["REDIS_HOST"] = redisHost
				
				agentName := fmt.Sprintf("%s-%s-map-%d", workflow.Name, step.Name, itemIndex)
				_, err := o.agentManager.Deploy(
					ctx,
					agentName,
					step.Config.Image,
					envVars,
					step.Config.ResourceLimits.CPULimit,
					step.Config.ResourceLimits.MemoryLimit,
					false, // auto-restart
					"",    // token
					nil,   // ports
					nil,   // volumes
					nil,   // health check
				)
				
				if err != nil {
					log.Printf("Failed to deploy agent for map item %d: %v", itemIndex, err)
					if mapConfig.ErrorHandling == "fail_fast" {
						errorChan <- fmt.Errorf("failed to deploy agent: %w", err)
						return
					}
					// Update map state for failed item
					o.updateMapItemState(ctx, workflow.ID, step.ID, itemIndex, nil, err)
					continue
				}
				
				// Wait for task completion
				result, err := o.waitForTaskCompletion(ctx, taskID, agentName, 5*time.Minute)
				if err != nil {
					log.Printf("Map item %d failed: %v", itemIndex, err)
					if mapConfig.ErrorHandling == "fail_fast" {
						errorChan <- fmt.Errorf("map item %d failed: %w", itemIndex, err)
						return
					}
				}
				
				// Update map state for completed item
				o.updateMapItemState(ctx, workflow.ID, step.ID, itemIndex, result, err)
			}
		}(w)
	}
	
	wg.Wait()
	close(errorChan)
	
	// Check for errors
	for err := range errorChan {
		if err != nil {
			step.Status = StepStatusFailed
			o.metricsCollector.RecordStepError(workflow.ID, step.ID, err)
			return err
		}
	}
	
	// Get final map state
	finalMapState, err := o.stateManager.GetMapState(ctx, workflow.ID, step.ID)
	if err != nil {
		return fmt.Errorf("failed to get final map state: %w", err)
	}
	
	// Store map results in workflow state
	resultKey := fmt.Sprintf("%s_results", step.ID)
	workflow.State[resultKey] = finalMapState.Results
	
	// Update step status
	if finalMapState.FailedItems > 0 && mapConfig.ErrorHandling == "fail_fast" {
		step.Status = StepStatusFailed
	} else {
		step.Status = StepStatusCompleted
	}
	step.CompletedAt = timePtr(time.Now())
	
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, step.Status, "", false)
	
	return o.workflowManager.SaveWorkflow(ctx, workflow)
}

// executeReduceStep executes a reduce step that aggregates results
func (o *Orchestrator) executeReduceStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Create task for reducer
	taskID := fmt.Sprintf("task-%s-%s-reduce-%d", workflow.ID, step.ID, time.Now().UnixNano())
	task := map[string]interface{}{
		"task_id":     taskID,
		"workflow_id": workflow.ID,
		"step_id":     step.ID,
		"step_name":   step.Name,
		"task_type":   step.Config.EnvVars["TASK_TYPE"],
		"input":       workflow.State,
		"created_at":  time.Now().Unix(),
	}
	
	// Write task to Redis
	taskKey := fmt.Sprintf("task:%s", taskID)
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal reducer task: %w", err)
	}
	
	if err := o.redisClient.Set(ctx, taskKey, taskData, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to write reducer task to Redis: %w", err)
	}
	
	// Prepare environment variables with Redis connection info
	envVars := make(map[string]string)
	for k, v := range step.Config.EnvVars {
		envVars[k] = v
	}
	
	// Add task ID to environment
	envVars["TASK_ID"] = taskID
	
	// Add Redis connection info if not already present
	if _, ok := envVars["REDIS_HOST"]; !ok {
		redisHost := os.Getenv("AGENTAINER_REDIS_HOST")
		if redisHost == "" {
			redisHost = "host.docker.internal"
		}
		envVars["REDIS_HOST"] = redisHost
	}
	if _, ok := envVars["REDIS_PORT"]; !ok {
		envVars["REDIS_PORT"] = "6379"
	}
	
	// Deploy reducer agent
	agent, err := o.agentManager.DeployWithWorkflow(
		ctx,
		fmt.Sprintf("%s-%s-reducer", workflow.Name, step.Name),
		step.Config.Image,
		envVars,
		step.Config.ResourceLimits.CPULimit,
		step.Config.ResourceLimits.MemoryLimit,
		false, // auto-restart
		"",    // token
		nil,   // ports
		nil,   // volumes
		nil,   // health check
		workflow.ID,
		step.ID,
		taskID,
		workflow.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy reducer agent: %w", err)
	}

	// Add job to workflow
	if err := o.workflowManager.AddJobToWorkflow(ctx, workflow.ID, agent.ID); err != nil {
		return fmt.Errorf("failed to add reducer job to workflow: %w", err)
	}

	// Start the agent
	if err := o.agentManager.Start(ctx, agent.ID); err != nil {
		return fmt.Errorf("failed to start reducer agent: %w", err)
	}

	// Mark step as running
	step.Status = StepStatusRunning
	stepStartTime := time.Now()
	step.StartedAt = &stepStartTime
	
	// Save updated workflow to persist step status
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after marking step as running: %v", err)
	}
	
	o.metricsCollector.RecordStepStart(workflow.ID, step.ID, step.Name)

	// Wait for task completion
	timeout := 5 * time.Minute // Default timeout
	if step.Config.Timeout != "" {
		if parsed, err := time.ParseDuration(step.Config.Timeout); err == nil {
			timeout = parsed
		}
	} else if workflow.Config.Timeout != "" {
		if parsed, err := time.ParseDuration(workflow.Config.Timeout); err == nil {
			timeout = parsed
		}
	}

	// Wait for task completion using Pub/Sub
	result, err := o.waitForTaskCompletion(ctx, taskID, agent.ID, timeout)
	if err != nil {
		step.Status = StepStatusFailed
		now := time.Now()
		step.CompletedAt = &now
		o.workflowManager.SaveWorkflow(ctx, workflow)
		o.metricsCollector.RecordStepError(workflow.ID, step.ID, err)
		o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusFailed, agent.ID, false)
		return err
	}
	
	// Update workflow state with reduced result
	if result != nil {
		for k, v := range result {
			workflow.State[k] = v
		}
	}
	
	// Mark step as completed
	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now
	
	// Save updated workflow to persist step status
	log.Printf("Saving workflow %s after reduce step %s completion", workflow.ID, step.ID)
	if err := o.workflowManager.SaveWorkflow(ctx, workflow); err != nil {
		log.Printf("Warning: failed to save workflow after reduce step completion: %v", err)
	}
	
	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, agent.ID, false)
	
	return nil
}

// executeDecisionStep executes a decision node
func (o *Orchestrator) executeDecisionStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	if step.Config.DecisionNode == nil {
		return fmt.Errorf("decision step %s has no decision node configuration", step.ID)
	}

	// Evaluate decision node
	selectedBranch, err := o.conditionEvaluator.EvaluateDecisionNode(ctx, workflow.ID, *step.Config.DecisionNode)
	if err != nil {
		return fmt.Errorf("failed to evaluate decision node: %w", err)
	}

	log.Printf("Decision node %s selected branch: %s", step.ID, selectedBranch.Name)

	// Store decision result in state
	if err := o.stateManager.SetValue(ctx, workflow.ID, fmt.Sprintf("decision_%s_result", step.ID), selectedBranch.ID); err != nil {
		log.Printf("Failed to store decision result: %v", err)
	}

	// Execute next steps based on branch
	if len(selectedBranch.NextSteps) > 0 {
		// Mark specified steps as ready to execute
		for _, nextStepID := range selectedBranch.NextSteps {
			for i := range workflow.Steps {
				if workflow.Steps[i].ID == nextStepID {
					// Remove dependency on this decision step
					newDeps := []string{}
					for _, dep := range workflow.Steps[i].DependsOn {
						if dep != step.ID {
							newDeps = append(newDeps, dep)
						}
					}
					workflow.Steps[i].DependsOn = newDeps
				}
			}
		}
	}

	// Execute sub-workflow if specified
	if selectedBranch.Workflow != "" {
		subWorkflow, err := o.subWorkflowExecutor.ExecuteSubWorkflow(
			ctx,
			workflow.ID,
			step.ID,
			selectedBranch.Workflow,
			workflow.State,
		)
		if err != nil {
			return fmt.Errorf("failed to execute sub-workflow: %w", err)
		}

		// Merge sub-workflow state back
		for k, v := range subWorkflow.State {
			workflow.State[k] = v
		}
	}

	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now

	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, "", false)

	return nil
}

// executeBranchStep executes a conditional branch
func (o *Orchestrator) executeBranchStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	if step.Config.BranchConfig == nil {
		return fmt.Errorf("branch step %s has no branch configuration", step.ID)
	}

	branch := step.Config.BranchConfig

	// Evaluate condition
	result, err := o.conditionEvaluator.Evaluate(ctx, workflow.ID, branch.Condition)
	if err != nil {
		return fmt.Errorf("failed to evaluate branch condition: %w", err)
	}

	log.Printf("Branch step %s condition evaluated to: %v", step.ID, result)

	// Execute appropriate branch
	if result {
		// Execute true branch
		if len(branch.TrueSteps) > 0 {
			for _, stepID := range branch.TrueSteps {
				// Enable step execution
				o.enableStep(workflow, stepID)
			}
		}
		if branch.TrueWorkflow != "" {
			_, err := o.subWorkflowExecutor.ExecuteSubWorkflow(
				ctx,
				workflow.ID,
				step.ID,
				branch.TrueWorkflow,
				workflow.State,
			)
			if err != nil {
				return fmt.Errorf("failed to execute true branch workflow: %w", err)
			}
		}
	} else {
		// Execute false branch
		if len(branch.FalseSteps) > 0 {
			for _, stepID := range branch.FalseSteps {
				o.enableStep(workflow, stepID)
			}
		}
		if branch.FalseWorkflow != "" {
			_, err := o.subWorkflowExecutor.ExecuteSubWorkflow(
				ctx,
				workflow.ID,
				step.ID,
				branch.FalseWorkflow,
				workflow.State,
			)
			if err != nil {
				return fmt.Errorf("failed to execute false branch workflow: %w", err)
			}
		}
	}

	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now

	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, "", false)

	return nil
}

// executeSubWorkflowStep executes a sub-workflow
func (o *Orchestrator) executeSubWorkflowStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	subWorkflowID := step.Config.SubWorkflowID
	if subWorkflowID == "" {
		subWorkflowID = step.Config.SubWorkflowName
	}

	if subWorkflowID == "" {
		return fmt.Errorf("sub-workflow step %s has no workflow ID or name", step.ID)
	}

	// Execute sub-workflow
	subWorkflow, err := o.subWorkflowExecutor.ExecuteSubWorkflow(
		ctx,
		workflow.ID,
		step.ID,
		subWorkflowID,
		workflow.State,
	)
	if err != nil {
		return fmt.Errorf("failed to execute sub-workflow: %w", err)
	}

	// Merge sub-workflow state back to parent
	for k, v := range subWorkflow.State {
		workflow.State[k] = v
	}

	// Store sub-workflow result
	step.Results = map[string]interface{}{
		"sub_workflow_id": subWorkflow.ID,
		"status":          subWorkflow.Status,
		"state":           subWorkflow.State,
	}

	step.Status = StepStatusCompleted
	now := time.Now()
	step.CompletedAt = &now

	// Record step completion
	o.metricsCollector.RecordStepComplete(workflow.ID, step.ID, StepStatusCompleted, "", false)

	return nil
}

// enableStep removes dependencies to enable step execution
func (o *Orchestrator) enableStep(workflow *Workflow, stepID string) {
	for i := range workflow.Steps {
		if workflow.Steps[i].ID == stepID {
			// Clear dependencies to allow execution
			workflow.Steps[i].DependsOn = []string{}
			log.Printf("Enabled step %s for execution", stepID)
			return
		}
	}
}

// waitForDependencies waits for dependent steps to complete
func (o *Orchestrator) waitForDependencies(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	for _, depID := range step.DependsOn {
		// Find dependent step in current workflow state
		var depStep *WorkflowStep
		for i := range workflow.Steps {
			if workflow.Steps[i].ID == depID {
				depStep = &workflow.Steps[i]
				break
			}
		}

		if depStep == nil {
			return fmt.Errorf("dependent step %s not found", depID)
		}

		// Check if already completed or failed
		if depStep.Status == StepStatusCompleted {
			continue
		}
		
		if depStep.Status == StepStatusFailed {
			return fmt.Errorf("dependent step %s failed", depID)
		}
		
		if depStep.Status == StepStatusSkipped {
			continue
		}
		
		// For pending or running steps, wait for completion
		maxWait := 30 * time.Minute
		waitStart := time.Now()
		checkInterval := 2 * time.Second
		
		for depStep.Status != StepStatusCompleted && depStep.Status != StepStatusFailed && depStep.Status != StepStatusSkipped {
			if time.Since(waitStart) > maxWait {
				return fmt.Errorf("timeout waiting for dependent step %s", depID)
			}
			
			time.Sleep(checkInterval)
			
			// Reload workflow to get updated step status
			updatedWorkflow, err := o.workflowManager.GetWorkflow(ctx, workflow.ID)
			if err != nil {
				return fmt.Errorf("failed to check dependency status: %w", err)
			}
			
			// Find the dependent step in updated workflow
			for i := range updatedWorkflow.Steps {
				if updatedWorkflow.Steps[i].ID == depID {
					depStep = &updatedWorkflow.Steps[i]
					// Also update in the current workflow
					for j := range workflow.Steps {
						if workflow.Steps[j].ID == depID {
							workflow.Steps[j].Status = depStep.Status
							workflow.Steps[j].CompletedAt = depStep.CompletedAt
							break
						}
					}
					break
				}
			}
			
			if depStep.Status == StepStatusFailed {
				return fmt.Errorf("dependent step %s failed", depID)
			}
		}
	}

	return nil
}

// Helper functions for map step

// extractMapInput extracts data from workflow state using a simple path
func (o *Orchestrator) extractMapInput(state map[string]interface{}, inputPath string) (interface{}, error) {
	// Simple implementation - just support direct key access for now
	// inputPath like "urls" or "items"
	if inputPath == "" {
		return nil, fmt.Errorf("input_path is required for map step")
	}
	
	// Remove leading $. if present
	if len(inputPath) > 2 && inputPath[:2] == "$." {
		inputPath = inputPath[2:]
	}
	
	data, exists := state[inputPath]
	if !exists {
		return nil, fmt.Errorf("key '%s' not found in workflow state", inputPath)
	}
	
	return data, nil
}

// updateMapItemState updates the state for a single map item
func (o *Orchestrator) updateMapItemState(ctx context.Context, workflowID, stepID string, itemIndex int, result map[string]interface{}, err error) {
	mapState, _ := o.stateManager.GetMapState(ctx, workflowID, stepID)
	if mapState == nil {
		return
	}
	
	if err != nil {
		mapState.FailedItems++
	} else {
		mapState.CompletedItems++
		if itemIndex < len(mapState.Results) {
			mapState.Results[itemIndex] = result
		}
	}
	
	o.stateManager.SetMapState(ctx, workflowID, stepID, mapState)
}

// timePtr returns a pointer to a time
func timePtr(t time.Time) *time.Time {
	return &t
}

// mustMarshal marshals data to JSON, panicking on error
func mustMarshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return string(data)
}