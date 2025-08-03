package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agentainer/agentainer-lab/internal/logging"
	"github.com/agentainer/agentainer-lab/internal/workflow"
	"github.com/gorilla/mux"
)

// CreateWorkflowRequest represents a request to create a new workflow
type CreateWorkflowRequest struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Config      workflow.WorkflowConfig `json:"config"`
	Steps       []workflow.WorkflowStep `json:"steps,omitempty"`
}

// AddStepRequest represents a request to add a step to a workflow
type AddStepRequest struct {
	Name      string                `json:"name"`
	Type      workflow.StepType     `json:"type"`
	Config    workflow.StepConfig   `json:"config"`
	DependsOn []string              `json:"depends_on,omitempty"`
}

// UpdateStateRequest represents a request to update workflow state
type UpdateStateRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// createWorkflowHandler creates a new workflow
func (s *Server) createWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	logging.Info("api", "Create workflow request received", map[string]interface{}{
		"name":       req.Name,
		"step_count": len(req.Steps),
	})

	// Validate input
	if req.Name == "" {
		s.sendError(w, http.StatusBadRequest, "Workflow name is required")
		return
	}

	// Create workflow
	wf, err := s.workflowMgr.CreateWorkflow(r.Context(), req.Name, req.Description, req.Config)
	if err != nil {
		logging.Error("api", "Failed to create workflow", map[string]interface{}{
			"name":  req.Name,
			"error": err.Error(),
		})
		s.sendError(w, http.StatusInternalServerError, "Failed to create workflow")
		return
	}

	// Add steps if provided
	if len(req.Steps) > 0 {
		logging.Info("api", "Adding steps to workflow", map[string]interface{}{
			"workflow_id": wf.ID,
			"step_count":  len(req.Steps),
		})
		wf.Steps = req.Steps
		// Initialize step statuses
		for i := range wf.Steps {
			if wf.Steps[i].Status == "" {
				wf.Steps[i].Status = workflow.StepStatusPending
			}
			// Generate ID if not provided
			if wf.Steps[i].ID == "" {
				wf.Steps[i].ID = generateStepID()
			}
		}
		// Save workflow with steps
		if err := s.workflowMgr.SaveWorkflow(r.Context(), wf); err != nil {
			logging.Error("api", "Failed to save workflow with steps", map[string]interface{}{
				"workflow_id": wf.ID,
				"error":       err.Error(),
			})
			s.sendError(w, http.StatusInternalServerError, "Failed to create workflow")
			return
		}
		logging.Info("api", "Workflow saved with steps", map[string]interface{}{
			"workflow_id": wf.ID,
			"steps":       len(wf.Steps),
		})
		
		// Refresh workflow to get latest state
		wf, err = s.workflowMgr.GetWorkflow(r.Context(), wf.ID)
		if err != nil {
			logging.Error("api", "Failed to retrieve workflow after save", map[string]interface{}{
				"workflow_id": wf.ID,
				"error":       err.Error(),
			})
		}
	} else {
		logging.Info("api", "No steps provided in request", map[string]interface{}{
			"workflow_id": wf.ID,
		})
	}

	// Audit log
	logging.AuditLog(logging.AuditEntry{
		UserID:     s.getUserID(r),
		Action:     "create_workflow",
		Resource:   "workflow",
		ResourceID: wf.ID,
		Result:     "success",
		Details: map[string]interface{}{
			"name": req.Name,
		},
	})

	s.sendResponse(w, http.StatusCreated, Response{
		Success: true,
		Message: "Workflow created successfully",
		Data:    wf,
	})
}

// listWorkflowsHandler lists all workflows
func (s *Server) listWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	// Get optional status filter
	status := workflow.WorkflowStatus(r.URL.Query().Get("status"))

	workflows, err := s.workflowMgr.ListWorkflows(r.Context(), status)
	if err != nil {
		logging.Error("api", "Failed to list workflows", map[string]interface{}{
			"error": err.Error(),
		})
		s.sendError(w, http.StatusInternalServerError, "Failed to list workflows")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Workflows retrieved successfully",
		Data:    workflows,
	})
}

// getWorkflowHandler retrieves a workflow by ID
func (s *Server) getWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	wf, err := s.workflowMgr.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, http.StatusNotFound, "Workflow not found")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "Failed to get workflow")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Workflow retrieved successfully",
		Data:    wf,
	})
}

// getWorkflowJobsHandler returns all jobs for a workflow
func (s *Server) getWorkflowJobsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	// Get job IDs
	jobIDs, err := s.workflowMgr.GetWorkflowJobs(r.Context(), workflowID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to get workflow jobs")
		return
	}

	// Get full agent details for each job
	var agents []interface{}
	for _, jobID := range jobIDs {
		agent, err := s.agentMgr.GetAgent(jobID)
		if err != nil {
			continue // Skip invalid agents
		}
		agents = append(agents, agent)
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Workflow jobs retrieved successfully",
		Data:    agents,
	})
}

