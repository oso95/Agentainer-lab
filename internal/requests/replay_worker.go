package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// ReplayWorker handles automatic replay of pending requests
type ReplayWorker struct {
	manager      *Manager
	redisClient  *redis.Client
	httpClient   *http.Client
	stopCh       chan bool
}

// NewReplayWorker creates a new replay worker
func NewReplayWorker(manager *Manager, redisClient *redis.Client) *ReplayWorker {
	return &ReplayWorker{
		manager:     manager,
		redisClient: redisClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan bool),
	}
}

// Start begins the replay worker
func (w *ReplayWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processAgents(ctx)
		}
	}
}

// Stop stops the replay worker
func (w *ReplayWorker) Stop() {
	close(w.stopCh)
}

// processAgents checks all agents for pending requests
func (w *ReplayWorker) processAgents(ctx context.Context) {
	// Get all agent IDs from Redis pattern
	keys, err := w.redisClient.Keys(ctx, "agent:*:requests:pending").Result()
	if err != nil {
		fmt.Printf("Error getting agent keys: %v\n", err)
		return
	}

	fmt.Printf("[ReplayWorker] Found %d agents with pending requests\n", len(keys))
	
	for _, key := range keys {
		// Extract agent ID from key
		agentID := extractAgentID(key)
		if agentID == "" {
			continue
		}

		// Check if agent is running
		isRunning := w.isAgentRunning(ctx, agentID)
		fmt.Printf("[ReplayWorker] Agent %s running status: %v\n", agentID, isRunning)
		if !isRunning {
			fmt.Printf("[ReplayWorker] Agent %s is not running, skipping\n", agentID)
			continue
		}

		fmt.Printf("[ReplayWorker] Processing pending requests for agent %s\n", agentID)
		// Process pending requests for this agent
		w.processPendingRequests(ctx, agentID)
	}
}

// processPendingRequests replays all pending requests for an agent
func (w *ReplayWorker) processPendingRequests(ctx context.Context, agentID string) {
	requests, err := w.manager.GetPendingRequests(ctx, agentID)
	if err != nil {
		fmt.Printf("Error getting pending requests for agent %s: %v\n", agentID, err)
		return
	}

	fmt.Printf("[ReplayWorker] Found %d pending requests for agent %s\n", len(requests), agentID)
	
	for _, req := range requests {
		// Skip if already processing or too many retries
		if req.Status == StatusProcessing || req.RetryCount >= req.MaxRetries {
			fmt.Printf("[ReplayWorker] Skipping request %s (status=%s, retries=%d/%d)\n", 
				req.ID, req.Status, req.RetryCount, req.MaxRetries)
			continue
		}

		fmt.Printf("[ReplayWorker] Replaying request %s: %s %s\n", req.ID, req.Method, req.Path)
		// Replay the request
		if err := w.replayRequest(ctx, agentID, req); err != nil {
			fmt.Printf("Error replaying request %s: %v\n", req.ID, err)
			// Mark as failed
			w.manager.MarkRequestFailed(ctx, agentID, req.ID, err)
		} else {
			fmt.Printf("[ReplayWorker] Successfully replayed request %s\n", req.ID)
		}
	}
}

// replayRequest replays a single request
func (w *ReplayWorker) replayRequest(ctx context.Context, agentID string, req *Request) error {
	// Use the proxy endpoint for replay since replay worker runs outside Docker network
	// Remove the agent prefix from the path if present
	path := req.Path
	prefix := fmt.Sprintf("/agent/%s", agentID)
	if strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
		if path == "" {
			path = "/"
		}
	}
	
	// Create target URL through proxy
	targetURL := fmt.Sprintf("http://localhost:8081/agent/%s%s", agentID, path)
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, bytes.NewReader(req.Body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Restore headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Add tracking header
	httpReq.Header.Set("X-Agentainer-Request-ID", req.ID)
	httpReq.Header.Set("X-Agentainer-Replay", "true")

	// Execute request
	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Store response
	if err := w.manager.StoreResponse(ctx, agentID, req.ID, resp); err != nil {
		fmt.Printf("Warning: Failed to store response for request %s: %v\n", req.ID, err)
	}

	return nil
}

// isAgentRunning checks if an agent is running
func (w *ReplayWorker) isAgentRunning(ctx context.Context, agentID string) bool {
	// Use the agent manager's GetAgent method to properly parse the agent data
	key := fmt.Sprintf("agent:%s", agentID)
	data, err := w.redisClient.Get(ctx, key).Result()
	if err != nil {
		// If not in Redis, agent doesn't exist
		return false
	}
	
	// Parse the JSON to check status
	// We need to import encoding/json for this
	var agentData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &agentData); err != nil {
		fmt.Printf("[ReplayWorker] Failed to parse agent data for %s: %v\n", agentID, err)
		return false
	}
	
	// Check if status is "running"
	if status, ok := agentData["status"].(string); ok {
		return status == "running"
	}
	
	return false
}

// extractAgentID extracts agent ID from Redis key
func extractAgentID(key string) string {
	// Key format: agent:{id}:requests:pending
	parts := strings.Split(key, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}