package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v8"
)

// StateSynchronizer keeps Agentainer agent states in sync with Docker container states
type StateSynchronizer struct {
	dockerClient *client.Client
	redisClient  *redis.Client
	interval     time.Duration
	
	mu       sync.RWMutex
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewStateSynchronizer creates a new state synchronizer
func NewStateSynchronizer(dockerClient *client.Client, redisClient *redis.Client, interval time.Duration) *StateSynchronizer {
	if interval <= 0 {
		interval = 30 * time.Second // Default interval
	}
	
	return &StateSynchronizer{
		dockerClient: dockerClient,
		redisClient:  redisClient,
		interval:     interval,
		stopChan:     make(chan struct{}),
	}
}

// Start begins the synchronization process
func (s *StateSynchronizer) Start(ctx context.Context) error {
	log.Printf("Starting state synchronizer with interval: %v", s.interval)
	
	// Run initial sync immediately and log results
	log.Println("Running initial state synchronization...")
	if err := s.syncStates(ctx); err != nil {
		log.Printf("ERROR: Initial sync failed: %v", err)
		// Don't fail startup, just log the error
	} else {
		log.Println("Initial state synchronization completed successfully")
	}
	
	// Start periodic sync
	s.wg.Add(1)
	go s.runPeriodicSync(ctx)
	
	// Watch for Docker events
	s.wg.Add(1)
	go s.watchDockerEvents(ctx)
	
	return nil
}

// Stop gracefully stops the synchronizer
func (s *StateSynchronizer) Stop() {
	log.Println("Stopping state synchronizer...")
	close(s.stopChan)
	s.wg.Wait()
}

// syncStates performs a full synchronization of all agent states
func (s *StateSynchronizer) syncStates(ctx context.Context) error {
	// Get all agent IDs from Redis
	agentIDs, err := s.redisClient.SMembers(ctx, "agents:list").Result()
	if err != nil {
		return fmt.Errorf("failed to get agent list: %w", err)
	}
	
	log.Printf("Starting sync for %d agents: %v", len(agentIDs), agentIDs)
	
	// Get all containers with agentainer labels
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", "agentainer.id")
	
	containers, err := s.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	
	log.Printf("Found %d containers with agentainer labels", len(containers))
	
	// Create a map of agent ID to container for quick lookup
	containerMap := make(map[string]types.Container)
	for _, container := range containers {
		if agentID, ok := container.Labels["agentainer.id"]; ok {
			containerMap[agentID] = container
			log.Printf("Found container %s for agent %s (state: %s)", 
				container.ID[:12], agentID, container.State)
		}
	}
	
	// Sync each agent
	successCount := 0
	failCount := 0
	for _, agentID := range agentIDs {
		if err := s.syncAgent(ctx, agentID, containerMap); err != nil {
			log.Printf("Failed to sync agent %s: %v", agentID, err)
			failCount++
		} else {
			successCount++
		}
	}
	
	log.Printf("Sync completed: %d successful, %d failed", successCount, failCount)
	
	return nil
}

// syncAgent syncs a single agent's state
func (s *StateSynchronizer) syncAgent(ctx context.Context, agentID string, containerMap map[string]types.Container) error {
	// Get agent from Redis
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := s.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		// Agent not found in Redis, remove from list
		log.Printf("Agent %s not found in Redis, removing from list", agentID)
		s.redisClient.SRem(ctx, "agents:list", agentID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}
	
	var agentObj agent.Agent
	if err := json.Unmarshal([]byte(data), &agentObj); err != nil {
		return fmt.Errorf("failed to unmarshal agent: %w", err)
	}
	
	// Log current state
	log.Printf("Syncing agent %s (%s) - Current state: %s, Container ID: %s", 
		agentID, agentObj.Name, agentObj.Status, agentObj.ContainerID)
	
	// Check container state
	container, exists := containerMap[agentID]
	updated := false
	
	if exists {
		// Container exists, update agent state based on container state
		newStatus := s.dockerStateToAgentStatus(container.State)
		if agentObj.Status != newStatus {
			log.Printf("Agent %s (%s): Docker container state is '%s', updating status from %s to %s", 
				agentID, agentObj.Name, container.State, agentObj.Status, newStatus)
			agentObj.Status = newStatus
			updated = true
		}
		
		// Update container ID if different
		if agentObj.ContainerID != container.ID {
			log.Printf("Agent %s (%s): container ID updated from %s to %s", 
				agentID, agentObj.Name, agentObj.ContainerID, container.ID)
			agentObj.ContainerID = container.ID
			updated = true
		}
	} else {
		// Container doesn't exist
		log.Printf("Agent %s (%s): No container found with label agentainer.id=%s", 
			agentID, agentObj.Name, agentID)
			
		if agentObj.Status == agent.StatusRunning || agentObj.Status == agent.StatusPaused {
			log.Printf("Agent %s (%s): was %s but container not found, marking as stopped", 
				agentID, agentObj.Name, agentObj.Status)
			agentObj.Status = agent.StatusStopped
			agentObj.ContainerID = ""
			updated = true
		} else if agentObj.ContainerID != "" {
			// Clear container ID if it's set but container doesn't exist
			log.Printf("Agent %s (%s): clearing non-existent container ID %s", 
				agentID, agentObj.Name, agentObj.ContainerID)
			agentObj.ContainerID = ""
			updated = true
		}
	}
	
	// Save updated agent if changed
	if updated {
		agentObj.UpdatedAt = time.Now()
		
		updatedData, err := json.Marshal(agentObj)
		if err != nil {
			return fmt.Errorf("failed to marshal agent: %w", err)
		}
		
		if err := s.redisClient.Set(ctx, key, updatedData, 0).Err(); err != nil {
			return fmt.Errorf("failed to save agent: %w", err)
		}
		
		// Also update the status key for backward compatibility
		statusKey := fmt.Sprintf("agent:%s:status", agentID)
		if err := s.redisClient.Set(ctx, statusKey, string(agentObj.Status), 0).Err(); err != nil {
			log.Printf("Failed to update status key: %v", err)
		}
		
		// Publish status change event
		s.publishStatusChange(ctx, agentID, agentObj.Status)
	}
	
	return nil
}

// dockerStateToAgentStatus converts Docker container state to agent status
func (s *StateSynchronizer) dockerStateToAgentStatus(state string) agent.Status {
	switch state {
	case "running":
		return agent.StatusRunning
	case "paused":
		return agent.StatusPaused
	case "created":
		return agent.StatusCreated
	case "exited", "dead", "removing", "removed":
		return agent.StatusStopped
	default:
		return agent.StatusFailed
	}
}

// runPeriodicSync runs synchronization at regular intervals
func (s *StateSynchronizer) runPeriodicSync(ctx context.Context) {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := s.syncStates(ctx); err != nil {
				log.Printf("Periodic sync failed: %v", err)
			}
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		}
	}
}

