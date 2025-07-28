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
	"github.com/agentainer/agentainer-lab/internal/backup"
	"github.com/agentainer/agentainer-lab/internal/config"
	"github.com/agentainer/agentainer-lab/internal/logging"
	"github.com/agentainer/agentainer-lab/internal/requests"
	"github.com/agentainer/agentainer-lab/internal/storage"
	"github.com/agentainer/agentainer-lab/internal/sync"
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
    --cpu 1 --memory 512M --auto-restart

  # Deploy from YAML configuration file
  agentainer deploy --config agents.yaml
  agentainer deploy --config ./deployments/production.yaml

Agent Access:
  • Proxy: http://localhost:8081/agent/<agent-id>/   (no auth, direct agent access)
  • API:   http://localhost:8081/agents/<agent-id>   (requires auth, management operations)
  
Resource Limits:
  • CPU:    0.5, 1, 2 (cores) or 500m (millicores)
  • Memory: 512M, 1G, 1.5G (also supports Mi/Gi for k8s compatibility)
  
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

	deployCmd.Flags().StringP("config", "", "", "Deploy from YAML configuration file")
	deployCmd.Flags().StringP("image", "i", "", "Docker image name (required for single deployment)")
	deployCmd.Flags().StringP("name", "n", "", "Agent name (required for single deployment)")
	deployCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables (key=value)")
	deployCmd.Flags().StringP("cpu", "c", "", "CPU limit (e.g., 0.5, 1, 2 for cores)")
	deployCmd.Flags().StringP("memory", "m", "", "Memory limit (e.g., 512M, 2G)")
	deployCmd.Flags().BoolP("auto-restart", "r", false, "Auto-restart on crash")
	deployCmd.Flags().StringP("token", "t", "", "Agent token")
	deployCmd.Flags().StringSliceP("port", "p", []string{}, "DEPRECATED: Port mappings are no longer supported. All access is through proxy.")
	deployCmd.Flags().StringSliceP("volume", "v", []string{}, "Volume mappings (host:container[:ro], e.g., ./data:/app/data or ./config:/app/config:ro)")
	deployCmd.Flags().String("health-endpoint", "/health", "Health check endpoint path")
	deployCmd.Flags().String("health-interval", "30s", "Health check interval")
	deployCmd.Flags().String("health-timeout", "5s", "Health check timeout")
	deployCmd.Flags().Int("health-retries", 3, "Health check retry count before restart")

	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	
	metricsCmd.Flags().BoolP("history", "H", false, "Show metrics history")
	metricsCmd.Flags().StringP("duration", "d", "1h", "History duration (e.g., 30m, 1h, 6h, 24h)")
	
	backupCreateCmd.Flags().StringP("name", "n", "", "Backup name (required)")
	backupCreateCmd.Flags().StringP("description", "d", "", "Backup description")
	backupCreateCmd.Flags().StringSliceP("agents", "a", []string{}, "Specific agents to backup (default: all)")
	backupCreateCmd.MarkFlagRequired("name")
	
	backupRestoreCmd.Flags().StringSliceP("agents", "a", []string{}, "Specific agents to restore (default: all)")
	
	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupDeleteCmd)
	backupCmd.AddCommand(backupExportCmd)
	
	auditCmd.Flags().StringP("user", "u", "", "Filter by user ID")
	auditCmd.Flags().StringP("action", "a", "", "Filter by action")
	auditCmd.Flags().StringP("resource", "r", "", "Filter by resource type")
	auditCmd.Flags().StringP("duration", "d", "24h", "Time duration to query")
	auditCmd.Flags().IntP("limit", "l", 100, "Maximum number of entries to show")

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
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(auditCmd)
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
	
	// Initialize logger
	logger, err := logging.NewLogger(redisClient, "", true) // Console logging enabled
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()
	
	// Set global logger
	logging.SetGlobalLogger(logger)
	
	logging.Info("server", "Agentainer server starting", map[string]interface{}{
		"version": "1.0",
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	})

	server := api.NewServer(cfg, agentMgr, storage, metricsCollector, redisClient, dockerClient)

	// Start state synchronizer with more frequent updates
	stateSynchronizer := sync.NewStateSynchronizer(dockerClient, redisClient, 10*time.Second) // Reduced from 30s to 10s
	if err := stateSynchronizer.Start(ctx); err != nil {
		log.Printf("Failed to start state synchronizer: %v", err)
	} else {
		defer stateSynchronizer.Stop()
		log.Println("State synchronizer started - agents will be automatically synced with Docker containers every 10 seconds")
	}

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
	configFile, _ := cmd.Flags().GetString("config")
	
	// Check if deploying from YAML config file
	if configFile != "" {
		deployFromYAML(configFile)
		return
	}
	
	// Otherwise, deploy single agent from CLI flags
	image, _ := cmd.Flags().GetString("image")
	name, _ := cmd.Flags().GetString("name")
	
	// Validate required flags for single deployment
	if image == "" || name == "" {
		log.Fatal("Either --config or both --name and --image are required")
	}
	
	// Create Docker client
	dockerClient, err := docker.NewClient(cfg.Docker.Host)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	
	// Check if image is actually a Dockerfile
	builder := docker.NewImageBuilder(dockerClient)
	if docker.IsDockerfile(image) {
		fmt.Printf("Detected Dockerfile: %s\n", image)
		
		// Generate unique image name
		generatedImageName := docker.GenerateImageName(name)
		finalImageName, err := builder.PreventDuplicateImage(context.Background(), generatedImageName)
		if err != nil {
			log.Fatalf("Failed to generate unique image name: %v", err)
		}
		
		fmt.Printf("Building Docker image: %s\n", finalImageName)
		
		// Create progress channel for build output
		progressChan := make(chan string, 100)
		buildCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		
		// Start build progress display
		doneChan := make(chan bool)
		go func() {
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinIdx := 0
			lastMsg := ""
			
			for {
				select {
				case msg, ok := <-progressChan:
					if !ok {
						doneChan <- true
						return
					}
					// Clear previous line and print new message
					if lastMsg != "" {
						fmt.Printf("\r%-120s", " ") // Clear line with more space
					}
					
					// Truncate long messages
					displayMsg := msg
					if len(msg) > 100 {
						displayMsg = msg[:97] + "..."
					}
					
					if strings.HasPrefix(msg, "Step ") || strings.HasPrefix(msg, "Successfully ") {
						fmt.Printf("\r%s %s\n", spinner[spinIdx], displayMsg)
						lastMsg = ""
					} else {
						fmt.Printf("\r%s %s", spinner[spinIdx], displayMsg)
						lastMsg = displayMsg
					}
					spinIdx = (spinIdx + 1) % len(spinner)
				case <-time.After(100 * time.Millisecond):
					if lastMsg != "" {
						fmt.Printf("\r%s %s", spinner[spinIdx], lastMsg)
						spinIdx = (spinIdx + 1) % len(spinner)
					}
				}
			}
		}()
		
		// Build the image
		if err := builder.BuildImage(buildCtx, image, finalImageName, progressChan); err != nil {
			<-doneChan
			log.Fatalf("Failed to build Docker image: %v", err)
		}
		
		// Wait for progress display to finish
		<-doneChan
		fmt.Println() // New line after build
		
		// Use the built image for deployment
		image = finalImageName
		fmt.Printf("Using built image: %s\n\n", image)
	}
	
	envVars, _ := cmd.Flags().GetStringSlice("env")
	cpuStr, _ := cmd.Flags().GetString("cpu")
	memoryStr, _ := cmd.Flags().GetString("memory")
	autoRestart, _ := cmd.Flags().GetBool("auto-restart")
	token, _ := cmd.Flags().GetString("token")
	portMappings, _ := cmd.Flags().GetStringSlice("port")
	volumeMappings, _ := cmd.Flags().GetStringSlice("volume")
	healthEndpoint, _ := cmd.Flags().GetString("health-endpoint")
	healthInterval, _ := cmd.Flags().GetString("health-interval")
	healthTimeout, _ := cmd.Flags().GetString("health-timeout")
	healthRetries, _ := cmd.Flags().GetInt("health-retries")
	
	// Parse CPU and memory limits using the same functions as YAML
	var cpuLimit, memoryLimit int64
	if cpuStr != "" {
		cpu, err := config.ParseCPU(cpuStr)
		if err != nil {
			log.Fatalf("Invalid CPU limit: %v", err)
		}
		cpuLimit = cpu
	}
	if memoryStr != "" {
		mem, err := config.ParseMemory(memoryStr)
		if err != nil {
			log.Fatalf("Invalid memory limit: %v", err)
		}
		memoryLimit = mem
	}

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

	// Reuse dockerClient if already created for building
	if dockerClient == nil {
		dockerClient, err = docker.NewClient(cfg.Docker.Host)
		if err != nil {
			log.Fatalf("Failed to create Docker client: %v", err)
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	agentMgr := agent.NewManager(dockerClient, redisClient, cfg.GetAgentConfigPath())

	// Create health check config
	var healthCheck *agent.HealthCheckConfig
	if healthEndpoint != "" {
		healthCheck = &agent.HealthCheckConfig{
			Endpoint: healthEndpoint,
			Interval: healthInterval,
			Timeout:  healthTimeout,
			Retries:  healthRetries,
		}
	}

	agentObj, err := agentMgr.Deploy(context.Background(), name, image, envMap, cpuLimit, memoryLimit, autoRestart, token, ports, volumes, healthCheck)
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
			fmt.Printf("  → Proxy:  http://localhost:%d/agent/%s/\n", cfg.Server.Port, agentObj.ID)
			fmt.Printf("  → API:    http://localhost:%d/agents/%s\n", cfg.Server.Port, agentObj.ID)
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

var healthCmd = &cobra.Command{
	Use:   "health [agent-id]",
	Short: "Get health status of an agent",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			viewAllHealthStatuses()
		} else {
			viewAgentHealth(args[0])
		}
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics [agent-id]",
	Short: "Get resource metrics for an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		history, _ := cmd.Flags().GetBool("history")
		duration, _ := cmd.Flags().GetString("duration")
		
		if history {
			viewMetricsHistory(args[0], duration)
		} else {
			viewCurrentMetrics(args[0])
		}
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup and restore agent configurations",
}

var backupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backup of agents",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		agents, _ := cmd.Flags().GetStringSlice("agents")
		
		createBackup(name, description, agents)
	},
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backups",
	Run: func(cmd *cobra.Command, args []string) {
		listBackups()
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [backup-id]",
	Short: "Restore agents from a backup",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		agents, _ := cmd.Flags().GetStringSlice("agents")
		restoreBackup(args[0], agents)
	},
}

