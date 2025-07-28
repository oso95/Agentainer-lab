package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v8"
	"github.com/agentainer/agentainer-lab/internal/storage"
)

// Metrics represents resource usage metrics for an agent
type Metrics struct {
	AgentID      string    `json:"agent_id"`
	Timestamp    time.Time `json:"timestamp"`
	CPU          CPUStats  `json:"cpu"`
	Memory       MemStats  `json:"memory"`
	Network      NetStats  `json:"network"`
	Disk         DiskStats `json:"disk"`
	ContainerID  string    `json:"container_id"`
}

// CPUStats represents CPU usage statistics
type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
	SystemCPU    uint64  `json:"system_cpu"`
	TotalUsage   uint64  `json:"total_usage"`
}

// MemStats represents memory usage statistics
type MemStats struct {
	Usage       uint64  `json:"usage"`
	Limit       uint64  `json:"limit"`
	UsagePercent float64 `json:"usage_percent"`
	Cache       uint64  `json:"cache"`
}

// NetStats represents network I/O statistics
type NetStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
}

// DiskStats represents disk I/O statistics
type DiskStats struct {
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
}

// Collector manages metrics collection for all agents
type Collector struct {
	dockerClient *client.Client
	storage      *storage.Storage
	redisClient  *redis.Client
	
	mu       sync.RWMutex
	agents   map[string]*agentCollector
	stopChan chan struct{}
	wg       sync.WaitGroup
}

type agentCollector struct {
	agentID     string
	containerID string
	lastStats   *types.StatsJSON
	stopChan    chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(dockerClient *client.Client, storage *storage.Storage) *Collector {
	return &Collector{
		dockerClient: dockerClient,
		storage:      storage,
		redisClient:  storage.GetRedisClient(),
		agents:       make(map[string]*agentCollector),
		stopChan:     make(chan struct{}),
	}
}

// Start begins metrics collection
func (c *Collector) Start(ctx context.Context) error {
	log.Println("Starting metrics collector...")
	
	// Start monitoring existing agents
	agents, err := c.storage.ListAgents()
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	
	for _, agent := range agents {
		if agent.Status == "running" && agent.ContainerID != "" {
			c.StartCollecting(agent.ID, agent.ContainerID)
		}
	}
	
	// Subscribe to agent events
	go c.watchAgentEvents(ctx)
	
	return nil
}

// Stop gracefully stops the collector
func (c *Collector) Stop() {
	log.Println("Stopping metrics collector...")
	close(c.stopChan)
	
	c.mu.Lock()
	for _, collector := range c.agents {
		close(collector.stopChan)
	}
	c.mu.Unlock()
	
	c.wg.Wait()
}

// StartCollecting begins metrics collection for a specific agent
func (c *Collector) StartCollecting(agentID, containerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Stop existing collector if any
	if existing, ok := c.agents[agentID]; ok {
		close(existing.stopChan)
		delete(c.agents, agentID)
	}
	
	collector := &agentCollector{
		agentID:     agentID,
		containerID: containerID,
		stopChan:    make(chan struct{}),
	}
	
	c.agents[agentID] = collector
	
	c.wg.Add(1)
	go c.collectMetrics(collector)
}

// StopCollecting stops metrics collection for a specific agent
func (c *Collector) StopCollecting(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if collector, ok := c.agents[agentID]; ok {
		close(collector.stopChan)
		delete(c.agents, agentID)
	}
}

// GetMetrics retrieves the latest metrics for an agent
func (c *Collector) GetMetrics(agentID string) (*Metrics, error) {
	key := fmt.Sprintf("metrics:current:%s", agentID)
	data, err := c.redisClient.Get(context.Background(), key).Result()
	if err != nil {
		return nil, fmt.Errorf("no metrics available for agent %s", agentID)
	}
	
	var metrics Metrics
	if err := json.Unmarshal([]byte(data), &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}
	
	return &metrics, nil
}

// GetMetricsHistory retrieves historical metrics for an agent
func (c *Collector) GetMetricsHistory(agentID string, duration time.Duration) ([]Metrics, error) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-duration)
	
	// Use Redis sorted set to store time-series data
	key := fmt.Sprintf("metrics:history:%s", agentID)
	results, err := c.redisClient.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", startTime.Unix()),
		Max: fmt.Sprintf("%d", endTime.Unix()),
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics history: %w", err)
	}
	
	metrics := make([]Metrics, 0, len(results))
	for _, result := range results {
		var m Metrics
		if err := json.Unmarshal([]byte(result), &m); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	
	return metrics, nil
}