// startWorkflowHandler starts workflow execution
func (s *Server) startWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	logging.Info("api", "Starting workflow execution", map[string]interface{}{
		"workflow_id": workflowID,
	})

	// Check if orchestrator is available
	if s.orchestrator == nil {
		logging.Error("api", "Orchestrator is nil", map[string]interface{}{
			"workflow_id": workflowID,
		})
		s.sendError(w, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Start workflow execution in background
	go func() {
		// Use background context for async execution
		ctx := context.Background()
		logging.Info("api", "Calling orchestrator.ExecuteWorkflow", map[string]interface{}{
			"workflow_id": workflowID,
		})
		if err := s.orchestrator.ExecuteWorkflow(ctx, workflowID); err != nil {
			logging.Error("api", "Failed to execute workflow", map[string]interface{}{
				"workflow_id": workflowID,
				"error":       err.Error(),
			})
		} else {
			logging.Info("api", "Workflow execution completed", map[string]interface{}{
				"workflow_id": workflowID,
			})
		}
	}()

	// Audit log
	logging.AuditLog(logging.AuditEntry{
		UserID:     s.getUserID(r),
		Action:     "start_workflow",
		Resource:   "workflow",
		ResourceID: workflowID,
		Result:     "success",
	})

	s.sendResponse(w, http.StatusAccepted, Response{
		Success: true,
		Message: "Workflow execution started",
		Data: map[string]string{
			"workflow_id": workflowID,
			"status":      "started",
		},
	})
}

// updateWorkflowStateHandler updates workflow state
func (s *Server) updateWorkflowStateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	var req UpdateStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Key == "" {
		s.sendError(w, http.StatusBadRequest, "State key is required")
		return
	}

	// Update state
	if err := s.workflowMgr.UpdateWorkflowState(r.Context(), workflowID, req.Key, req.Value); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to update state")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "State updated successfully",
	})
}

// getWorkflowStateHandler retrieves workflow state
func (s *Server) getWorkflowStateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]
	key := r.URL.Query().Get("key")

	if key != "" {
		// Get specific key
		value, err := s.workflowMgr.GetWorkflowState(r.Context(), workflowID, key)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.sendError(w, http.StatusNotFound, "State key not found")
				return
			}
			s.sendError(w, http.StatusInternalServerError, "Failed to get state")
			return
		}

		s.sendResponse(w, http.StatusOK, Response{
			Success: true,
			Message: "State retrieved successfully",
			Data: map[string]interface{}{
				key: value,
			},
		})
	} else {
		// Get all state
		wf, err := s.workflowMgr.GetWorkflow(r.Context(), workflowID)
		if err != nil {
			s.sendError(w, http.StatusInternalServerError, "Failed to get workflow")
			return
		}

		s.sendResponse(w, http.StatusOK, Response{
			Success: true,
			Message: "State retrieved successfully",
			Data:    wf.State,
		})
	}
}

// addWorkflowStepHandler adds a step to a workflow
func (s *Server) addWorkflowStepHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	var req AddStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Name == "" || req.Type == "" {
		s.sendError(w, http.StatusBadRequest, "Step name and type are required")
		return
	}

	// Get workflow
	wf, err := s.workflowMgr.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Workflow not found")
		return
	}

	// Add step
	step := workflow.WorkflowStep{
		ID:        generateStepID(),
		Name:      req.Name,
		Type:      req.Type,
		Status:    workflow.StepStatusPending,
		Config:    req.Config,
		DependsOn: req.DependsOn,
	}

	wf.Steps = append(wf.Steps, step)
	
	// Save workflow
	if err := s.workflowMgr.SaveWorkflow(r.Context(), wf); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to add step")
		return
	}

	s.sendResponse(w, http.StatusCreated, Response{
		Success: true,
		Message: "Step added successfully",
		Data:    step,
	})
}

// Helper function to generate step IDs
func generateStepID() string {
	return fmt.Sprintf("step-%d", time.Now().UnixNano())
}

// Trigger handlers

// CreateTriggerRequest represents a request to create a workflow trigger
type CreateTriggerRequest struct {
	Type   workflow.TriggerType   `json:"type"`
	Config workflow.TriggerConfig `json:"config"`
}

// createWorkflowTriggerHandler creates a trigger for a workflow
func (s *Server) createWorkflowTriggerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	var req CreateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Create trigger
	trigger, err := s.scheduler.CreateTrigger(r.Context(), workflowID, req.Type, req.Config)
	if err != nil {
		logging.Error("api", "Failed to create trigger", map[string]interface{}{
			"workflow_id": workflowID,
			"type":        req.Type,
			"error":       err.Error(),
		})
		s.sendError(w, http.StatusInternalServerError, "Failed to create trigger")
		return
	}

	// Audit log
	logging.AuditLog(logging.AuditEntry{
		UserID:     s.getUserID(r),
		Action:     "create_trigger",
		Resource:   "workflow_trigger",
		ResourceID: trigger.ID,
		Result:     "success",
		Details: map[string]interface{}{
			"workflow_id": workflowID,
			"type":        req.Type,
		},
	})

	s.sendResponse(w, http.StatusCreated, Response{
		Success: true,
		Message: "Trigger created successfully",
		Data:    trigger,
	})
}

