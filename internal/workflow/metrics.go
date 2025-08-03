package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// MetricType defines the type of metric
type MetricType string

const (
	MetricTypeWorkflowStart    MetricType = "workflow_start"
	MetricTypeWorkflowComplete MetricType = "workflow_complete"
	MetricTypeWorkflowFailed   MetricType = "workflow_failed"
	MetricTypeStepStart        MetricType = "step_start"
	MetricTypeStepComplete     MetricType = "step_complete"
	MetricTypeStepFailed       MetricType = "step_failed"
	MetricTypeAgentPooled      MetricType = "agent_pooled"
	MetricTypeAgentReused      MetricType = "agent_reused"
)

// WorkflowMetrics holds metrics for a workflow execution
type WorkflowMetrics struct {
	WorkflowID    string                 `json:"workflow_id"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      *time.Duration         `json:"duration,omitempty"`
	Status        WorkflowStatus         `json:"status"`
	StepMetrics   map[string]*StepMetrics `json:"step_metrics"`
	ResourceUsage ResourceUsage          `json:"resource_usage"`
	Errors        []string               `json:"errors,omitempty"`
	mu            sync.RWMutex
}

// StepMetrics holds metrics for a workflow step
type StepMetrics struct {
	StepID        string         `json:"step_id"`
	StepName      string         `json:"step_name"`
	StartTime     time.Time      `json:"start_time"`
	EndTime       *time.Time     `json:"end_time,omitempty"`
	Duration      *time.Duration `json:"duration,omitempty"`
	Status        StepStatus     `json:"status"`
	RetryCount    int            `json:"retry_count"`
	AgentID       string         `json:"agent_id,omitempty"`
	AgentPooled   bool           `json:"agent_pooled"`
	ResourceUsage ResourceUsage  `json:"resource_usage"`
	Error         string         `json:"error,omitempty"`
}

// ResourceUsage tracks resource consumption
type ResourceUsage struct {
	CPUMillicores    int64   `json:"cpu_millicores"`
	MemoryBytes      int64   `json:"memory_bytes"`
	NetworkBytesIn   int64   `json:"network_bytes_in"`
	NetworkBytesOut  int64   `json:"network_bytes_out"`
	AgentsDeployed   int     `json:"agents_deployed"`
	AgentsReused     int     `json:"agents_reused"`
	PoolUtilization  float64 `json:"pool_utilization"`
}

// MetricsCollector collects and stores workflow metrics
type MetricsCollector struct {
	redisClient *redis.Client
	metrics     sync.Map // map[workflowID]*WorkflowMetrics
	mu          sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(redisClient *redis.Client) *MetricsCollector {
	return &MetricsCollector{
		redisClient: redisClient,
	}
}

// RecordWorkflowStart records the start of a workflow
func (mc *MetricsCollector) RecordWorkflowStart(workflowID string, workflowName string) {
	metrics := &WorkflowMetrics{
		WorkflowID:  workflowID,
		StartTime:   time.Now(),
		Status:      WorkflowStatusRunning,
		StepMetrics: make(map[string]*StepMetrics),
	}
	
	mc.metrics.Store(workflowID, metrics)
	
	// Publish event
	mc.publishMetricEvent(MetricTypeWorkflowStart, map[string]interface{}{
		"workflow_id":   workflowID,
		"workflow_name": workflowName,
		"timestamp":     time.Now(),
	})
}

// RecordWorkflowComplete records workflow completion
func (mc *MetricsCollector) RecordWorkflowComplete(workflowID string, status WorkflowStatus) {
	metricsInterface, exists := mc.metrics.Load(workflowID)
	if !exists {
		return
	}
	
	metrics := metricsInterface.(*WorkflowMetrics)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	now := time.Now()
	metrics.EndTime = &now
	duration := now.Sub(metrics.StartTime)
	metrics.Duration = &duration
	metrics.Status = status
	
	// Calculate aggregate resource usage
	for _, stepMetrics := range metrics.StepMetrics {
		metrics.ResourceUsage.CPUMillicores += stepMetrics.ResourceUsage.CPUMillicores
		metrics.ResourceUsage.MemoryBytes += stepMetrics.ResourceUsage.MemoryBytes
		metrics.ResourceUsage.NetworkBytesIn += stepMetrics.ResourceUsage.NetworkBytesIn
		metrics.ResourceUsage.NetworkBytesOut += stepMetrics.ResourceUsage.NetworkBytesOut
		metrics.ResourceUsage.AgentsDeployed += stepMetrics.ResourceUsage.AgentsDeployed
		metrics.ResourceUsage.AgentsReused += stepMetrics.ResourceUsage.AgentsReused
	}
	
	// Save to Redis
	mc.saveMetrics(workflowID, metrics)
	
	// Publish event
	eventType := MetricTypeWorkflowComplete
	if status == WorkflowStatusFailed {
		eventType = MetricTypeWorkflowFailed
	}
	
	mc.publishMetricEvent(eventType, map[string]interface{}{
		"workflow_id": workflowID,
		"status":      status,
		"duration_ms": duration.Milliseconds(),
		"timestamp":   now,
	})
}

// RecordStepStart records the start of a workflow step
func (mc *MetricsCollector) RecordStepStart(workflowID, stepID, stepName string) {
	metricsInterface, exists := mc.metrics.Load(workflowID)
	if !exists {
		return
	}
	
	metrics := metricsInterface.(*WorkflowMetrics)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	stepMetrics := &StepMetrics{
		StepID:    stepID,
		StepName:  stepName,
		StartTime: time.Now(),
		Status:    StepStatusRunning,
	}
	
	metrics.StepMetrics[stepID] = stepMetrics
	
	// Publish event
	mc.publishMetricEvent(MetricTypeStepStart, map[string]interface{}{
		"workflow_id": workflowID,
		"step_id":     stepID,
		"step_name":   stepName,
		"timestamp":   time.Now(),
	})
}

// RecordStepComplete records step completion
func (mc *MetricsCollector) RecordStepComplete(workflowID, stepID string, status StepStatus, agentID string, pooled bool) {
	metricsInterface, exists := mc.metrics.Load(workflowID)
	if !exists {
		return
	}
	
	metrics := metricsInterface.(*WorkflowMetrics)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	stepMetrics, exists := metrics.StepMetrics[stepID]
	if !exists {
		return
	}
	
	now := time.Now()
	stepMetrics.EndTime = &now
	duration := now.Sub(stepMetrics.StartTime)
	stepMetrics.Duration = &duration
	stepMetrics.Status = status
	stepMetrics.AgentID = agentID
	stepMetrics.AgentPooled = pooled
	
	if pooled {
		stepMetrics.ResourceUsage.AgentsReused = 1
	} else {
		stepMetrics.ResourceUsage.AgentsDeployed = 1
	}
	
	// Publish event
	eventType := MetricTypeStepComplete
	if status == StepStatusFailed {
		eventType = MetricTypeStepFailed
	}
	
	mc.publishMetricEvent(eventType, map[string]interface{}{
		"workflow_id": workflowID,
		"step_id":     stepID,
		"status":      status,
		"duration_ms": duration.Milliseconds(),
		"agent_id":    agentID,
		"pooled":      pooled,
		"timestamp":   now,
	})
}

// RecordStepError records a step error
func (mc *MetricsCollector) RecordStepError(workflowID, stepID string, err error) {
	metricsInterface, exists := mc.metrics.Load(workflowID)
	if !exists {
		return
	}
	
	metrics := metricsInterface.(*WorkflowMetrics)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	if stepMetrics, exists := metrics.StepMetrics[stepID]; exists {
		stepMetrics.Error = err.Error()
		stepMetrics.RetryCount++
	}
	
	metrics.Errors = append(metrics.Errors, fmt.Sprintf("Step %s: %v", stepID, err))
}

// RecordResourceUsage updates resource usage for a step
func (mc *MetricsCollector) RecordResourceUsage(workflowID, stepID string, usage ResourceUsage) {
	metricsInterface, exists := mc.metrics.Load(workflowID)
	if !exists {
		return
	}
	
	metrics := metricsInterface.(*WorkflowMetrics)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	if stepMetrics, exists := metrics.StepMetrics[stepID]; exists {
		stepMetrics.ResourceUsage = usage
	}
}

// GetWorkflowMetrics retrieves metrics for a workflow
func (mc *MetricsCollector) GetWorkflowMetrics(workflowID string) (*WorkflowMetrics, error) {
	// Try memory first
	if metricsInterface, exists := mc.metrics.Load(workflowID); exists {
		return metricsInterface.(*WorkflowMetrics), nil
	}
	
	// Try Redis
	ctx := context.Background()
	key := fmt.Sprintf("metrics:workflow:%s", workflowID)
	data, err := mc.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("metrics not found: %w", err)
	}
	
	var metrics WorkflowMetrics
	if err := json.Unmarshal([]byte(data), &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	
	return &metrics, nil
}

// GetWorkflowHistory retrieves historical metrics for workflows
func (mc *MetricsCollector) GetWorkflowHistory(ctx context.Context, duration time.Duration) ([]*WorkflowMetrics, error) {
	// Get workflow IDs from the last duration
	now := time.Now()
	start := now.Add(-duration)
	
	// Use Redis sorted set to get workflows by timestamp
	workflowIDs, err := mc.redisClient.ZRangeByScore(ctx, "metrics:workflows:timeline", &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", start.Unix()),
		Max: fmt.Sprintf("%d", now.Unix()),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow timeline: %w", err)
	}
	
	var metrics []*WorkflowMetrics
	for _, id := range workflowIDs {
		if m, err := mc.GetWorkflowMetrics(id); err == nil {
			metrics = append(metrics, m)
		}
	}
	
	return metrics, nil
}

// GetAggregateMetrics returns aggregate metrics for a time period
func (mc *MetricsCollector) GetAggregateMetrics(ctx context.Context, duration time.Duration) (map[string]interface{}, error) {
	workflows, err := mc.GetWorkflowHistory(ctx, duration)
	if err != nil {
		return nil, err
	}
	
	// Calculate aggregates
	totalWorkflows := len(workflows)
	completedWorkflows := 0
	failedWorkflows := 0
	totalSteps := 0
	failedSteps := 0
	totalDuration := time.Duration(0)
	totalCPU := int64(0)
	totalMemory := int64(0)
	totalAgentsDeployed := 0
	totalAgentsReused := 0
	
	for _, wf := range workflows {
		if wf.Status == WorkflowStatusCompleted {
			completedWorkflows++
		} else if wf.Status == WorkflowStatusFailed {
			failedWorkflows++
		}
		
		if wf.Duration != nil {
			totalDuration += *wf.Duration
		}
		
		totalCPU += wf.ResourceUsage.CPUMillicores
		totalMemory += wf.ResourceUsage.MemoryBytes
		totalAgentsDeployed += wf.ResourceUsage.AgentsDeployed
		totalAgentsReused += wf.ResourceUsage.AgentsReused
		
		for _, step := range wf.StepMetrics {
			totalSteps++
			if step.Status == StepStatusFailed {
				failedSteps++
			}
		}
	}
	
	avgDuration := time.Duration(0)
	if totalWorkflows > 0 {
		avgDuration = totalDuration / time.Duration(totalWorkflows)
	}
	
	poolEfficiency := float64(0)
	if totalAgentsDeployed+totalAgentsReused > 0 {
		poolEfficiency = float64(totalAgentsReused) / float64(totalAgentsDeployed+totalAgentsReused) * 100
	}
	
	return map[string]interface{}{
		"period":               duration.String(),
		"total_workflows":      totalWorkflows,
		"completed_workflows":  completedWorkflows,
		"failed_workflows":     failedWorkflows,
		"success_rate":         float64(completedWorkflows) / float64(totalWorkflows) * 100,
		"total_steps":          totalSteps,
		"failed_steps":         failedSteps,
		"avg_duration":         avgDuration.String(),
		"total_cpu_millicores": totalCPU,
		"total_memory_mb":      totalMemory / 1024 / 1024,
		"agents_deployed":      totalAgentsDeployed,
		"agents_reused":        totalAgentsReused,
		"pool_efficiency":      poolEfficiency,
	}, nil
}

// saveMetrics saves metrics to Redis
func (mc *MetricsCollector) saveMetrics(workflowID string, metrics *WorkflowMetrics) error {
	ctx := context.Background()
	
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}
	
	// Save metrics
	key := fmt.Sprintf("metrics:workflow:%s", workflowID)
	if err := mc.redisClient.Set(ctx, key, data, 7*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}
	
	// Add to timeline
	if err := mc.redisClient.ZAdd(ctx, "metrics:workflows:timeline", &redis.Z{
		Score:  float64(metrics.StartTime.Unix()),
		Member: workflowID,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to timeline: %w", err)
	}
	
	return nil
}

// publishMetricEvent publishes a metric event
func (mc *MetricsCollector) publishMetricEvent(metricType MetricType, data map[string]interface{}) {
	ctx := context.Background()
	
	event := map[string]interface{}{
		"type":      string(metricType),
		"timestamp": time.Now(),
		"data":      data,
	}
	
	if eventData, err := json.Marshal(event); err == nil {
		mc.redisClient.Publish(ctx, "metrics:events", eventData)
	}
}

// CleanupOldMetrics removes metrics older than the retention period
func (mc *MetricsCollector) CleanupOldMetrics(ctx context.Context, retention time.Duration) error {
	// Remove old entries from timeline
	cutoff := time.Now().Add(-retention)
	if err := mc.redisClient.ZRemRangeByScore(ctx, "metrics:workflows:timeline", 
		"-inf", fmt.Sprintf("%d", cutoff.Unix())).Err(); err != nil {
		return fmt.Errorf("failed to cleanup timeline: %w", err)
	}
	
	// TODO: Also cleanup individual metric keys
	
	return nil
}