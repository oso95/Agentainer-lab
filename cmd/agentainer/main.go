package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"
	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/agentainer/agentainer-lab/internal/api"
	"github.com/agentainer/agentainer-lab/internal/config"
	"github.com/agentainer/agentainer-lab/internal/requests"
	"github.com/agentainer/agentainer-lab/internal/storage"
	"github.com/agentainer/agentainer-lab/pkg/docker"
	"github.com/agentainer/agentainer-lab/pkg/metrics"
)

var (
	cfgFile string
	cfg     *config.Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "agentainer",
	Short: "Agentainer - LLM Agent Container Runtime",
	Long: `Agentainer Lab - A proof-of-concept runtime for deploying and managing LLM-based agents as containerized microservices.

Features:
  • Auto-port assignment (9000-9999 range) for seamless deployment
  • Proxy routing: Access agents via http://localhost:8081/agent/{id}/
  • Persistent storage with volume mounting for stateful agents
  • Unified resume command works for any stopped/paused/failed agent
  • Full lifecycle management: deploy, start, stop, pause, resume, restart, remove
  • REST API and CLI interface for programmatic control

Quick Start:
  1. Start the server:     agentainer server
  2. Deploy an agent:      agentainer deploy --name my-agent --image nginx:latest
  3. Start the agent:      agentainer start <agent-id>
  4. Access via proxy:     http://localhost:8081/agent/<agent-id>/

For more examples: agentainer deploy --help`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Agentainer server",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an agent from a Docker image",
	Long: `Deploy an agent container with optional persistent storage and environment configuration.

Examples:
  # Basic deployment
  agentainer deploy --name my-agent --image nginx:latest

  # With persistent storage for stateful agents
  agentainer deploy --name ai-agent --image my-ai:latest --volume ./agent-data:/app/data

  # Full configuration with environment variables
  agentainer deploy --name production-agent --image my-app:latest \
    --volume ./data:/app/data --volume ./config:/app/config:ro \
    --env API_KEY=secret --env DEBUG=false \
    --cpu 1000000000 --memory 536870912 --auto-restart

Agent Access:
  All agents are accessed through the secure proxy:
  • Proxy: http://localhost:8081/agent/<agent-id>/
  • API:   http://localhost:8081/agents/<agent-id>
  
Volume Formats:
  • host:container        (read-write)
  • host:container:ro     (read-only)
  • ./relative/path       (relative to current directory)
  • /absolute/path        (absolute path)
  
Note: Agents run in an isolated network. Direct port access is disabled for security.`,
	Run: func(cmd *cobra.Command, args []string) {
		deployAgent(cmd)
	},
}

var startCmd = &cobra.Command{
	Use:   "start [agent-id]",
	Short: "Start an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		startAgent(args[0])
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [agent-id]",
	Short: "Stop an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		stopAgent(args[0])
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [agent-id]",
	Short: "Restart an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		restartAgent(args[0])
	},
}

var pauseCmd = &cobra.Command{
	Use:   "pause [agent-id]",
	Short: "Pause an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pauseAgent(args[0])
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume [agent-id]",
	Short: "Resume an agent (works for paused, stopped, failed, or created agents)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resumeAgent(args[0])
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [agent-id]",
	Short: "View agent logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viewLogs(cmd, args[0])
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Run: func(cmd *cobra.Command, args []string) {
		listAgents()
	},
}