var backupDeleteCmd = &cobra.Command{
	Use:   "delete [backup-id]",
	Short: "Delete a backup",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deleteBackup(args[0])
	},
}

var backupExportCmd = &cobra.Command{
	Use:   "export [backup-id] [output-file]",
	Short: "Export backup as tar.gz file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		exportBackup(args[0], args[1])
	},
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	Run: func(cmd *cobra.Command, args []string) {
		user, _ := cmd.Flags().GetString("user")
		action, _ := cmd.Flags().GetString("action")
		resource, _ := cmd.Flags().GetString("resource")
		duration, _ := cmd.Flags().GetString("duration")
		limit, _ := cmd.Flags().GetInt("limit")
		
		viewAuditLogs(user, action, resource, duration, limit)
	},
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

func deployFromYAML(configFile string) {
	// Load deployment configuration
	deployConfig, err := config.LoadDeploymentConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load deployment config: %v", err)
	}

	fmt.Printf("Deploying agents from: %s\n", configFile)
	fmt.Printf("Deployment: %s\n", deployConfig.Metadata.Name)
	if deployConfig.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", deployConfig.Metadata.Description)
	}
	fmt.Println(strings.Repeat("-", 80))

	// Create managers
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

	// Track deployed agents
	deployedAgents := []struct {
		ID    string
		Name  string
		Image string
	}{}

	// Deploy each agent spec
	for _, spec := range deployConfig.Spec.Agents {
		fmt.Printf("\nDeploying agent: %s\n", spec.Name)
		
		// Convert spec to agent configs (handles replicas)
		agentConfigs, err := spec.ConvertToAgentConfigs()
		if err != nil {
			log.Printf("Failed to convert agent spec %s: %v", spec.Name, err)
			continue
		}

		// Deploy each replica
		for _, agentConfig := range agentConfigs {
			// Use default token if not specified
			token := agentConfig.Token
			if token == "" {
				token = cfg.Security.DefaultToken
			}

			// Empty port mappings (not supported in new architecture)
			var portMappings []agent.PortMapping

			// Deploy the agent
			agentObj, err := agentMgr.Deploy(
				context.Background(),
				agentConfig.Name,
				agentConfig.Image,
				agentConfig.EnvVars,
				agentConfig.CPULimit,
				agentConfig.MemoryLimit,
				agentConfig.AutoRestart,
				token,
				portMappings,
				agentConfig.Volumes,
				agentConfig.HealthCheck,
			)
			if err != nil {
				log.Printf("Failed to deploy %s: %v", agentConfig.Name, err)
				continue
			}

			deployedAgents = append(deployedAgents, struct {
				ID    string
				Name  string
				Image string
			}{
				ID:    agentObj.ID,
				Name:  agentObj.Name,
				Image: agentObj.Image,
			})

			fmt.Printf("  ✓ %s (ID: %s)\n", agentObj.Name, agentObj.ID)
		}
	}

	// Summary
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("\nDeployment Summary:\n")
	fmt.Printf("Total agents deployed: %d\n\n", len(deployedAgents))

	if len(deployedAgents) > 0 {
		fmt.Printf("%-20s %-40s %-30s\n", "NAME", "ID", "IMAGE")
		fmt.Println(strings.Repeat("-", 90))
		for _, agent := range deployedAgents {
			fmt.Printf("%-20s %-40s %-30s\n", agent.Name, agent.ID, agent.Image)
		}

		fmt.Printf("\nAccess all agents through proxy:\n")
		fmt.Printf("  http://localhost:%d/agent/<agent-id>/\n", cfg.Server.Port)
		fmt.Printf("\nStart agents with:\n")
		fmt.Printf("  agentainer start <agent-id>\n")
	}
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
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		fmt.Println("Unexpected response format")
		return
	}
	
	pendingReqs, ok := data["pending"].([]interface{})
	if !ok {
		fmt.Println("No pending requests data available")
		return
	}
	
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

