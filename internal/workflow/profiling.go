package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// PerformanceProfiler collects performance metrics and profiles
type PerformanceProfiler struct {
	redisClient      *redis.Client
	metricsCollector *MetricsCollector
	profiles         map[string]*WorkflowProfile
	mu               sync.RWMutex
}

// WorkflowProfile contains performance profiling data
type WorkflowProfile struct {
	WorkflowID    string                       `json:"workflow_id"`
	StartTime     time.Time                    `json:"start_time"`
	EndTime       *time.Time                   `json:"end_time,omitempty"`
	StepProfiles  map[string]*StepProfile      `json:"step_profiles"`
	ResourceUsage *ResourceProfile             `json:"resource_usage"`
	Bottlenecks   []PerformanceBottleneck      `json:"bottlenecks,omitempty"`
	Recommendations []string                   `json:"recommendations,omitempty"`
}

// StepProfile contains performance data for a single step
type StepProfile struct {
	StepID        string                 `json:"step_id"`
	StepName      string                 `json:"step_name"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      time.Duration          `json:"duration"`
	CPUTime       time.Duration          `json:"cpu_time"`
	WaitTime      time.Duration          `json:"wait_time"`
	IOTime        time.Duration          `json:"io_time"`
	MemoryUsage   MemoryProfile          `json:"memory_usage"`
	NetworkUsage  NetworkProfile         `json:"network_usage"`
	ContainerStats *ContainerProfile     `json:"container_stats,omitempty"`
}

// ResourceProfile tracks overall resource usage
type ResourceProfile struct {
	CPUUsage      CPUProfile       `json:"cpu_usage"`
	MemoryUsage   MemoryProfile    `json:"memory_usage"`
	DiskUsage     DiskProfile      `json:"disk_usage"`
	NetworkUsage  NetworkProfile   `json:"network_usage"`
	Samples       []ResourceSample `json:"samples"`
}

// CPUProfile contains CPU usage metrics
type CPUProfile struct {
	TotalCPUTime    time.Duration `json:"total_cpu_time"`
	UserCPUTime     time.Duration `json:"user_cpu_time"`
	SystemCPUTime   time.Duration `json:"system_cpu_time"`
	AvgCPUPercent   float64       `json:"avg_cpu_percent"`
	MaxCPUPercent   float64       `json:"max_cpu_percent"`
	CoreUtilization map[int]float64 `json:"core_utilization"`
}

// MemoryProfile contains memory usage metrics
type MemoryProfile struct {
	AvgMemoryMB   float64 `json:"avg_memory_mb"`
	MaxMemoryMB   float64 `json:"max_memory_mb"`
	MinMemoryMB   float64 `json:"min_memory_mb"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	HeapInUseMB   float64 `json:"heap_inuse_mb"`
	GCCount       uint32  `json:"gc_count"`
	GCPauseTimeMs float64 `json:"gc_pause_time_ms"`
}

// DiskProfile contains disk I/O metrics
type DiskProfile struct {
	ReadBytes     int64         `json:"read_bytes"`
	WriteBytes    int64         `json:"write_bytes"`
	ReadOps       int64         `json:"read_ops"`
	WriteOps      int64         `json:"write_ops"`
	AvgReadLatency time.Duration `json:"avg_read_latency"`
	AvgWriteLatency time.Duration `json:"avg_write_latency"`
}

// NetworkProfile contains network usage metrics
type NetworkProfile struct {
	BytesSent     int64         `json:"bytes_sent"`
	BytesReceived int64         `json:"bytes_received"`
	PacketsSent   int64         `json:"packets_sent"`
	PacketsReceived int64       `json:"packets_received"`
	AvgLatency    time.Duration `json:"avg_latency"`
	PacketLoss    float64       `json:"packet_loss"`
}

// ContainerProfile contains container-specific metrics
type ContainerProfile struct {
	ContainerID   string        `json:"container_id"`
	ImageName     string        `json:"image_name"`
	CPUShares     int           `json:"cpu_shares"`
	MemoryLimitMB int           `json:"memory_limit_mb"`
	RestartCount  int           `json:"restart_count"`
	ExitCode      int           `json:"exit_code"`
}

// ResourceSample is a point-in-time resource measurement
type ResourceSample struct {
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryMB      float64   `json:"memory_mb"`
	DiskReadBPS   int64     `json:"disk_read_bps"`
	DiskWriteBPS  int64     `json:"disk_write_bps"`
	NetworkInBPS  int64     `json:"network_in_bps"`
	NetworkOutBPS int64     `json:"network_out_bps"`
}

// PerformanceBottleneck identifies performance issues
type PerformanceBottleneck struct {
	Type        BottleneckType `json:"type"`
	Severity    string         `json:"severity"` // low, medium, high
	Component   string         `json:"component"`
	Description string         `json:"description"`
	Impact      string         `json:"impact"`
	StartTime   time.Time      `json:"start_time"`
	Duration    time.Duration  `json:"duration"`
}

// BottleneckType defines types of performance bottlenecks
type BottleneckType string