var invokeCmd = &cobra.Command{
	Use:   "invoke [agent-id]",
	Short: "Invoke an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		invokeAgent(args[0])
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [agent-id]",
	Short: "Remove an agent (stops container and deletes from system)",
	Long: `Remove an agent completely from the system. This will:
  • Stop the container if it's running
  • Remove the container from Docker
  • Delete the agent from storage
  • Clean up cache entries

This operation is irreversible. Use with caution.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeAgent(args[0])
	},
}

var requestsCmd = &cobra.Command{
	Use:   "requests [agent-id]",
	Short: "View pending requests for an agent",
	Long: `View and manage persisted requests for an agent.

This shows requests that were sent to the agent while it was not running,
and are queued for replay when the agent starts.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viewRequests(args[0])
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.agentainer/config.yaml)")

	deployCmd.Flags().StringP("image", "i", "", "Docker image name (required)")
	deployCmd.Flags().StringP("name", "n", "", "Agent name (required)")
	deployCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables (key=value)")
	deployCmd.Flags().Int64P("cpu", "c", 0, "CPU limit (nanocpus)")
	deployCmd.Flags().Int64P("memory", "m", 0, "Memory limit (bytes)")
	deployCmd.Flags().BoolP("auto-restart", "r", false, "Auto-restart on crash")
	deployCmd.Flags().StringP("token", "t", "", "Agent token")
	deployCmd.Flags().StringSliceP("port", "p", []string{}, "DEPRECATED: Port mappings are no longer supported. All access is through proxy.")
	deployCmd.Flags().StringSliceP("volume", "v", []string{}, "Volume mappings (host:container[:ro], e.g., ./data:/app/data or ./config:/app/config:ro)")
	deployCmd.MarkFlagRequired("image")
	deployCmd.MarkFlagRequired("name")

	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(invokeCmd)
	rootCmd.AddCommand(requestsCmd)
}

func runServer() {
	ctx := context.Background()

	dockerClient, err := docker.NewClient(cfg.Docker.Host)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	storage := storage.NewStorage(redisClient)
	agentMgr := agent.NewManager(dockerClient, redisClient, cfg.GetAgentConfigPath())
	metricsCollector := metrics.NewCollector(dockerClient, storage)

	server := api.NewServer(cfg, agentMgr, storage, metricsCollector, redisClient)

	// Start replay worker if request persistence is enabled
	if cfg.Features.RequestPersistence {
		requestMgr := requests.NewManager(redisClient)
		replayWorker := requests.NewReplayWorker(requestMgr, redisClient)
		go replayWorker.Start(ctx)
		defer replayWorker.Stop()
		
		log.Println("Request persistence and replay enabled")
	}

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")
	dockerClient.Close()
	redisClient.Close()
}