func viewAgentHealth(agentID string) {
	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Make API request to get health status
	url := fmt.Sprintf("http://localhost:%d/agents/%s/health", cfg.Server.Port, agentID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+cfg.Security.DefaultToken)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to get health status: %v", err)
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
	
	// Display health status
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		fmt.Println("Unexpected response format")
		return
	}
	
	fmt.Printf("Health Status for Agent %s:\n", agentID)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Healthy: %v\n", data["healthy"])
	fmt.Printf("Last Check: %s\n", data["last_check"])
	if failCount, ok := data["failure_count"].(float64); ok && failCount > 0 {
		fmt.Printf("Failure Count: %d\n", int(failCount))
	}
	if msg, ok := data["message"].(string); ok && msg != "" {
		fmt.Printf("Message: %s\n", msg)
	}
}

func viewAllHealthStatuses() {
	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Make API request to get all health statuses
	url := fmt.Sprintf("http://localhost:%d/health/agents", cfg.Server.Port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+cfg.Security.DefaultToken)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to get health statuses: %v", err)
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
	
	// Display all health statuses
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok || len(data) == 0 {
		fmt.Println("No agents with health monitoring enabled")
		return
	}
	
	fmt.Println("Agent Health Status Summary:")
	fmt.Printf("%-20s %-10s %-20s %-30s\n", "AGENT ID", "HEALTHY", "FAILURES", "LAST CHECK")
	fmt.Println(strings.Repeat("-", 80))
	
	for agentID, statusData := range data {
		status := statusData.(map[string]interface{})
		healthy := "✓"
		if !status["healthy"].(bool) {
			healthy = "✗"
		}
		failures := int(status["failure_count"].(float64))
		lastCheck := status["last_check"].(string)
		
		fmt.Printf("%-20s %-10s %-20d %-30s\n", agentID, healthy, failures, lastCheck)
	}
}