// listWorkflowTriggersHandler lists all triggers for a workflow
func (s *Server) listWorkflowTriggersHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	triggers, err := s.scheduler.ListTriggers(r.Context(), workflowID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to list triggers")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Triggers retrieved successfully",
		Data:    triggers,
	})
}

// enableTriggerHandler enables a trigger
func (s *Server) enableTriggerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	triggerID := vars["triggerId"]

	if err := s.scheduler.EnableTrigger(r.Context(), triggerID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, http.StatusNotFound, "Trigger not found")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "Failed to enable trigger")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Trigger enabled successfully",
	})
}

// disableTriggerHandler disables a trigger
func (s *Server) disableTriggerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	triggerID := vars["triggerId"]

	if err := s.scheduler.DisableTrigger(r.Context(), triggerID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, http.StatusNotFound, "Trigger not found")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "Failed to disable trigger")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Trigger disabled successfully",
	})
}

// executeTriggerHandler manually executes a trigger
func (s *Server) executeTriggerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	// Create a manual trigger execution
	trigger, err := s.scheduler.CreateTrigger(
		r.Context(),
		workflowID,
		workflow.TriggerTypeManual,
		workflow.TriggerConfig{
			InputData: map[string]interface{}{
				"triggered_by": s.getUserID(r),
				"triggered_at": time.Now().Format(time.RFC3339),
			},
		},
	)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to execute workflow")
		return
	}

	// Execute immediately
	go s.scheduler.ExecuteTrigger(r.Context(), trigger.ID)

	s.sendResponse(w, http.StatusAccepted, Response{
		Success: true,
		Message: "Workflow execution triggered",
		Data: map[string]string{
			"trigger_id": trigger.ID,
		},
	})
}

// MapReduce pattern handlers

// MapReduceRequest represents a request to create a MapReduce workflow
type MapReduceRequest struct {
	workflow.MapReduceConfig
}

// createMapReduceHandler creates a MapReduce workflow with simplified API
func (s *Server) createMapReduceHandler(w http.ResponseWriter, r *http.Request) {
	var req MapReduceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Name == "" || req.MapperImage == "" || req.ReducerImage == "" {
		s.sendError(w, http.StatusBadRequest, "Name, mapper_image, and reducer_image are required")
		return
	}

	// Execute MapReduce
	wf, err := s.orchestrator.ExecuteMapReduce(r.Context(), req.MapReduceConfig)
	if err != nil {
		logging.Error("api", "Failed to create MapReduce workflow", map[string]interface{}{
			"name":  req.Name,
			"error": err.Error(),
		})
		s.sendError(w, http.StatusInternalServerError, "Failed to create MapReduce workflow")
		return
	}

	// Audit log
	logging.AuditLog(logging.AuditEntry{
		UserID:     s.getUserID(r),
		Action:     "create_mapreduce",
		Resource:   "workflow",
		ResourceID: wf.ID,
		Result:     "success",
		Details: map[string]interface{}{
			"name":    req.Name,
			"pattern": "mapreduce",
		},
	})

	s.sendResponse(w, http.StatusCreated, Response{
		Success: true,
		Message: "MapReduce workflow created and started",
		Data:    wf,
	})
}

// Metrics handlers

// getWorkflowMetricsHandler retrieves metrics for a specific workflow
func (s *Server) getWorkflowMetricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	metrics, err := s.workflowMetrics.GetWorkflowMetrics(workflowID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, http.StatusNotFound, "Workflow metrics not found")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "Failed to get workflow metrics")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Workflow metrics retrieved successfully",
		Data:    metrics,
	})
}

// getWorkflowHistoryHandler retrieves historical workflow metrics
func (s *Server) getWorkflowHistoryHandler(w http.ResponseWriter, r *http.Request) {
	// Parse duration parameter (default: 1 hour)
	durationStr := r.URL.Query().Get("duration")
	duration := 1 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	// Limit to 7 days max
	if duration > 7*24*time.Hour {
		duration = 7 * 24 * time.Hour
	}

	metrics, err := s.workflowMetrics.GetWorkflowHistory(r.Context(), duration)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to get workflow history")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Workflow history retrieved successfully",
		Data: map[string]interface{}{
			"duration":  duration.String(),
			"workflows": metrics,
		},
	})
}

// getAggregateMetricsHandler retrieves aggregate metrics
func (s *Server) getAggregateMetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse duration parameter (default: 1 hour)
	durationStr := r.URL.Query().Get("duration")
	duration := 1 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	// Limit to 7 days max
	if duration > 7*24*time.Hour {
		duration = 7 * 24 * time.Hour
	}

	aggregates, err := s.workflowMetrics.GetAggregateMetrics(r.Context(), duration)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to get aggregate metrics")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Aggregate metrics retrieved successfully",
		Data:    aggregates,
	})
}