package workflow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/go-redis/redis/v8"
)

// PooledAgentState represents the state of an agent in the pool
type PooledAgentState string

const (
	StateIdle        PooledAgentState = "idle"
	StateActive      PooledAgentState = "active"
	StateTerminating PooledAgentState = "terminating"
)

// PooledAgent represents an agent in the pool
type PooledAgent struct {
	Agent        *agent.Agent
	Pool         *AgentPool
	State        PooledAgentState
	LastUsed     time.Time
	UsageCount   int
	HealthStatus bool
}

// AgentPool manages a pool of reusable agents
type AgentPool struct {
	ID           string
	Image        string
	Config       *PoolConfig
	IdleAgents   chan *PooledAgent
	ActiveAgents sync.Map
	Metrics      *PoolMetrics
	
	agentManager *agent.Manager
	redisClient  *redis.Client
	mu           sync.RWMutex
	currentSize  int
	stats        PoolStats
}

// PoolMetrics tracks pool performance
type PoolMetrics struct {
	RequestsTotal    int64
	RequestsActive   int64
	QueueDepth       int64
	AvgWaitTime      time.Duration
	AvgExecutionTime time.Duration
	UtilizationRate  float64
	mu               sync.RWMutex
}

// PoolStats represents pool statistics
type PoolStats struct {
	TotalAgents  int
	ActiveAgents int
	IdleAgents   int
	TotalUses    int64
}

// PoolManager manages multiple agent pools
type PoolManager struct {
	pools        sync.Map
	agentManager *agent.Manager
	redisClient  *redis.Client
}

// NewPoolManager creates a new pool manager
func NewPoolManager(agentManager *agent.Manager, redisClient *redis.Client) *PoolManager {
	pm := &PoolManager{
		agentManager: agentManager,
		redisClient:  redisClient,
	}

	// Start background cleanup worker
	go pm.cleanupWorker()

	return pm
}

// GetOrCreatePool gets an existing pool or creates a new one
func (pm *PoolManager) GetOrCreatePool(ctx context.Context, image string, config *PoolConfig) (*AgentPool, error) {
	poolID := fmt.Sprintf("pool-%s", image)
	
	// Check if pool already exists
	if poolInterface, exists := pm.pools.Load(poolID); exists {
		return poolInterface.(*AgentPool), nil
	}

	// Create new pool
	pool := &AgentPool{
		ID:           poolID,
		Image:        image,
		Config:       config,
		IdleAgents:   make(chan *PooledAgent, config.MaxSize),
		Metrics:      &PoolMetrics{},
		agentManager: pm.agentManager,
		redisClient:  pm.redisClient,
	}

	// Store pool
	pm.pools.Store(poolID, pool)

	// Initialize pool with minimum agents
	if err := pool.initialize(ctx); err != nil {
		pm.pools.Delete(poolID)
		return nil, fmt.Errorf("failed to initialize pool: %w", err)
	}

	// Start pool maintenance workers
	go pool.idleCleanupWorker()
	go pool.healthCheckWorker()

	return pool, nil
}

// initialize creates the minimum number of agents for the pool
func (p *AgentPool) initialize(ctx context.Context) error {
	for i := 0; i < p.Config.MinSize; i++ {
		if err := p.createNewAgent(ctx); err != nil {
			return fmt.Errorf("failed to create agent %d: %w", i, err)
		}
	}
	return nil
}

// createNewAgent creates a new agent and adds it to the pool
func (p *AgentPool) createNewAgent(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.currentSize >= p.Config.MaxSize {
		return fmt.Errorf("pool at maximum size")
	}

	// Deploy new agent
	agentObj, err := p.agentManager.Deploy(
		ctx,
		fmt.Sprintf("%s-pooled-%d", p.ID, time.Now().Unix()),
		p.Image,
		nil, // env vars
		0,   // cpu limit
		0,   // memory limit
		false, // auto restart
		"",   // token
		nil,  // ports
		nil,  // volumes
		nil,  // health check
	)
	if err != nil {
		return fmt.Errorf("failed to deploy agent: %w", err)
	}

	// Start the agent
	if err := p.agentManager.Start(ctx, agentObj.ID); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Create pooled agent
	pooledAgent := &PooledAgent{
		Agent:        agentObj,
		Pool:         p,
		State:        StateIdle,
		LastUsed:     time.Now(),
		UsageCount:   0,
		HealthStatus: true,
	}

	// Add to idle pool
	select {
	case p.IdleAgents <- pooledAgent:
		p.currentSize++
		p.stats.TotalAgents++
	default:
		// Pool is full, terminate the agent
		p.terminateAgent(ctx, pooledAgent)
		return fmt.Errorf("pool channel is full")
	}

	return nil
}

