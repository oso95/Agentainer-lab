package agentsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v8"
)

// Agent represents the agent structure for sync purposes
type Agent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	ContainerID string    `json:"container_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// QuickSync performs an immediate synchronization of a specific agent or all agents
type QuickSync struct {
	dockerClient *client.Client
	redisClient  *redis.Client
}

// NewQuickSync creates a new quick sync utility
func NewQuickSync(dockerClient *client.Client, redisClient *redis.Client) *QuickSync {
	return &QuickSync{
		dockerClient: dockerClient,
		redisClient:  redisClient,
	}
}

// SyncAgent synchronizes a specific agent's state immediately
func (q *QuickSync) SyncAgent(ctx context.Context, agentID string) error {
	// Get agent from Redis
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := q.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("agent not found")
	} else if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}
	
	var agentObj Agent
	if err := json.Unmarshal([]byte(data), &agentObj); err != nil {
		return fmt.Errorf("failed to unmarshal agent: %w", err)
	}
	
	// Check container state
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("agentainer.id=%s", agentID))
	
	containers, err := q.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	
	updated := false
	if len(containers) > 0 {
		container := containers[0]
		newStatus := dockerStateToAgentStatus(container.State)
		
		if agentObj.Status != newStatus || agentObj.ContainerID != container.ID {
			agentObj.Status = newStatus
			agentObj.ContainerID = container.ID
			updated = true
		}
	} else {
		// No container found
		if agentObj.Status == "running" || agentObj.Status == "paused" {
			agentObj.Status = "stopped"
			agentObj.ContainerID = ""
			updated = true
		} else if agentObj.ContainerID != "" {
			agentObj.ContainerID = ""
			updated = true
		}
	}
	
	if updated {
		agentObj.UpdatedAt = time.Now()
		
		updatedData, err := json.Marshal(agentObj)
		if err != nil {
			return fmt.Errorf("failed to marshal agent: %w", err)
		}
		
		if err := q.redisClient.Set(ctx, key, updatedData, 0).Err(); err != nil {
			return fmt.Errorf("failed to save agent: %w", err)
		}
		
		log.Printf("Quick sync: Updated agent %s status to %s", agentID, agentObj.Status)
	}
	
	return nil
}

// SyncAll synchronizes all agents immediately
func (q *QuickSync) SyncAll(ctx context.Context) error {
	// Get all agent IDs
	agentIDs, err := q.redisClient.SMembers(ctx, "agents:list").Result()
	if err != nil {
		return fmt.Errorf("failed to get agent list: %w", err)
	}
	
	// Get all containers with agentainer labels
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", "agentainer.id")
	
	containers, err := q.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	
	// Create container map
	containerMap := make(map[string]types.Container)
	for _, container := range containers {
		if agentID, ok := container.Labels["agentainer.id"]; ok {
			containerMap[agentID] = container
		}
	}
	
	// Sync each agent
	for _, agentID := range agentIDs {
		if err := q.syncAgentWithMap(ctx, agentID, containerMap); err != nil {
			log.Printf("Failed to sync agent %s: %v", agentID, err)
		}
	}
	
	return nil
}

func (q *QuickSync) syncAgentWithMap(ctx context.Context, agentID string, containerMap map[string]types.Container) error {
	// Get agent from Redis
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := q.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		q.redisClient.SRem(ctx, "agents:list", agentID)
		return nil
	} else if err != nil {
		return err
	}
	
	var agentObj Agent
	if err := json.Unmarshal([]byte(data), &agentObj); err != nil {
		return err
	}
	
	updated := false
	if container, exists := containerMap[agentID]; exists {
		newStatus := dockerStateToAgentStatus(container.State)
		if agentObj.Status != newStatus || agentObj.ContainerID != container.ID {
			agentObj.Status = newStatus
			agentObj.ContainerID = container.ID
			updated = true
		}
	} else {
		if agentObj.Status == "running" || agentObj.Status == "paused" {
			agentObj.Status = "stopped"
			agentObj.ContainerID = ""
			updated = true
		} else if agentObj.ContainerID != "" {
			agentObj.ContainerID = ""
			updated = true
		}
	}
	
	if updated {
		agentObj.UpdatedAt = time.Now()
		updatedData, err := json.Marshal(agentObj)
		if err != nil {
			return err
		}
		
		return q.redisClient.Set(ctx, key, updatedData, 0).Err()
	}
	
	return nil
}

func dockerStateToAgentStatus(state string) string {
	switch state {
	case "running":
		return "running"
	case "paused":
		return "paused"
	case "created":
		return "created"
	case "exited", "dead", "removing", "removed":
		return "stopped"
	default:
		return "failed"
	}
}