func deployAgent(cmd *cobra.Command) {
	image, _ := cmd.Flags().GetString("image")
	name, _ := cmd.Flags().GetString("name")
	envVars, _ := cmd.Flags().GetStringSlice("env")
	cpuLimit, _ := cmd.Flags().GetInt64("cpu")
	memoryLimit, _ := cmd.Flags().GetInt64("memory")
	autoRestart, _ := cmd.Flags().GetBool("auto-restart")
	token, _ := cmd.Flags().GetString("token")
	portMappings, _ := cmd.Flags().GetStringSlice("port")
	volumeMappings, _ := cmd.Flags().GetStringSlice("volume")

	if token == "" {
		token = cfg.Security.DefaultToken
	}

	envMap := make(map[string]string)
	for _, env := range envVars {
		if len(env) > 0 {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
	}

	ports, err := parsePortMappings(portMappings)
	if err != nil {
		log.Fatalf("Failed to parse port mappings: %v", err)
	}

	volumes, err := parseVolumeMappings(volumeMappings)
	if err != nil {
		log.Fatalf("Failed to parse volume mappings: %v", err)
	}

	dockerClient, err := docker.NewClient(cfg.Docker.Host)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	agentMgr := agent.NewManager(dockerClient, redisClient, cfg.GetAgentConfigPath())

	agentObj, err := agentMgr.Deploy(context.Background(), name, image, envMap, cpuLimit, memoryLimit, autoRestart, token, ports, volumes)
	if err != nil {
		log.Fatalf("Failed to deploy agent: %v", err)
	}

	fmt.Printf("Agent deployed successfully!\n")
	fmt.Printf("ID: %s\n", agentObj.ID)
	fmt.Printf("Name: %s\n", agentObj.Name)
	fmt.Printf("Image: %s\n", agentObj.Image)
	fmt.Printf("Status: %s\n", agentObj.Status)
	
	// In the new architecture, all access is through the proxy
	fmt.Printf("\nAccess:\n")
	fmt.Printf("  Proxy: http://localhost:%d/agent/%s/\n", cfg.Server.Port, agentObj.ID)
	fmt.Printf("  API:   http://localhost:%d/agents/%s\n", cfg.Server.Port, agentObj.ID)
	if len(agentObj.Volumes) > 0 {
		fmt.Printf("Volume mappings:\n")
		for _, volume := range agentObj.Volumes {
			readOnlyStr := ""
			if volume.ReadOnly {
				readOnlyStr = " (read-only)"
			}
			fmt.Printf("  %s:%s%s\n", volume.HostPath, volume.ContainerPath, readOnlyStr)
		}
	}
}

func startAgent(agentID string) {
	agentMgr := createAgentManager()
	
	if err := agentMgr.Start(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
	
	fmt.Printf("Agent %s started successfully\n", agentID)
}

func stopAgent(agentID string) {
	agentMgr := createAgentManager()
	
	if err := agentMgr.Stop(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to stop agent: %v", err)
	}
	
	fmt.Printf("Agent %s stopped successfully\n", agentID)
}

func restartAgent(agentID string) {
	agentMgr := createAgentManager()
	
	if err := agentMgr.Restart(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to restart agent: %v", err)
	}
	
	fmt.Printf("Agent %s restarted successfully\n", agentID)
}

func pauseAgent(agentID string) {
	agentMgr := createAgentManager()
	
	if err := agentMgr.Pause(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to pause agent: %v", err)
	}
	
	fmt.Printf("Agent %s paused successfully\n", agentID)
}

func resumeAgent(agentID string) {
	agentMgr := createAgentManager()
	
	if err := agentMgr.Resume(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to resume agent: %v", err)
	}
	
	fmt.Printf("Agent %s resumed successfully\n", agentID)
}

func removeAgent(agentID string) {
	agentMgr := createAgentManager()
	
	// Get agent info before removal for confirmation
	agent, err := agentMgr.GetAgent(agentID)
	if err != nil {
		log.Fatalf("Failed to find agent: %v", err)
	}
	
	fmt.Printf("Removing agent '%s' (ID: %s, Status: %s)\n", agent.Name, agentID, agent.Status)
	
	if err := agentMgr.Remove(context.Background(), agentID); err != nil {
		log.Fatalf("Failed to remove agent: %v", err)
	}
	
	fmt.Printf("Agent %s removed successfully\n", agentID)
}

func viewLogs(cmd *cobra.Command, agentID string) {
	follow, _ := cmd.Flags().GetBool("follow")
	
	agentMgr := createAgentManager()
	
	logs, err := agentMgr.GetLogs(context.Background(), agentID, follow)
	if err != nil {
		log.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()

	buf := make([]byte, 1024)
	for {
		n, err := logs.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading logs: %v", err)
			}
			break
		}
		fmt.Print(string(buf[:n]))
	}
}

func listAgents() {
	agentMgr := createAgentManager()
	
	// CLI lists all agents regardless of token
	agents, err := agentMgr.ListAgents("")
	if err != nil {
		log.Fatalf("Failed to list agents: %v", err)
	}

	if len(agents) == 0 {
		fmt.Println("No agents found")
		return
	}

	fmt.Printf("%-20s %-20s %-30s %-10s\n", "ID", "NAME", "IMAGE", "STATUS")
	fmt.Println(strings.Repeat("-", 80))
	
	for _, agentObj := range agents {
		fmt.Printf("%-20s %-20s %-30s %-10s\n", agentObj.ID, agentObj.Name, agentObj.Image, agentObj.Status)
		if agentObj.Status == agent.StatusRunning {
			fmt.Printf("  → Access: http://localhost:%d/agent/%s/\n", cfg.Server.Port, agentObj.ID)
		}
	}
}

func invokeAgent(agentID string) {
	agentMgr := createAgentManager()
	
	agentObj, err := agentMgr.GetAgent(agentID)
	if err != nil {
		log.Fatalf("Failed to get agent: %v", err)
	}

	if agentObj.Status != agent.StatusRunning {
		log.Fatalf("Agent is not running (status: %s)", agentObj.Status)
	}

	fmt.Printf("Agent %s invoked successfully\n", agentID)
}

func createAgentManager() *agent.Manager {
	dockerClient, err := docker.NewClient(cfg.Docker.Host)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return agent.NewManager(dockerClient, redisClient, cfg.GetAgentConfigPath())
}

func parsePortMappings(portMappings []string) ([]agent.PortMapping, error) {
	var ports []agent.PortMapping
	
	for _, mapping := range portMappings {
		if mapping == "" {
			continue
		}
		
		// Parse format: host:container/protocol or host:container (default tcp)
		parts := strings.Split(mapping, "/")
		protocol := "tcp"
		if len(parts) == 2 {
			protocol = parts[1]
		}
		
		portParts := strings.Split(parts[0], ":")
		if len(portParts) != 2 {
			return nil, fmt.Errorf("invalid port mapping format: %s (expected host:container or host:container/protocol)", mapping)
		}
		
		hostPort, err := strconv.Atoi(portParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid host port: %s", portParts[0])
		}
		
		containerPort, err := strconv.Atoi(portParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid container port: %s", portParts[1])
		}
		
		ports = append(ports, agent.PortMapping{
			HostPort:      hostPort,
			ContainerPort: containerPort,
			Protocol:      protocol,
		})
	}
	
	return ports, nil
}

func parseVolumeMappings(volumeMappings []string) ([]agent.VolumeMapping, error) {
	var volumes []agent.VolumeMapping
	
	for _, mapping := range volumeMappings {
		if mapping == "" {
			continue
		}
		
		// Parse format: host:container or host:container:ro
		parts := strings.Split(mapping, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("invalid volume mapping format: %s (expected host:container or host:container:ro)", mapping)
		}
		
		hostPath := parts[0]
		containerPath := parts[1]
		readOnly := false
		
		if len(parts) == 3 && parts[2] == "ro" {
			readOnly = true
		}
		
		if hostPath == "" || containerPath == "" {
			return nil, fmt.Errorf("invalid volume mapping: host and container paths cannot be empty")
		}
		
		volumes = append(volumes, agent.VolumeMapping{
			HostPath:      hostPath,
			ContainerPath: containerPath,
			ReadOnly:      readOnly,
		})
	}
	
	return volumes, nil
}

func viewRequests(agentID string) {
	
	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Make API request to get pending requests
	url := fmt.Sprintf("http://localhost:%d/agents/%s/requests", cfg.Server.Port, agentID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+cfg.Security.DefaultToken)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to get requests: %v", err)
	}
	defer resp.Body.Close()
	
	// Parse response
	var apiResp api.Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}
	
	if !apiResp.Success {
		log.Fatalf("API error: %s", apiResp.Message)
	}
	
	// Display requests
	data := apiResp.Data.(map[string]interface{})
	pendingReqs := data["pending"].([]interface{})
	
	if len(pendingReqs) == 0 {
		fmt.Printf("No pending requests for agent %s\n", agentID)
		return
	}
	
	fmt.Printf("Pending requests for agent %s:\n", agentID)
	fmt.Println(strings.Repeat("-", 80))
	
	for _, req := range pendingReqs {
		r := req.(map[string]interface{})
		fmt.Printf("ID: %s\n", r["id"])
		fmt.Printf("Method: %s %s\n", r["method"], r["path"])
		fmt.Printf("Status: %s\n", r["status"])
		fmt.Printf("Created: %s\n", r["created_at"])
		if retries, ok := r["retry_count"].(float64); ok && retries > 0 {
			fmt.Printf("Retries: %d/%d\n", int(retries), int(r["max_retries"].(float64)))
		}
		fmt.Println(strings.Repeat("-", 80))
	}
}