// GetAgent retrieves an agent from the pool
func (p *AgentPool) GetAgent(ctx context.Context) (*PooledAgent, error) {
	startTime := time.Now()
	p.Metrics.mu.Lock()
	p.Metrics.RequestsTotal++
	p.Metrics.RequestsActive++
	p.Metrics.mu.Unlock()

	select {
	case agent := <-p.IdleAgents:
		// Got an idle agent
		agent.State = StateActive
		agent.LastUsed = time.Now()
		p.ActiveAgents.Store(agent.Agent.ID, agent)
		
		// Update stats
		p.mu.Lock()
		p.stats.ActiveAgents++
		p.stats.TotalUses++
		p.mu.Unlock()
		
		// Update metrics
		p.Metrics.mu.Lock()
		p.Metrics.AvgWaitTime = time.Since(startTime)
		p.Metrics.mu.Unlock()
		
		return agent, nil
		
	case <-time.After(100 * time.Millisecond):
		// No idle agents available
		p.mu.RLock()
		canCreate := p.currentSize < p.Config.MaxSize
		p.mu.RUnlock()
		
		if canCreate {
			// Create new agent if under limit
			if err := p.createNewAgent(ctx); err != nil {
				return nil, fmt.Errorf("failed to create new agent: %w", err)
			}
			// Retry getting agent
			return p.GetAgent(ctx)
		}
		
		// At capacity
		return nil, fmt.Errorf("pool at capacity")
	}
}

// ReleaseAgent returns an agent to the pool
func (p *AgentPool) ReleaseAgent(agent *PooledAgent) {
	agent.UsageCount++
	
	// Update stats
	p.mu.Lock()
	p.stats.ActiveAgents--
	p.mu.Unlock()
	
	// Check if agent should be retired
	if agent.UsageCount >= p.Config.MaxAgentUses {
		log.Printf("Retiring agent %s after %d uses", agent.Agent.ID, agent.UsageCount)
		go p.terminateAgent(context.Background(), agent)
		return
	}
	
	// Return to pool for reuse
	agent.State = StateIdle
	agent.LastUsed = time.Now()
	p.ActiveAgents.Delete(agent.Agent.ID)
	
	select {
	case p.IdleAgents <- agent:
		// Successfully returned to pool
	default:
		// Pool is full, terminate agent
		go p.terminateAgent(context.Background(), agent)
	}
	
	// Update metrics
	p.Metrics.mu.Lock()
	p.Metrics.RequestsActive--
	p.updateUtilization()
	p.Metrics.mu.Unlock()
}

// WarmUp pre-creates agents to minimize cold starts
func (p *AgentPool) WarmUp(ctx context.Context) error {
	p.mu.RLock()
	current := p.currentSize
	target := p.Config.MinSize
	p.mu.RUnlock()
	
	for i := current; i < target; i++ {
		if err := p.createNewAgent(ctx); err != nil {
			return fmt.Errorf("failed to warm up pool: %w", err)
		}
	}
	
	return nil
}

// terminateAgent gracefully terminates an agent
func (p *AgentPool) terminateAgent(ctx context.Context, agent *PooledAgent) error {
	agent.State = StateTerminating
	
	// Stop the agent
	if err := p.agentManager.Stop(ctx, agent.Agent.ID); err != nil {
		log.Printf("Failed to stop agent %s: %v", agent.Agent.ID, err)
	}
	
	// Remove the agent
	if err := p.agentManager.Remove(ctx, agent.Agent.ID); err != nil {
		log.Printf("Failed to remove agent %s: %v", agent.Agent.ID, err)
	}
	
	p.mu.Lock()
	p.currentSize--
	p.mu.Unlock()
	
	return nil
}

