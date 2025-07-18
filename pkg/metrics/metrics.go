package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/agentainer/agentainer-lab/internal/storage"
)

type Collector struct {
	dockerClient *client.Client
	storage      *storage.Storage
}

type AgentMetrics struct {
	AgentID     string  `json:"agent_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsage int64   `json:"memory_usage"`
	MemoryLimit int64   `json:"memory_limit"`
	NetworkRx   int64   `json:"network_rx"`
	NetworkTx   int64   `json:"network_tx"`
	Uptime      int64   `json:"uptime"`
	Timestamp   int64   `json:"timestamp"`
}

func NewCollector(dockerClient *client.Client, storage *storage.Storage) *Collector {
	return &Collector{
		dockerClient: dockerClient,
		storage:      storage,
	}
}

func (c *Collector) CollectMetrics(ctx context.Context, agentID, containerID string) (*AgentMetrics, error) {
	stats, err := c.dockerClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer stats.Body.Close()

	var containerStats types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	cpuPercent := calculateCPUPercent(containerStats)
	memoryUsage := containerStats.MemoryStats.Usage
	memoryLimit := containerStats.MemoryStats.Limit

	var networkRx, networkTx int64
	for _, network := range containerStats.Networks {
		networkRx += int64(network.RxBytes)
		networkTx += int64(network.TxBytes)
	}

	uptime := time.Now().Unix() - containerStats.Read.Unix()

	metrics := &AgentMetrics{
		AgentID:     agentID,
		CPUPercent:  cpuPercent,
		MemoryUsage: int64(memoryUsage),
		MemoryLimit: int64(memoryLimit),
		NetworkRx:   networkRx,
		NetworkTx:   networkTx,
		Uptime:      uptime,
		Timestamp:   time.Now().Unix(),
	}

	if err := c.storeMetrics(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to store metrics: %w", err)
	}

	return metrics, nil
}

func (c *Collector) GetAgentMetrics(ctx context.Context, agentID string) (*AgentMetrics, error) {
	key := fmt.Sprintf("agent:%s:metrics", agentID)
	data, err := c.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	var metrics AgentMetrics
	if err := json.Unmarshal([]byte(data), &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

func (c *Collector) storeMetrics(ctx context.Context, metrics *AgentMetrics) error {
	key := fmt.Sprintf("agent:%s:metrics", metrics.AgentID)
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	return c.storage.Set(ctx, key, string(data), time.Hour)
}

func calculateCPUPercent(stats types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	
	if systemDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}