func viewCurrentMetrics(agentID string) {
	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Make API request to get current metrics
	url := fmt.Sprintf("http://localhost:%d/agents/%s/metrics", cfg.Server.Port, agentID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+cfg.Security.DefaultToken)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to get metrics: %v", err)
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
	
	// Display metrics
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		fmt.Println("No metrics data available")
		return
	}
	
	fmt.Printf("Resource Metrics for Agent %s:\n", agentID)
	fmt.Println(strings.Repeat("=", 60))
	
	// CPU metrics
	if cpu, ok := data["cpu"].(map[string]interface{}); ok {
		fmt.Println("\nCPU:")
		fmt.Printf("  Usage: %.2f%%\n", cpu["usage_percent"])
	}
	
	// Memory metrics
	if mem, ok := data["memory"].(map[string]interface{}); ok {
		fmt.Println("\nMemory:")
		usage := mem["usage"].(float64)
		limit := mem["limit"].(float64)
		fmt.Printf("  Usage: %s / %s (%.2f%%)\n", 
			formatBytes(int64(usage)), 
			formatBytes(int64(limit)),
			mem["usage_percent"])
	}
	
	// Network metrics
	if net, ok := data["network"].(map[string]interface{}); ok {
		fmt.Println("\nNetwork:")
		fmt.Printf("  RX: %s (%d packets)\n", 
			formatBytes(int64(net["rx_bytes"].(float64))),
			int64(net["rx_packets"].(float64)))
		fmt.Printf("  TX: %s (%d packets)\n",
			formatBytes(int64(net["tx_bytes"].(float64))),
			int64(net["tx_packets"].(float64)))
	}
	
	// Disk I/O metrics
	if disk, ok := data["disk"].(map[string]interface{}); ok {
		fmt.Println("\nDisk I/O:")
		fmt.Printf("  Read:  %s\n", formatBytes(int64(disk["read_bytes"].(float64))))
		fmt.Printf("  Write: %s\n", formatBytes(int64(disk["write_bytes"].(float64))))
	}
	
	fmt.Printf("\nTimestamp: %s\n", data["timestamp"])
}