func (c *Collector) collectMetrics(collector *agentCollector) {
	defer c.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	// Run initial collection
	c.collectOnce(collector)
	
	for {
		select {
		case <-ticker.C:
			c.collectOnce(collector)
		case <-collector.stopChan:
			return
		case <-c.stopChan:
			return
		}
	}
}

func (c *Collector) collectOnce(collector *agentCollector) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Get container stats
	statsResp, err := c.dockerClient.ContainerStats(ctx, collector.containerID, false)
	if err != nil {
		log.Printf("Failed to get stats for container %s: %v", collector.containerID, err)
		return
	}
	defer statsResp.Body.Close()
	
	var stats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		log.Printf("Failed to decode stats: %v", err)
		return
	}
	
	// Calculate metrics
	metrics := c.calculateMetrics(collector.agentID, collector.containerID, &stats, collector.lastStats)
	collector.lastStats = &stats
	
	// Store current metrics
	c.storeMetrics(metrics)
}

func (c *Collector) calculateMetrics(agentID, containerID string, current, previous *types.StatsJSON) *Metrics {
	metrics := &Metrics{
		AgentID:     agentID,
		ContainerID: containerID,
		Timestamp:   time.Now(),
	}
	
	// Calculate CPU usage
	if previous != nil {
		cpuDelta := float64(current.CPUStats.CPUUsage.TotalUsage - previous.CPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(current.CPUStats.SystemUsage - previous.CPUStats.SystemUsage)
		
		if systemDelta > 0 && cpuDelta > 0 {
			cpuPercent := (cpuDelta / systemDelta) * float64(len(current.CPUStats.CPUUsage.PercpuUsage)) * 100.0
			metrics.CPU.UsagePercent = cpuPercent
		}
	}
	
	metrics.CPU.TotalUsage = current.CPUStats.CPUUsage.TotalUsage
	metrics.CPU.SystemCPU = current.CPUStats.SystemUsage
	
	// Memory metrics
	metrics.Memory.Usage = current.MemoryStats.Usage
	metrics.Memory.Limit = current.MemoryStats.Limit
	metrics.Memory.Cache = current.MemoryStats.Stats["cache"]
	
	if current.MemoryStats.Limit > 0 {
		metrics.Memory.UsagePercent = (float64(current.MemoryStats.Usage) / float64(current.MemoryStats.Limit)) * 100.0
	}
	
	// Network metrics (sum all interfaces)
	for _, netStats := range current.Networks {
		metrics.Network.RxBytes += netStats.RxBytes
		metrics.Network.TxBytes += netStats.TxBytes
		metrics.Network.RxPackets += netStats.RxPackets
		metrics.Network.TxPackets += netStats.TxPackets
	}
	
	// Disk I/O metrics
	for _, ioStats := range current.BlkioStats.IoServiceBytesRecursive {
		switch ioStats.Op {
		case "Read":
			metrics.Disk.ReadBytes += ioStats.Value
		case "Write":
			metrics.Disk.WriteBytes += ioStats.Value
		}
	}
	
	return metrics
}

func (c *Collector) storeMetrics(metrics *Metrics) {
	ctx := context.Background()
	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("Failed to marshal metrics: %v", err)
		return
	}
	
	// Store current metrics
	currentKey := fmt.Sprintf("metrics:current:%s", metrics.AgentID)
	c.redisClient.Set(ctx, currentKey, data, 1*time.Hour)
	
	// Store in history (keep 24 hours of data)
	historyKey := fmt.Sprintf("metrics:history:%s", metrics.AgentID)
	c.redisClient.ZAdd(ctx, historyKey, &redis.Z{
		Score:  float64(metrics.Timestamp.Unix()),
		Member: string(data),
	})
	
	// Clean up old data
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	c.redisClient.ZRemRangeByScore(ctx, historyKey, "0", fmt.Sprintf("%d", cutoff))
}

func (c *Collector) watchAgentEvents(ctx context.Context) {
	// Subscribe to agent status changes
	pubsub := c.redisClient.Subscribe(ctx, "agent:status:*")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			// Parse agent ID from channel name
			if len(msg.Channel) > 13 { // "agent:status:"
				agentID := msg.Channel[13:]
				
				// Get agent details
				agent, err := c.storage.GetAgent(agentID)
				if err != nil {
					continue
				}
				
				if msg.Payload == "running" && agent.ContainerID != "" {
					c.StartCollecting(agentID, agent.ContainerID)
				} else {
					c.StopCollecting(agentID)
				}
			}
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		}
	}
}