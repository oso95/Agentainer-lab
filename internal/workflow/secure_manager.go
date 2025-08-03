package workflow

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// SecureManager wraps the workflow manager with security checks
type SecureManager struct {
	manager         *Manager
	securityManager *SecurityManager
}

// NewSecureManager creates a new secure workflow manager
func NewSecureManager(redisClient *redis.Client) (*SecureManager, error) {
	securityManager, err := NewSecurityManager(redisClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create security manager: %w", err)
	}

	return &SecureManager{
		manager:         NewManager(redisClient),
		securityManager: securityManager,
	}, nil
}

// CreateWorkflow creates a new workflow with security checks
func (sm *SecureManager) CreateWorkflow(ctx context.Context, secCtx *SecurityContext, workflow *Workflow) error {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowCreate) {
		return fmt.Errorf("access denied: missing workflow:create permission")
	}

	// Set tenant ID
	if workflow.Metadata == nil {
		workflow.Metadata = make(map[string]string)
	}
	workflow.Metadata["tenant_id"] = secCtx.TenantID
	workflow.Metadata["created_by"] = secCtx.UserID

	// Validate against tenant config
	tenant, err := sm.securityManager.GetTenant(ctx, secCtx.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Check workflow limits
	existingCount, err := sm.countTenantWorkflows(ctx, secCtx.TenantID)
	if err != nil {
		return err
	}
	if existingCount >= tenant.Config.MaxWorkflows {
		return fmt.Errorf("workflow limit exceeded: %d/%d", existingCount, tenant.Config.MaxWorkflows)
	}

	// Validate images
	if err := sm.validateWorkflowImages(workflow, tenant.Config); err != nil {
		return err
	}

	// Save workflow
	if err := sm.manager.SaveWorkflow(ctx, workflow); err != nil {
		return err
	}

	// Audit log
	sm.securityManager.logAudit(ctx, &AuditLog{
		TenantID:   secCtx.TenantID,
		UserID:     secCtx.UserID,
		Action:     "workflow.create",
		Resource:   "workflow",
		ResourceID: workflow.ID,
		Result:     "success",
		Details: map[string]interface{}{
			"workflow_name": workflow.Name,
		},
	})

	return nil
}

// GetWorkflow retrieves a workflow with security checks
func (sm *SecureManager) GetWorkflow(ctx context.Context, secCtx *SecurityContext, workflowID string) (*Workflow, error) {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowRead) {
		return nil, fmt.Errorf("access denied: missing workflow:read permission")
	}

	// Get workflow
	workflow, err := sm.manager.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	// Check tenant isolation
	workflowTenantID := workflow.Metadata["tenant_id"]
	if err := sm.securityManager.EnforceTenantIsolation(ctx, secCtx, workflowTenantID); err != nil {
		return nil, err
	}

	return workflow, nil
}

// UpdateWorkflow updates a workflow with security checks
func (sm *SecureManager) UpdateWorkflow(ctx context.Context, secCtx *SecurityContext, workflow *Workflow) error {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowUpdate) {
		return fmt.Errorf("access denied: missing workflow:update permission")
	}

	// Get existing workflow to check tenant
	existing, err := sm.manager.GetWorkflow(ctx, workflow.ID)
	if err != nil {
		return err
	}

	// Check tenant isolation
	workflowTenantID := existing.Metadata["tenant_id"]
	if err := sm.securityManager.EnforceTenantIsolation(ctx, secCtx, workflowTenantID); err != nil {
		return err
	}

	// Preserve security metadata
	workflow.Metadata["tenant_id"] = workflowTenantID
	workflow.Metadata["updated_by"] = secCtx.UserID

	// Validate against tenant config
	tenant, err := sm.securityManager.GetTenant(ctx, secCtx.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Validate images
	if err := sm.validateWorkflowImages(workflow, tenant.Config); err != nil {
		return err
	}

	// Update workflow
	if err := sm.manager.SaveWorkflow(ctx, workflow); err != nil {
		return err
	}

	// Audit log
	sm.securityManager.logAudit(ctx, &AuditLog{
		TenantID:   secCtx.TenantID,
		UserID:     secCtx.UserID,
		Action:     "workflow.update",
		Resource:   "workflow",
		ResourceID: workflow.ID,
		Result:     "success",
	})

	return nil
}