func viewMetricsHistory(agentID, duration string) {
	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Make API request to get metrics history
	url := fmt.Sprintf("http://localhost:%d/agents/%s/metrics/history?duration=%s", 
		cfg.Server.Port, agentID, duration)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+cfg.Security.DefaultToken)
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to get metrics history: %v", err)
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
	
	// Display metrics history
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		fmt.Println("No metrics history available")
		return
	}
	
	fmt.Printf("Metrics History for Agent %s (Duration: %s):\n", agentID, data["duration"])
	fmt.Println(strings.Repeat("=", 80))
	
	metrics, ok := data["metrics"].([]interface{})
	if !ok || len(metrics) == 0 {
		fmt.Println("No metrics data in the specified time range")
		return
	}
	
	// Display summary table
	fmt.Printf("\n%-20s %-10s %-15s %-15s %-15s\n", "TIMESTAMP", "CPU %", "MEMORY", "NET RX", "NET TX")
	fmt.Println(strings.Repeat("-", 80))
	
	for _, metric := range metrics {
		m := metric.(map[string]interface{})
		timestamp := m["timestamp"].(string)
		
		cpu := m["cpu"].(map[string]interface{})
		cpuPercent := cpu["usage_percent"].(float64)
		
		mem := m["memory"].(map[string]interface{})
		memUsage := mem["usage"].(float64)
		memLimit := mem["limit"].(float64)
		memPercent := (memUsage / memLimit) * 100
		
		net := m["network"].(map[string]interface{})
		rxBytes := net["rx_bytes"].(float64)
		txBytes := net["tx_bytes"].(float64)
		
		// Format timestamp to show only time for readability
		t, _ := time.Parse(time.RFC3339, timestamp)
		timeStr := t.Format("15:04:05")
		
		fmt.Printf("%-20s %-10.2f %-15s %-15s %-15s\n",
			timeStr,
			cpuPercent,
			fmt.Sprintf("%s (%.1f%%)", formatBytes(int64(memUsage)), memPercent),
			formatBytes(int64(rxBytes)),
			formatBytes(int64(txBytes)))
	}
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func createBackup(name, description string, agentIDs []string) {
	// Create backup manager
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
	backupMgr := backup.NewManager(agentMgr, redisClient, "")

	// Create backup
	b, err := backupMgr.CreateBackup(context.Background(), name, description, agentIDs)
	if err != nil {
		log.Fatalf("Failed to create backup: %v", err)
	}

	fmt.Printf("Backup created successfully!\n")
	fmt.Printf("ID: %s\n", b.ID)
	fmt.Printf("Name: %s\n", b.Name)
	fmt.Printf("Agents: %d\n", len(b.Agents))
	fmt.Printf("Created: %s\n", b.CreatedAt.Format(time.RFC3339))
}

