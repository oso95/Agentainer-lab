package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v8"
	"github.com/agentainer/agentainer-lab/pkg/agentsync"
)

type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusPaused  Status = "paused"
	StatusFailed  Status = "failed"
	
	// Network configuration
	AgentainerNetworkName = "agentainer-network"
)

func (s Status) MarshalBinary() ([]byte, error) {
	return []byte(s), nil
}

func (s *Status) UnmarshalBinary(data []byte) error {
	*s = Status(data)
	return nil
}

type Agent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	ContainerID  string            `json:"container_id"`
	Status       Status            `json:"status"`
	EnvVars      map[string]string `json:"env_vars"`
	CPULimit     int64             `json:"cpu_limit"`
	MemoryLimit  int64             `json:"memory_limit"`
	AutoRestart  bool              `json:"auto_restart"`
	Token        string            `json:"token"`
	Ports        []PortMapping     `json:"ports"`
	Volumes      []VolumeMapping   `json:"volumes"`
	HealthCheck  *HealthCheckConfig `json:"health_check,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type PortMapping struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
}

type VolumeMapping struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

type HealthCheckConfig struct {
	Endpoint string `json:"endpoint"`
	Interval string `json:"interval"`
	Timeout  string `json:"timeout,omitempty"`
	Retries  int    `json:"retries,omitempty"`
}

type Manager struct {
	dockerClient *client.Client
	redisClient  *redis.Client
	configPath   string
	quickSync    *agentsync.QuickSync
}

func NewManager(dockerClient *client.Client, redisClient *redis.Client, configPath string) *Manager {
	m := &Manager{
		dockerClient: dockerClient,
		redisClient:  redisClient,
		configPath:   configPath,
		quickSync:    agentsync.NewQuickSync(dockerClient, redisClient),
	}
	
	// Ensure the internal network exists
	ctx := context.Background()
	if err := m.ensureNetworkExists(ctx); err != nil {
		log.Printf("Warning: Failed to create network: %v", err)
	}
	
	return m
}

func (m *Manager) Deploy(ctx context.Context, name, image string, envVars map[string]string, cpuLimit, memoryLimit int64, autoRestart bool, token string, ports []PortMapping, volumes []VolumeMapping, healthCheck *HealthCheckConfig) (*Agent, error) {
	// Validate that the Docker image exists
	_, _, err := m.dockerClient.ImageInspectWithRaw(ctx, image)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, fmt.Errorf("docker image '%s' not found. Please build or pull the image first", image)
		}
		return nil, fmt.Errorf("failed to inspect docker image: %w", err)
	}
	
	id := generateID()
	
	// In the new architecture, we don't expose ports directly
	// All access is through the proxy
	// ports parameter is kept for backward compatibility but ignored
	
	agent := &Agent{
		ID:          id,
		Name:        name,
		Image:       image,
		Status:      StatusCreated,
		EnvVars:     envVars,
		CPULimit:    cpuLimit,
		MemoryLimit: memoryLimit,
		AutoRestart: autoRestart,
		Token:       token,
		Ports:       []PortMapping{}, // No longer exposing ports
		Volumes:     volumes,
		HealthCheck: healthCheck,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := m.saveAgent(agent); err != nil {
		return nil, fmt.Errorf("failed to save agent: %w", err)
	}

	return agent, nil
}

func (m *Manager) Start(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	if agent.Status == StatusRunning {
		return fmt.Errorf("agent is already running")
	}

	if agent.ContainerID != "" {
		if err := m.dockerClient.ContainerStart(ctx, agent.ContainerID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("failed to start existing container: %w", err)
		}
	} else {
		containerID, err := m.createContainer(ctx, agent)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}
		agent.ContainerID = containerID
	}

	agent.Status = StatusRunning
	agent.UpdatedAt = time.Now()
	
	if err := m.saveAgent(agent); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}
	
	// Trigger immediate sync to ensure consistency
	go func() {
		if err := m.quickSync.SyncAgent(context.Background(), agentID); err != nil {
			log.Printf("Failed to quick sync agent %s after start: %v", agentID, err)
		}
	}()

	return nil
}

func (m *Manager) Stop(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	if agent.Status == StatusStopped {
		return fmt.Errorf("agent is already stopped")
	}

	if agent.ContainerID != "" {
		timeout := 10
		if err := m.dockerClient.ContainerStop(ctx, agent.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	agent.Status = StatusStopped
	agent.UpdatedAt = time.Now()
	
	if err := m.saveAgent(agent); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}
	
	// Trigger immediate sync to ensure consistency
	go func() {
		if err := m.quickSync.SyncAgent(context.Background(), agentID); err != nil {
			log.Printf("Failed to quick sync agent %s after stop: %v", agentID, err)
		}
	}()

	return nil
}

func (m *Manager) Restart(ctx context.Context, agentID string) error {
	if err := m.Stop(ctx, agentID); err != nil {
		return err
	}
	return m.Start(ctx, agentID)
}

func (m *Manager) Pause(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	if agent.Status != StatusRunning {
		return fmt.Errorf("agent is not running")
	}

	if err := m.dockerClient.ContainerPause(ctx, agent.ContainerID); err != nil {
		return fmt.Errorf("failed to pause container: %w", err)
	}

	agent.Status = StatusPaused
	agent.UpdatedAt = time.Now()
	
	if err := m.saveAgent(agent); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}
	
	// Trigger immediate sync to ensure consistency
	go func() {
		if err := m.quickSync.SyncAgent(context.Background(), agentID); err != nil {
			log.Printf("Failed to quick sync agent %s after pause: %v", agentID, err)
		}
	}()

	return nil
}

func (m *Manager) Resume(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	switch agent.Status {
	case StatusRunning:
		return fmt.Errorf("agent is already running")
	
	case StatusPaused:
		// Unpause the container
		if err := m.dockerClient.ContainerUnpause(ctx, agent.ContainerID); err != nil {
			return fmt.Errorf("failed to resume paused container: %w", err)
		}
	
	case StatusStopped, StatusFailed, StatusCreated:
		// Rehydrate from saved state - restart the container
		if agent.ContainerID != "" {
			// Try to start existing container
			if err := m.dockerClient.ContainerStart(ctx, agent.ContainerID, types.ContainerStartOptions{}); err != nil {
				// If start fails, create a new container with same configuration
				containerID, createErr := m.createContainer(ctx, agent)
				if createErr != nil {
					return fmt.Errorf("failed to rehydrate agent state: %w", createErr)
				}
				agent.ContainerID = containerID
			}
		} else {
			// No existing container, create new one with saved configuration
			containerID, err := m.createContainer(ctx, agent)
			if err != nil {
				return fmt.Errorf("failed to rehydrate agent state: %w", err)
			}
			agent.ContainerID = containerID
		}
	
	default:
		return fmt.Errorf("cannot resume agent in status: %s", agent.Status)
	}

	agent.Status = StatusRunning
	agent.UpdatedAt = time.Now()
	
	if err := m.saveAgent(agent); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}
	
	// Trigger immediate sync to ensure consistency
	go func() {
		if err := m.quickSync.SyncAgent(context.Background(), agentID); err != nil {
			log.Printf("Failed to quick sync agent %s after resume: %v", agentID, err)
		}
	}()

	return nil
}

func (m *Manager) Remove(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	// Stop the container if it's running
	if agent.Status == StatusRunning || agent.Status == StatusPaused {
		if agent.ContainerID != "" {
			timeout := 10
			if err := m.dockerClient.ContainerStop(ctx, agent.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
				// Log but don't fail if stop fails - we still want to clean up
				log.Printf("Warning: failed to stop container %s: %v", agent.ContainerID, err)
			}
		}
	}

	// Remove the container from Docker
	if agent.ContainerID != "" {
		if err := m.dockerClient.ContainerRemove(ctx, agent.ContainerID, types.ContainerRemoveOptions{Force: true}); err != nil {
			// Log but don't fail if remove fails - container might already be gone
			log.Printf("Warning: failed to remove container %s: %v", agent.ContainerID, err)
		}
	}

	// Remove agent from storage
	if err := m.removeAgentFromStorage(agentID); err != nil {
		return fmt.Errorf("failed to remove agent from storage: %w", err)
	}

	// Remove agent from Redis cache
	if err := m.redisClient.Del(ctx, fmt.Sprintf("agent:%s", agentID)).Err(); err != nil {
		// Log but don't fail if Redis deletion fails
		log.Printf("Warning: failed to remove agent from cache: %v", err)
	}
	
	// Clean up any request queues for this agent
	requestKeys := []string{
		fmt.Sprintf("agent:%s:requests:pending", agentID),
		fmt.Sprintf("agent:%s:requests:completed", agentID),
		fmt.Sprintf("agent:%s:requests:failed", agentID),
	}
	for _, key := range requestKeys {
		if err := m.redisClient.Del(ctx, key).Err(); err != nil {
			log.Printf("Warning: failed to remove request queue %s: %v", key, err)
		}
	}
	
	// Also clean up any individual request data
	iter := m.redisClient.Scan(ctx, 0, fmt.Sprintf("request:%s:*", agentID), 0).Iterator()
	for iter.Next(ctx) {
		if err := m.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			log.Printf("Warning: failed to remove request %s: %v", iter.Val(), err)
		}
	}

	return nil
}

func (m *Manager) GetAgent(agentID string) (*Agent, error) {
	ctx := context.Background()
	
	// Get agent from Redis
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := m.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("agent not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	
	var agent Agent
	if err := json.Unmarshal([]byte(data), &agent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent: %w", err)
	}
	
	return &agent, nil
}

func (m *Manager) ListAgents(token string) ([]Agent, error) {
	// Quick sync all agents before listing to ensure fresh data
	ctx := context.Background()
	if err := m.quickSync.SyncAll(ctx); err != nil {
		// Log but don't fail - still return what we have
		log.Printf("Warning: Failed to sync before list: %v", err)
	}
	
	allAgents, err := m.loadAgents()
	if err != nil {
		return nil, err
	}
	
	// Always return all agents - filtering by token is deprecated
	// in the network-isolated architecture where tokens are only
	// used for API authentication, not agent ownership
	return allAgents, nil
}

func (m *Manager) GetLogs(ctx context.Context, agentID string, follow bool) (io.ReadCloser, error) {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return nil, err
	}

	if agent.ContainerID == "" {
		return nil, fmt.Errorf("container not found")
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	}

	return m.dockerClient.ContainerLogs(ctx, agent.ContainerID, options)
}

func (m *Manager) createContainer(ctx context.Context, agent *Agent) (string, error) {
	env := make([]string, 0, len(agent.EnvVars))
	for key, value := range agent.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// No port bindings in the new architecture
	// Containers are accessed through the proxy only

	// Create volume mounts
	var mounts []mount.Mount
	for _, volume := range agent.Volumes {
		// Ensure host directory exists
		hostPath, err := filepath.Abs(volume.HostPath)
		if err != nil {
			return "", fmt.Errorf("invalid host path %s: %w", volume.HostPath, err)
		}
		
		// Create directory if it doesn't exist
		if err := os.MkdirAll(hostPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create host directory %s: %w", hostPath, err)
		}
		
		mountType := mount.TypeBind
		if volume.ReadOnly {
			mounts = append(mounts, mount.Mount{
				Type:     mountType,
				Source:   hostPath,
				Target:   volume.ContainerPath,
				ReadOnly: true,
			})
		} else {
			mounts = append(mounts, mount.Mount{
				Type:   mountType,
				Source: hostPath,
				Target: volume.ContainerPath,
			})
		}
	}

	config := &container.Config{
		Image:        agent.Image,
		Env:          env,
		Labels: map[string]string{
			"agentainer.id":   agent.ID,
			"agentainer.name": agent.Name,
		},
		Hostname: agent.ID, // Use agent ID as hostname for easy identification
	}

	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "no",
		},
		Resources: container.Resources{
			Memory:   agent.MemoryLimit,
			NanoCPUs: agent.CPULimit,
		},
		Mounts:       mounts,
		NetworkMode: container.NetworkMode(AgentainerNetworkName),
	}

	if agent.AutoRestart {
		hostConfig.RestartPolicy.Name = "always"
	}
	

	resp, err := m.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := m.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (m *Manager) saveAgent(agent *Agent) error {
	ctx := context.Background()
	
	// Save agent to Redis as primary storage
	data, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent: %w", err)
	}
	
	key := fmt.Sprintf("agent:%s", agent.ID)
	if err := m.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save agent to Redis: %w", err)
	}
	
	// Also save to agents list for efficient listing
	if err := m.redisClient.SAdd(ctx, "agents:list", agent.ID).Err(); err != nil {
		return fmt.Errorf("failed to add agent to list: %w", err)
	}
	
	return nil
}

func (m *Manager) removeAgentFromStorage(agentID string) error {
	// Remove agent from Redis storage
	ctx := context.Background()
	key := fmt.Sprintf("agent:%s", agentID)
	
	// Check if agent exists first
	exists, err := m.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check agent existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("agent not found in storage")
	}
	
	// Delete the agent
	if err := m.redisClient.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete agent from Redis: %w", err)
	}
	
	// Remove from agents list
	if err := m.redisClient.SRem(ctx, "agents:list", agentID).Err(); err != nil {
		return fmt.Errorf("failed to remove agent from list: %w", err)
	}
	
	return nil
}

func (m *Manager) loadAgents() ([]Agent, error) {
	ctx := context.Background()
	
	// Get all agent IDs from Redis set
	agentIDs, err := m.redisClient.SMembers(ctx, "agents:list").Result()
	if err != nil {
		log.Printf("ERROR: Failed to get agent list from Redis: %v", err)
		return nil, fmt.Errorf("failed to get agent list: %w", err)
	}
	
	log.Printf("DEBUG: Found %d agent IDs in Redis: %v", len(agentIDs), agentIDs)
	
	agents := make([]Agent, 0, len(agentIDs))
	for _, id := range agentIDs {
		key := fmt.Sprintf("agent:%s", id)
		data, err := m.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			// Agent in list but not found, clean up
			m.redisClient.SRem(ctx, "agents:list", id)
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to get agent %s: %w", id, err)
		}
		
		var agent Agent
		if err := json.Unmarshal([]byte(data), &agent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent %s: %w", id, err)
		}
		
		agents = append(agents, agent)
	}
	
	return agents, nil
}

func generateID() string {
	return fmt.Sprintf("agent-%d", time.Now().UnixNano())
}

func (m *Manager) ensureNetworkExists(ctx context.Context) error {
	// Check if network already exists
	networks, err := m.dockerClient.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	
	for _, net := range networks {
		if net.Name == AgentainerNetworkName {
			return nil // Network already exists
		}
	}
	
	// Create the network
	_, err = m.dockerClient.NetworkCreate(ctx, AgentainerNetworkName, types.NetworkCreate{
		Driver: "bridge",
		Options: map[string]string{
			"com.docker.network.bridge.name": "agentainer0",
		},
		Labels: map[string]string{
			"managed-by": "agentainer",
		},
	})
	
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	
	log.Printf("Created Agentainer network: %s", AgentainerNetworkName)
	return nil
}