const (
	BottleneckCPU       BottleneckType = "cpu"
	BottleneckMemory    BottleneckType = "memory"
	BottleneckIO        BottleneckType = "io"
	BottleneckNetwork   BottleneckType = "network"
	BottleneckConcurrency BottleneckType = "concurrency"
	BottleneckWait      BottleneckType = "wait"
)

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler(redisClient *redis.Client, metricsCollector *MetricsCollector) *PerformanceProfiler {
	return &PerformanceProfiler{
		redisClient:      redisClient,
		metricsCollector: metricsCollector,
		profiles:         make(map[string]*WorkflowProfile),
	}
}

// StartProfiling begins profiling a workflow
func (pp *PerformanceProfiler) StartProfiling(workflowID string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	profile := &WorkflowProfile{
		WorkflowID:    workflowID,
		StartTime:     time.Now(),
		StepProfiles:  make(map[string]*StepProfile),
		ResourceUsage: &ResourceProfile{
			Samples: make([]ResourceSample, 0),
		},
	}

	pp.profiles[workflowID] = profile

	// Start resource monitoring goroutine
	go pp.monitorResources(workflowID)
}

// StopProfiling stops profiling a workflow and generates report
func (pp *PerformanceProfiler) StopProfiling(workflowID string) (*WorkflowProfile, error) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	profile, exists := pp.profiles[workflowID]
	if !exists {
		return nil, fmt.Errorf("no profile found for workflow %s", workflowID)
	}

	now := time.Now()
	profile.EndTime = &now

	// Analyze performance and identify bottlenecks
	pp.analyzePerformance(profile)

	// Generate recommendations
	pp.generateRecommendations(profile)

	// Save profile to Redis
	if err := pp.saveProfile(context.Background(), profile); err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}

	// Clean up
	delete(pp.profiles, workflowID)

	return profile, nil
}

// ProfileStep starts profiling a workflow step
func (pp *PerformanceProfiler) ProfileStep(workflowID, stepID, stepName string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	profile, exists := pp.profiles[workflowID]
	if !exists {
		return
	}

	stepProfile := &StepProfile{
		StepID:    stepID,
		StepName:  stepName,
		StartTime: time.Now(),
	}

	profile.StepProfiles[stepID] = stepProfile
}

// CompleteStepProfile completes profiling for a step
func (pp *PerformanceProfiler) CompleteStepProfile(workflowID, stepID string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	profile, exists := pp.profiles[workflowID]
	if !exists {
		return
	}

	stepProfile, exists := profile.StepProfiles[stepID]
	if !exists {
		return
	}

	now := time.Now()
	stepProfile.EndTime = &now
	stepProfile.Duration = now.Sub(stepProfile.StartTime)

	// Collect final metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stepProfile.MemoryUsage = MemoryProfile{
		HeapAllocMB: float64(memStats.HeapAlloc) / 1024 / 1024,
		HeapInUseMB: float64(memStats.HeapInuse) / 1024 / 1024,
		GCCount:     memStats.NumGC,
	}
}

// monitorResources continuously monitors resource usage
func (pp *PerformanceProfiler) monitorResources(workflowID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pp.mu.RLock()
		profile, exists := pp.profiles[workflowID]
		pp.mu.RUnlock()

		if !exists || profile.EndTime != nil {
			return
		}

		// Collect resource sample
		sample := pp.collectResourceSample()
		
		pp.mu.Lock()
		profile.ResourceUsage.Samples = append(profile.ResourceUsage.Samples, sample)
		pp.mu.Unlock()
	}
}

// collectResourceSample collects current resource usage
func (pp *PerformanceProfiler) collectResourceSample() ResourceSample {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return ResourceSample{
		Timestamp: time.Now(),
		MemoryMB:  float64(memStats.Alloc) / 1024 / 1024,
		// CPU and other metrics would be collected from system stats
		// This is simplified for the example
	}
}

// analyzePerformance analyzes performance data and identifies bottlenecks
func (pp *PerformanceProfiler) analyzePerformance(profile *WorkflowProfile) {
	bottlenecks := []PerformanceBottleneck{}

	// Analyze step durations
	var totalDuration time.Duration
	var longestStep string
	var longestDuration time.Duration

	for stepID, stepProfile := range profile.StepProfiles {
		if stepProfile.Duration > longestDuration {
			longestDuration = stepProfile.Duration
			longestStep = stepID
		}
		totalDuration += stepProfile.Duration
	}

	// Check for slow steps
	avgDuration := totalDuration / time.Duration(len(profile.StepProfiles))
	if longestDuration > avgDuration*3 {
		bottlenecks = append(bottlenecks, PerformanceBottleneck{
			Type:        BottleneckWait,
			Severity:    "high",
			Component:   longestStep,
			Description: fmt.Sprintf("Step %s takes significantly longer than average", longestStep),
			Impact:      fmt.Sprintf("Step duration: %v, Average: %v", longestDuration, avgDuration),
			StartTime:   profile.StepProfiles[longestStep].StartTime,
			Duration:    longestDuration,
		})
	}

	// Analyze memory usage
	if len(profile.ResourceUsage.Samples) > 0 {
		var maxMemory float64
		var totalMemory float64
		for _, sample := range profile.ResourceUsage.Samples {
			if sample.MemoryMB > maxMemory {
				maxMemory = sample.MemoryMB
			}
			totalMemory += sample.MemoryMB
		}
		avgMemory := totalMemory / float64(len(profile.ResourceUsage.Samples))

		if maxMemory > avgMemory*2 {
			bottlenecks = append(bottlenecks, PerformanceBottleneck{
				Type:        BottleneckMemory,
				Severity:    "medium",
				Component:   "workflow",
				Description: "Memory usage spikes detected",
				Impact:      fmt.Sprintf("Max memory: %.2f MB, Average: %.2f MB", maxMemory, avgMemory),
				StartTime:   profile.StartTime,
				Duration:    time.Since(profile.StartTime),
			})
		}
	}

	profile.Bottlenecks = bottlenecks
}