// idleCleanupWorker removes idle agents that have exceeded timeout
func (p *AgentPool) idleCleanupWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		p.cleanupIdleAgents()
	}
}

// GetStats returns pool statistics
func (p *AgentPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return PoolStats{
		TotalAgents:  p.currentSize,
		ActiveAgents: p.stats.ActiveAgents,
		IdleAgents:   p.currentSize - p.stats.ActiveAgents,
		TotalUses:    p.stats.TotalUses,
	}
}

// cleanupIdleAgents removes agents that have been idle too long
func (p *AgentPool) cleanupIdleAgents() {
	idleTimeout, _ := time.ParseDuration(p.Config.IdleTimeout)
	if idleTimeout == 0 {
		idleTimeout = 5 * time.Minute
	}
	
	now := time.Now()
	
	// Check idle agents
	var toTerminate []*PooledAgent
	
	// Temporarily store idle agents
	var tempAgents []*PooledAgent
	
	// Drain idle agents channel
	for {
		select {
		case agent := <-p.IdleAgents:
			if now.Sub(agent.LastUsed) > idleTimeout {
				toTerminate = append(toTerminate, agent)
			} else {
				tempAgents = append(tempAgents, agent)
			}
		default:
			// No more agents in channel
			goto done
		}
	}
	
done:
	// Put non-terminated agents back
	for _, agent := range tempAgents {
		select {
		case p.IdleAgents <- agent:
		default:
			// Channel full, terminate
			toTerminate = append(toTerminate, agent)
		}
	}
	
	// Terminate idle agents
	for _, agent := range toTerminate {
		log.Printf("Terminating agent %s due to idle timeout", agent.Agent.ID)
		go p.terminateAgent(context.Background(), agent)
	}
}

// healthCheckWorker monitors agent health
func (p *AgentPool) healthCheckWorker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		p.checkAgentHealth()
	}
}

// checkAgentHealth verifies all agents are healthy
func (p *AgentPool) checkAgentHealth() {
	// Check active agents
	p.ActiveAgents.Range(func(key, value interface{}) bool {
		agent := value.(*PooledAgent)
		if !p.isHealthy(agent) {
			log.Printf("Terminating unhealthy agent %s", agent.Agent.ID)
			go p.terminateAgent(context.Background(), agent)
			p.ActiveAgents.Delete(key)
		}
		return true
	})
}

// isHealthy checks if an agent is healthy
func (p *AgentPool) isHealthy(agent *PooledAgent) bool {
	// Check if agent status is running
	currentAgent, err := p.agentManager.GetAgent(agent.Agent.ID)
	if err != nil || currentAgent.Status != "running" {
		return false
	}
	
	// Additional health checks can be added here
	return true
}

// updateUtilization calculates current pool utilization
func (p *AgentPool) updateUtilization() {
	activeCount := 0
	p.ActiveAgents.Range(func(_, _ interface{}) bool {
		activeCount++
		return true
	})
	
	p.mu.RLock()
	total := p.currentSize
	p.mu.RUnlock()
	
	if total > 0 {
		p.Metrics.UtilizationRate = float64(activeCount) / float64(total)
	}
}

// cleanupWorker runs periodic cleanup across all pools
func (pm *PoolManager) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		pm.pools.Range(func(key, value interface{}) bool {
			pool := value.(*AgentPool)
			pool.cleanupIdleAgents()
			return true
		})
	}
}

// GetAllPools returns all active pools
func (pm *PoolManager) GetAllPools() map[string]*AgentPool {
	pools := make(map[string]*AgentPool)
	pm.pools.Range(func(key, value interface{}) bool {
		pools[key.(string)] = value.(*AgentPool)
		return true
	})
	return pools
}

// GetConfig returns the pool configuration
func (p *AgentPool) GetConfig() *PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Config
}