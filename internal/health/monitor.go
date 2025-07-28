package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/go-redis/redis/v8"
)

// HealthStatus represents the health state of an agent
type HealthStatus struct {
	AgentID      string    `json:"agent_id"`
	Healthy      bool      `json:"healthy"`
	LastCheck    time.Time `json:"last_check"`
	FailureCount int       `json:"failure_count"`
	Message      string    `json:"message"`
}

// CheckConfig defines health check configuration for an agent
type CheckConfig struct {
	Endpoint string        `json:"endpoint"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// Monitor manages health checks for all agents
type Monitor struct {
	agentMgr    *agent.Manager
	redisClient *redis.Client
	httpClient  *http.Client
	
	mu          sync.RWMutex
	checks      map[string]*agentCheck
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

type agentCheck struct {
	agentID  string
	config   CheckConfig
	status   HealthStatus
	stopChan chan struct{}
}

// NewMonitor creates a new health monitor
func NewMonitor(agentMgr *agent.Manager, redisClient *redis.Client) *Monitor {
	return &Monitor{
		agentMgr:    agentMgr,
		redisClient: redisClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		checks:   make(map[string]*agentCheck),
		stopChan: make(chan struct{}),
	}
}

// Start begins monitoring all agents
func (m *Monitor) Start(ctx context.Context) error {
	log.Println("Starting health monitor...")
	
	// Start monitoring existing agents
	agents, err := m.agentMgr.ListAgents("")
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	
	for _, agent := range agents {
		if agent.Status == "running" {
			m.StartMonitoring(agent.ID, CheckConfig{
				Endpoint: "/health",
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
				Retries:  3,
			})
		}
	}
	
	// Subscribe to agent events
	go m.watchAgentEvents(ctx)
	
	return nil
}

// Stop gracefully stops the monitor
func (m *Monitor) Stop() {
	log.Println("Stopping health monitor...")
	close(m.stopChan)
	
	m.mu.Lock()
	for _, check := range m.checks {
		close(check.stopChan)
	}
	m.mu.Unlock()
	
	m.wg.Wait()
}

// StartMonitoring begins health checking for a specific agent
func (m *Monitor) StartMonitoring(agentID string, config CheckConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Stop existing check if any
	if existing, ok := m.checks[agentID]; ok {
		close(existing.stopChan)
		delete(m.checks, agentID)
	}
	
	// Default values
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.Retries == 0 {
		config.Retries = 3
	}
	if config.Endpoint == "" {
		config.Endpoint = "/health"
	}
	
	check := &agentCheck{
		agentID:  agentID,
		config:   config,
		stopChan: make(chan struct{}),
		status: HealthStatus{
			AgentID:   agentID,
			Healthy:   true,
			LastCheck: time.Now(),
		},
	}
	
	m.checks[agentID] = check
	
	m.wg.Add(1)
	go m.runHealthCheck(check)
}

// StopMonitoring stops health checking for a specific agent
func (m *Monitor) StopMonitoring(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if check, ok := m.checks[agentID]; ok {
		close(check.stopChan)
		delete(m.checks, agentID)
	}
}

// GetStatus returns the current health status of an agent
func (m *Monitor) GetStatus(agentID string) (*HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	check, ok := m.checks[agentID]
	if !ok {
		return nil, fmt.Errorf("no health check for agent %s", agentID)
	}
	
	status := check.status
	return &status, nil
}

// GetAllStatuses returns health status for all monitored agents
func (m *Monitor) GetAllStatuses() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	statuses := make(map[string]HealthStatus)
	for id, check := range m.checks {
		statuses[id] = check.status
	}
	
	return statuses
}

func (m *Monitor) runHealthCheck(check *agentCheck) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(check.config.Interval)
	defer ticker.Stop()
	
	// Run initial check
	m.performCheck(check)
	
	for {
		select {
		case <-ticker.C:
			m.performCheck(check)
		case <-check.stopChan:
			return
		case <-m.stopChan:
			return
		}
	}
}

func (m *Monitor) performCheck(check *agentCheck) {
	ctx, cancel := context.WithTimeout(context.Background(), check.config.Timeout)
	defer cancel()
	
	// Get agent info
	agent, err := m.agentMgr.GetAgent(check.agentID)
	if err != nil {
		m.updateStatus(check, false, fmt.Sprintf("Failed to get agent info: %v", err))
		return
	}
	
	// Only check running agents
	if agent.Status != "running" {
		m.StopMonitoring(check.agentID)
		return
	}
	
	// Perform HTTP health check through proxy
	url := fmt.Sprintf("http://localhost:8081/agent/%s%s", check.agentID, check.config.Endpoint)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		m.updateStatus(check, false, fmt.Sprintf("Failed to create request: %v", err))
		return
	}
	
	// Add authorization header for proxy
	req.Header.Set("Authorization", "Bearer agentainer-default-token")
	
	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.updateStatus(check, false, fmt.Sprintf("Health check failed: %v", err))
		m.handleFailure(check)
		return
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		m.updateStatus(check, true, "Health check passed")
	} else {
		m.updateStatus(check, false, fmt.Sprintf("Health check returned status %d", resp.StatusCode))
		m.handleFailure(check)
	}
}

func (m *Monitor) updateStatus(check *agentCheck, healthy bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if healthy {
		check.status.FailureCount = 0
	} else {
		check.status.FailureCount++
	}
	
	check.status.Healthy = healthy
	check.status.LastCheck = time.Now()
	check.status.Message = message
	
	// Store in Redis
	key := fmt.Sprintf("health:%s", check.agentID)
	data, _ := json.Marshal(check.status)
	m.redisClient.Set(context.Background(), key, data, 24*time.Hour)
}

func (m *Monitor) handleFailure(check *agentCheck) {
	// Check if we've exceeded retry count
	if check.status.FailureCount >= check.config.Retries {
		log.Printf("Agent %s failed health check %d times, attempting restart...", 
			check.agentID, check.status.FailureCount)
		
		// Get agent to check if auto-restart is enabled
		agent, err := m.agentMgr.GetAgent(check.agentID)
		if err != nil {
			log.Printf("Failed to get agent info: %v", err)
			return
		}
		
		if agent.AutoRestart {
			// Attempt to restart the agent
			if err := m.agentMgr.Restart(context.Background(), check.agentID); err != nil {
				log.Printf("Failed to restart agent %s: %v", check.agentID, err)
			} else {
				log.Printf("Successfully restarted agent %s", check.agentID)
				// Reset failure count after successful restart
				check.status.FailureCount = 0
			}
		}
	}
}

func (m *Monitor) watchAgentEvents(ctx context.Context) {
	// Subscribe to agent status changes
	pubsub := m.redisClient.Subscribe(ctx, "agent:status:*")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			// Parse agent ID from channel name
			if len(msg.Channel) > 13 { // "agent:status:"
				agentID := msg.Channel[13:]
				
				// Check new status
				if msg.Payload == string(agent.StatusRunning) {
					// Start monitoring
					m.StartMonitoring(agentID, CheckConfig{
						Endpoint: "/health",
						Interval: 30 * time.Second,
						Timeout:  5 * time.Second,
						Retries:  3,
					})
				} else {
					// Stop monitoring
					m.StopMonitoring(agentID)
				}
			}
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		}
	}
}