// generateRecommendations generates performance improvement recommendations
func (pp *PerformanceProfiler) generateRecommendations(profile *WorkflowProfile) {
	recommendations := []string{}

	// Check for parallelization opportunities
	sequentialSteps := 0
	for _, stepProfile := range profile.StepProfiles {
		if stepProfile.StepName != "" { // Check if step is sequential
			sequentialSteps++
		}
	}

	if sequentialSteps > 3 {
		recommendations = append(recommendations, 
			"Consider parallelizing independent steps to reduce overall execution time")
	}

	// Check for memory issues
	for _, bottleneck := range profile.Bottlenecks {
		if bottleneck.Type == BottleneckMemory {
			recommendations = append(recommendations,
				"Optimize memory usage by processing data in smaller batches",
				"Consider increasing memory limits for memory-intensive steps")
		}
		if bottleneck.Type == BottleneckWait {
			recommendations = append(recommendations,
				fmt.Sprintf("Investigate why step %s is taking longer than expected", bottleneck.Component),
				"Consider adding timeouts to prevent steps from running indefinitely")
		}
	}

	// Check for resource utilization
	if len(profile.ResourceUsage.Samples) > 10 {
		recommendations = append(recommendations,
			"Enable agent pooling to reuse containers and reduce startup overhead")
	}

	profile.Recommendations = recommendations
}

// saveProfile saves the performance profile to Redis
func (pp *PerformanceProfiler) saveProfile(ctx context.Context, profile *WorkflowProfile) error {
	key := fmt.Sprintf("profile:%s", profile.WorkflowID)
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}

	return pp.redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}

// GetProfile retrieves a saved performance profile
func (pp *PerformanceProfiler) GetProfile(ctx context.Context, workflowID string) (*WorkflowProfile, error) {
	key := fmt.Sprintf("profile:%s", workflowID)
	data, err := pp.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var profile WorkflowProfile
	if err := json.Unmarshal([]byte(data), &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// ExportProfile exports performance data in various formats
func (pp *PerformanceProfiler) ExportProfile(profile *WorkflowProfile, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(profile, "", "  ")
	case "pprof":
		// Export as pprof format for use with go tool pprof
		return pp.exportPprof(profile)
	case "csv":
		return pp.exportCSV(profile)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportPprof exports profile in pprof format
func (pp *PerformanceProfiler) exportPprof(profile *WorkflowProfile) ([]byte, error) {
	// This would create a pprof-compatible profile
	// Simplified for the example
	return nil, fmt.Errorf("pprof export not fully implemented")
}

// exportCSV exports profile data as CSV
func (pp *PerformanceProfiler) exportCSV(profile *WorkflowProfile) ([]byte, error) {
	csv := "timestamp,step_id,duration_ms,memory_mb,cpu_percent\n"
	
	for stepID, stepProfile := range profile.StepProfiles {
		csv += fmt.Sprintf("%s,%s,%d,%.2f,%.2f\n",
			stepProfile.StartTime.Format(time.RFC3339),
			stepID,
			stepProfile.Duration.Milliseconds(),
			stepProfile.MemoryUsage.AvgMemoryMB,
			0.0, // CPU percent would be calculated from actual metrics
		)
	}
	
	return []byte(csv), nil
}

// GenerateFlameGraph generates flame graph data for visualization
func (pp *PerformanceProfiler) GenerateFlameGraph(profile *WorkflowProfile) map[string]interface{} {
	// Generate flame graph data structure
	flameData := map[string]interface{}{
		"name":     profile.WorkflowID,
		"value":    profile.EndTime.Sub(profile.StartTime).Milliseconds(),
		"children": []map[string]interface{}{},
	}

	for stepID, stepProfile := range profile.StepProfiles {
		stepData := map[string]interface{}{
			"name":  stepProfile.StepName,
			"value": stepProfile.Duration.Milliseconds(),
			"data": map[string]interface{}{
				"step_id": stepID,
				"memory":  stepProfile.MemoryUsage.AvgMemoryMB,
			},
		}
		flameData["children"] = append(flameData["children"].([]map[string]interface{}), stepData)
	}

	return flameData
}