// DeleteWorkflow deletes a workflow with security checks
func (sm *SecureManager) DeleteWorkflow(ctx context.Context, secCtx *SecurityContext, workflowID string) error {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowDelete) {
		return fmt.Errorf("access denied: missing workflow:delete permission")
	}

	// Get workflow to check tenant
	workflow, err := sm.manager.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	// Check tenant isolation
	workflowTenantID := workflow.Metadata["tenant_id"]
	if err := sm.securityManager.EnforceTenantIsolation(ctx, secCtx, workflowTenantID); err != nil {
		return err
	}

	// Delete workflow from Redis
	key := fmt.Sprintf("workflow:%s", workflowID)
	if err := sm.manager.redisClient.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	
	// Remove from workflow index
	if err := sm.manager.redisClient.SRem(ctx, "workflows", workflowID).Err(); err != nil {
		return fmt.Errorf("failed to remove from index: %w", err)
	}

	// Audit log
	sm.securityManager.logAudit(ctx, &AuditLog{
		TenantID:   secCtx.TenantID,
		UserID:     secCtx.UserID,
		Action:     "workflow.delete",
		Resource:   "workflow",
		ResourceID: workflowID,
		Result:     "success",
		Details: map[string]interface{}{
			"workflow_name": workflow.Name,
		},
	})

	return nil
}

// ExecuteWorkflow executes a workflow with security checks
func (sm *SecureManager) ExecuteWorkflow(ctx context.Context, secCtx *SecurityContext, workflowID string, orchestrator *Orchestrator) error {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowExecute) {
		return fmt.Errorf("access denied: missing workflow:execute permission")
	}

	// Get workflow to check tenant
	workflow, err := sm.manager.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	// Check tenant isolation
	workflowTenantID := workflow.Metadata["tenant_id"]
	if err := sm.securityManager.EnforceTenantIsolation(ctx, secCtx, workflowTenantID); err != nil {
		return err
	}

	// Get tenant config
	tenant, err := sm.securityManager.GetTenant(ctx, secCtx.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Apply tenant resource limits
	if workflow.Config.MaxParallel > tenant.Config.MaxParallelSteps {
		workflow.Config.MaxParallel = tenant.Config.MaxParallelSteps
	}

	// Execute workflow
	if err := orchestrator.ExecuteWorkflow(ctx, workflowID); err != nil {
		// Audit log for failure
		sm.securityManager.logAudit(ctx, &AuditLog{
			TenantID:   secCtx.TenantID,
			UserID:     secCtx.UserID,
			Action:     "workflow.execute",
			Resource:   "workflow",
			ResourceID: workflowID,
			Result:     "failure",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		})
		return err
	}

	// Audit log for success
	sm.securityManager.logAudit(ctx, &AuditLog{
		TenantID:   secCtx.TenantID,
		UserID:     secCtx.UserID,
		Action:     "workflow.execute",
		Resource:   "workflow",
		ResourceID: workflowID,
		Result:     "success",
		Details: map[string]interface{}{
			"workflow_name": workflow.Name,
		},
	})

	return nil
}

// ListWorkflows lists workflows with security checks
func (sm *SecureManager) ListWorkflows(ctx context.Context, secCtx *SecurityContext, status WorkflowStatus) ([]*Workflow, error) {
	// Check permission
	if !sm.securityManager.CheckPermission(ctx, secCtx, PermissionWorkflowRead) {
		return nil, fmt.Errorf("access denied: missing workflow:read permission")
	}

	// Get all workflows
	workflows, err := sm.manager.ListWorkflows(ctx, status)
	if err != nil {
		return nil, err
	}

	// Filter by tenant
	tenantWorkflows := []*Workflow{}
	for i := range workflows {
		wf := &workflows[i]
		workflowTenantID := wf.Metadata["tenant_id"]
		if workflowTenantID == secCtx.TenantID {
			tenantWorkflows = append(tenantWorkflows, wf)
		}
	}

	return tenantWorkflows, nil
}

// Helper methods

func (sm *SecureManager) countTenantWorkflows(ctx context.Context, tenantID string) (int, error) {
	workflows, err := sm.manager.ListWorkflows(ctx, "")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, workflow := range workflows {
		if workflow.Metadata["tenant_id"] == tenantID {
			count++
		}
	}

	return count, nil
}

func (sm *SecureManager) validateWorkflowImages(workflow *Workflow, tenantConfig TenantConfig) error {
	// Collect all images
	images := []string{}
	for _, step := range workflow.Steps {
		if step.Config.Image != "" {
			images = append(images, step.Config.Image)
		}
	}

	// Check allowed images
	if len(tenantConfig.AllowedImages) > 0 {
		for _, image := range images {
			allowed := false
			for _, allowedImage := range tenantConfig.AllowedImages {
				if image == allowedImage || matchesImagePattern(image, allowedImage) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("image %s is not in allowed list", image)
			}
		}
	}

	// Check forbidden images
	for _, image := range images {
		for _, forbiddenImage := range tenantConfig.ForbiddenImages {
			if image == forbiddenImage || matchesImagePattern(image, forbiddenImage) {
				return fmt.Errorf("image %s is forbidden", image)
			}
		}
	}

	return nil
}

func matchesImagePattern(image, pattern string) bool {
	// Simple wildcard matching for image patterns
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(image) >= len(prefix) && image[:len(prefix)] == prefix
	}
	return image == pattern
}