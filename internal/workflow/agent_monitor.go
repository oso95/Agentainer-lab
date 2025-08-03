package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
)

// AgentMonitor provides methods to monitor agent execution
type AgentMonitor struct {
	agentManager *agent.Manager
}

// NewAgentMonitor creates a new agent monitor
func NewAgentMonitor(agentManager *agent.Manager) *AgentMonitor {
	return &AgentMonitor{
		agentManager: agentManager,
	}
}

// WaitForAgentCompletion waits for an agent to complete execution
func (am *AgentMonitor) WaitForAgentCompletion(ctx context.Context, agentID string, timeout time.Duration) (*agent.Agent, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for agent %s to complete", agentID)
		case <-ticker.C:
			agent, err := am.agentManager.GetAgent(agentID)
			if err != nil {
				return nil, fmt.Errorf("failed to get agent status: %w", err)
			}

			// Check if agent has completed
			switch agent.Status {
			case "stopped", "failed":
				return agent, nil
			case "running", "starting":
				// Still running, continue waiting
				continue
			case "created":
				// Not started yet, continue waiting
				continue
			case "paused":
				// Paused, treat as error
				return agent, fmt.Errorf("agent was paused during execution")
			default:
				// Unknown status, treat as error
				return agent, fmt.Errorf("agent in unexpected status: %s", agent.Status)
			}
		}
	}
}

// WaitForMultipleAgents waits for multiple agents to complete
func (am *AgentMonitor) WaitForMultipleAgents(ctx context.Context, agentIDs []string, timeout time.Duration) (map[string]*agent.Agent, error) {
	results := make(map[string]*agent.Agent)
	errors := make(map[string]error)

	// Create a channel to collect results
	type result struct {
		agentID string
		agent   *agent.Agent
		err     error
	}
	resultChan := make(chan result, len(agentIDs))

	// Start monitoring each agent concurrently
	for _, agentID := range agentIDs {
		go func(id string) {
			agent, err := am.WaitForAgentCompletion(ctx, id, timeout)
			resultChan <- result{agentID: id, agent: agent, err: err}
		}(agentID)
	}

	// Collect results
	for i := 0; i < len(agentIDs); i++ {
		res := <-resultChan
		if res.err != nil {
			errors[res.agentID] = res.err
		} else {
			results[res.agentID] = res.agent
		}
	}

	// If any errors occurred, return them
	if len(errors) > 0 {
		return results, fmt.Errorf("some agents failed: %v", errors)
	}

	return results, nil
}

// GetAgentExitCode extracts the exit code from a completed agent
func (am *AgentMonitor) GetAgentExitCode(agent *agent.Agent) (int, error) {
	// This would need to be implemented based on how exit codes are stored
	// For now, return 0 for success, 1 for failure
	if agent.Status == "failed" {
		return 1, nil
	}
	return 0, nil
}