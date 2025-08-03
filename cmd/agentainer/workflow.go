package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Manage workflows",
	Long:  "Commands for creating and managing Agentainer Flow workflows",
}

var workflowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workflow",
	Long: `Create a new workflow with orchestration capabilities.

Examples:
  # Create a basic workflow
  agentainer workflow create --name data-pipeline --description "Process daily data"

  # Create with specific configuration
  agentainer workflow create --name etl-workflow --max-parallel 10 --timeout 2h`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		maxParallel, _ := cmd.Flags().GetInt("max-parallel")
		timeout, _ := cmd.Flags().GetString("timeout")

		if name == "" {
			fmt.Println("Error: workflow name is required")
			os.Exit(1)
		}

		payload := map[string]interface{}{
			"name":        name,
			"description": description,
			"config": map[string]interface{}{
				"max_parallel": maxParallel,
				"timeout":      timeout,
			},
		}

		apiResp, err := makeAPIRequest("POST", "/workflows", payload)
		if err != nil {
			fmt.Printf("Error creating workflow: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to create workflow: %s\n", apiResp.Message)
			os.Exit(1)
		}

		if data, ok := apiResp.Data.(map[string]interface{}); ok {
			fmt.Printf("Workflow created successfully:\n")
			fmt.Printf("  ID: %s\n", data["id"])
			fmt.Printf("  Name: %s\n", data["name"])
		}
	},
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflows",
	Long:  "List all workflows with their current status",
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")
		
		url := "/workflows"
		if status != "" {
			url += "?status=" + status
		}

		apiResp, err := makeAPIRequest("GET", url, nil)
		if err != nil {
			fmt.Printf("Error listing workflows: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to list workflows: %s\n", apiResp.Message)
			os.Exit(1)
		}

		if workflows, ok := apiResp.Data.([]interface{}); ok {
			if len(workflows) == 0 {
				fmt.Println("No workflows found")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCREATED")

			for _, wf := range workflows {
				if workflow, ok := wf.(map[string]interface{}); ok {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						workflow["id"],
						workflow["name"],
						workflow["status"],
						workflow["created_at"],
					)
				}
			}
			w.Flush()
		}
	},
}

var workflowGetCmd = &cobra.Command{
	Use:   "get [workflow-id]",
	Short: "Get workflow details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowID := args[0]

		apiResp, err := makeAPIRequest("GET", fmt.Sprintf("/workflows/%s", workflowID), nil)
		if err != nil {
			fmt.Printf("Error getting workflow: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to get workflow: %s\n", apiResp.Message)
			os.Exit(1)
		}

		fmt.Println(apiResp.Data)
	},
}

var workflowStartCmd = &cobra.Command{
	Use:   "start [workflow-id]",
	Short: "Start workflow execution",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowID := args[0]

		apiResp, err := makeAPIRequest("POST", fmt.Sprintf("/workflows/%s/start", workflowID), nil)
		if err != nil {
			fmt.Printf("Error starting workflow: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to start workflow: %s\n", apiResp.Message)
			os.Exit(1)
		}

		fmt.Printf("Workflow %s started successfully\n", workflowID)
	},
}

var workflowJobsCmd = &cobra.Command{
	Use:   "jobs [workflow-id]",
	Short: "List all jobs for a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowID := args[0]

		apiResp, err := makeAPIRequest("GET", fmt.Sprintf("/workflows/%s/jobs", workflowID), nil)
		if err != nil {
			fmt.Printf("Error getting workflow jobs: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to get workflow jobs: %s\n", apiResp.Message)
			os.Exit(1)
		}

		if agents, ok := apiResp.Data.([]interface{}); ok {
			if len(agents) == 0 {
				fmt.Println("No jobs found for this workflow")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "AGENT ID\tNAME\tSTATUS\tSTEP ID\tTASK ID")

			for _, agent := range agents {
				if a, ok := agent.(map[string]interface{}); ok {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						a["id"],
						a["name"],
						a["status"],
						a["step_id"],
						a["task_id"],
					)
				}
			}
			w.Flush()
		}
	},
}