// watchDockerEvents watches for Docker container events and syncs affected agents
func (s *StateSynchronizer) watchDockerEvents(ctx context.Context) {
	defer s.wg.Done()
	
	// Create event filter for containers with agentainer labels
	eventFilters := filters.NewArgs()
	eventFilters.Add("type", "container")
	eventFilters.Add("label", "agentainer.id")
	
	events, errs := s.dockerClient.Events(ctx, types.EventsOptions{
		Filters: eventFilters,
	})
	
	for {
		select {
		case event := <-events:
			// Handle container state changes
			if agentID, ok := event.Actor.Attributes["agentainer.id"]; ok {
				log.Printf("Docker event for agent %s: %s", agentID, event.Action)
				
				// Get fresh container list for this agent
				containerFilters := filters.NewArgs()
				containerFilters.Add("label", fmt.Sprintf("agentainer.id=%s", agentID))
				
				containers, err := s.dockerClient.ContainerList(ctx, types.ContainerListOptions{
					All:     true,
					Filters: containerFilters,
				})
				if err != nil {
					log.Printf("Failed to list containers for agent %s: %v", agentID, err)
					continue
				}
				
				containerMap := make(map[string]types.Container)
				for _, container := range containers {
					containerMap[agentID] = container
				}
				
				// Sync this specific agent
				if err := s.syncAgent(ctx, agentID, containerMap); err != nil {
					log.Printf("Failed to sync agent %s after event: %v", agentID, err)
				}
			}
			
		case err := <-errs:
			if err != nil {
				log.Printf("Docker event error: %v", err)
			}
			return
			
		case <-ctx.Done():
			return
			
		case <-s.stopChan:
			return
		}
	}
}

// publishStatusChange publishes agent status change events
func (s *StateSynchronizer) publishStatusChange(ctx context.Context, agentID string, status agent.Status) {
	channel := fmt.Sprintf("agent:status:%s", agentID)
	if err := s.redisClient.Publish(ctx, channel, string(status)).Err(); err != nil {
		log.Printf("Failed to publish status change: %v", err)
	}
}

// SyncNow triggers an immediate synchronization
func (s *StateSynchronizer) SyncNow(ctx context.Context) error {
	log.Println("Triggering immediate state sync...")
	return s.syncStates(ctx)
}