func listBackups() {
	// Create backup manager
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
	backupMgr := backup.NewManager(agentMgr, redisClient, "")

	// List backups
	backups, err := backupMgr.ListBackups()
	if err != nil {
		log.Fatalf("Failed to list backups: %v", err)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return
	}

	fmt.Printf("%-20s %-30s %-10s %-20s\n", "ID", "NAME", "AGENTS", "CREATED")
	fmt.Println(strings.Repeat("-", 80))

	for _, b := range backups {
		fmt.Printf("%-20s %-30s %-10d %-20s\n", 
			b.ID, 
			b.Name,
			len(b.Agents),
			b.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func restoreBackup(backupID string, agentIDs []string) {
	// Create backup manager
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
	backupMgr := backup.NewManager(agentMgr, redisClient, "")

	// Restore backup
	if err := backupMgr.RestoreBackup(context.Background(), backupID, agentIDs); err != nil {
		log.Fatalf("Failed to restore backup: %v", err)
	}

	fmt.Printf("Backup %s restored successfully!\n", backupID)
}

func deleteBackup(backupID string) {
	// Create backup manager
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
	backupMgr := backup.NewManager(agentMgr, redisClient, "")

	// Delete backup
	if err := backupMgr.DeleteBackup(backupID); err != nil {
		log.Fatalf("Failed to delete backup: %v", err)
	}

	fmt.Printf("Backup %s deleted successfully!\n", backupID)
}

func exportBackup(backupID, outputPath string) {
	// Create backup manager
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
	backupMgr := backup.NewManager(agentMgr, redisClient, "")

	// Export backup
	if err := backupMgr.ExportBackup(backupID, outputPath); err != nil {
		log.Fatalf("Failed to export backup: %v", err)
	}

	fmt.Printf("Backup %s exported to %s\n", backupID, outputPath)
}

func viewAuditLogs(userID, action, resource, durationStr string, limit int) {
	// Parse duration
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Fatalf("Invalid duration: %v", err)
	}
	
	// Create logger to access audit logs
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()
	
	logger, err := logging.NewLogger(redisClient, "", false)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Get audit logs
	filter := logging.AuditFilter{
		Duration: duration,
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Limit:    limit,
	}
	
	logs, err := logger.GetAuditLogs(context.Background(), filter)
	if err != nil {
		log.Fatalf("Failed to get audit logs: %v", err)
	}
	
	if len(logs) == 0 {
		fmt.Println("No audit logs found matching the criteria")
		return
	}
	
	// Display logs
	fmt.Printf("Audit Logs (Last %s):\n", durationStr)
	fmt.Printf("%-20s %-20s %-15s %-20s %-10s %-15s\n", "TIMESTAMP", "USER", "ACTION", "RESOURCE", "RESULT", "IP")
	fmt.Println(strings.Repeat("-", 100))
	
	for _, log := range logs {
		timestamp := log.Timestamp.Format("2006-01-02 15:04:05")
		userDisplay := log.UserID
		if len(userDisplay) > 18 {
			userDisplay = userDisplay[:15] + "..."
		}
		
		resourceDisplay := fmt.Sprintf("%s/%s", log.Resource, log.ResourceID)
		if len(resourceDisplay) > 18 {
			resourceDisplay = resourceDisplay[:15] + "..."
		}
		
		fmt.Printf("%-20s %-20s %-15s %-20s %-10s %-15s\n",
			timestamp,
			userDisplay,
			log.Action,
			resourceDisplay,
			log.Result,
			log.IP)
	}
}