var workflowMapReduceCmd = &cobra.Command{
	Use:   "mapreduce",
	Short: "Create and run a MapReduce workflow",
	Long: `Create and run a MapReduce workflow with simplified configuration.

Examples:
  # Basic MapReduce workflow
  agentainer workflow mapreduce --name process-docs \
    --mapper web-scraper:latest --reducer analyzer:latest \
    --parallel 10

  # With pooling for better performance
  agentainer workflow mapreduce --name fast-processing \
    --mapper processor:v2 --reducer aggregator:v2 \
    --parallel 20 --pool-size 5`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		mapperImage, _ := cmd.Flags().GetString("mapper")
		reducerImage, _ := cmd.Flags().GetString("reducer")
		parallel, _ := cmd.Flags().GetInt("parallel")
		poolSize, _ := cmd.Flags().GetInt("pool-size")
		timeout, _ := cmd.Flags().GetString("timeout")

		if name == "" || mapperImage == "" || reducerImage == "" {
			fmt.Println("Error: name, mapper, and reducer are required")
			os.Exit(1)
		}

		payload := map[string]interface{}{
			"name":          name,
			"mapper_image":  mapperImage,
			"reducer_image": reducerImage,
			"max_parallel":  parallel,
			"pool_size":     poolSize,
			"timeout":       timeout,
		}

		apiResp, err := makeAPIRequest("POST", "/workflows/mapreduce", payload)
		if err != nil {
			fmt.Printf("Error creating MapReduce workflow: %v\n", err)
			os.Exit(1)
		}

		if !apiResp.Success {
			fmt.Printf("Failed to create MapReduce workflow: %s\n", apiResp.Message)
			os.Exit(1)
		}

		if data, ok := apiResp.Data.(map[string]interface{}); ok {
			fmt.Printf("MapReduce workflow created and started:\n")
			fmt.Printf("  Workflow ID: %s\n", data["id"])
			fmt.Printf("  Name: %s\n", data["name"])
			fmt.Printf("  Status: %s\n", data["status"])
			fmt.Printf("\nTrack progress with: agentainer workflow get %s\n", data["id"])
			fmt.Printf("View jobs with: agentainer workflow jobs %s\n", data["id"])
		}
	},
}

func init() {
	// Create workflow command flags
	workflowCreateCmd.Flags().StringP("name", "n", "", "Workflow name (required)")
	workflowCreateCmd.Flags().StringP("description", "d", "", "Workflow description")
	workflowCreateCmd.Flags().IntP("max-parallel", "p", 5, "Maximum parallel executions")
	workflowCreateCmd.Flags().StringP("timeout", "t", "1h", "Workflow timeout")
	workflowCreateCmd.MarkFlagRequired("name")

	// List workflow command flags
	workflowListCmd.Flags().StringP("status", "s", "", "Filter by status (pending, running, completed, failed)")

	// MapReduce command flags
	workflowMapReduceCmd.Flags().StringP("name", "n", "", "Workflow name (required)")
	workflowMapReduceCmd.Flags().StringP("mapper", "m", "", "Mapper image (required)")
	workflowMapReduceCmd.Flags().StringP("reducer", "r", "", "Reducer image (required)")
	workflowMapReduceCmd.Flags().IntP("parallel", "p", 10, "Maximum parallel tasks")
	workflowMapReduceCmd.Flags().IntP("pool-size", "", 5, "Agent pool size")
	workflowMapReduceCmd.Flags().StringP("timeout", "t", "30m", "Workflow timeout")
	workflowMapReduceCmd.MarkFlagRequired("name")
	workflowMapReduceCmd.MarkFlagRequired("mapper")
	workflowMapReduceCmd.MarkFlagRequired("reducer")

	// Add subcommands
	workflowCmd.AddCommand(workflowCreateCmd)
	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowGetCmd)
	workflowCmd.AddCommand(workflowStartCmd)
	workflowCmd.AddCommand(workflowJobsCmd)
	workflowCmd.AddCommand(workflowMapReduceCmd)

	// Add workflow command to root
	rootCmd.AddCommand(workflowCmd)
}