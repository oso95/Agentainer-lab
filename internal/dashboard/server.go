package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/agentainer/agentainer-lab/internal/workflow"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templateFS embed.FS

// DashboardServer serves the monitoring dashboard
type DashboardServer struct {
	addr             string
	redisClient      *redis.Client
	workflowManager  *workflow.Manager
	agentManager     *agent.Manager
	metricsCollector *workflow.MetricsCollector
	websocketHub     *WebSocketHub
	templates        map[string]*template.Template
}

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(addr string, redisClient *redis.Client, agentManager *agent.Manager) (*DashboardServer, error) {
	// Parse templates individually to avoid naming conflicts
	templates := make(map[string]*template.Template)
	
	// Parse base template
	baseContent, err := templateFS.ReadFile("templates/base.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read base template: %w", err)
	}
	
	// Parse each template with its own base
	templateFiles := []string{"dashboard.html", "workflows.html", "metrics.html", "agents.html", "workflow-detail.html"}
	for _, tmplFile := range templateFiles {
		// Create new template instance
		tmpl := template.New(tmplFile)
		
		// Parse base template first
		tmpl, err = tmpl.Parse(string(baseContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse base for %s: %w", tmplFile, err)
		}
		
		// Parse the specific template
		content, err := templateFS.ReadFile("templates/" + tmplFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", tmplFile, err)
		}
		
		tmpl, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", tmplFile, err)
		}
		
		templates[tmplFile] = tmpl
	}

	ds := &DashboardServer{
		addr:             addr,
		redisClient:      redisClient,
		workflowManager:  workflow.NewManager(redisClient),
		agentManager:     agentManager,
		metricsCollector: workflow.NewMetricsCollector(redisClient),
		websocketHub:     NewWebSocketHub(),
		templates:        templates,
	}

	return ds, nil
}

// GetRouter returns the dashboard router for integration into main API server
func (ds *DashboardServer) GetRouter() *mux.Router {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.FileServer(http.FS(staticFS)))

	// Dashboard pages
	r.HandleFunc("/", ds.handleDashboard).Methods("GET")
	r.HandleFunc("/workflows", ds.handleWorkflows).Methods("GET")
	r.HandleFunc("/workflow/{id}", ds.handleWorkflowDetail).Methods("GET")
	r.HandleFunc("/metrics", ds.handleMetrics).Methods("GET")
	r.HandleFunc("/agents", ds.handleAgents).Methods("GET")

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/workflows", ds.apiListWorkflows).Methods("GET")
	api.HandleFunc("/workflow/{id}", ds.apiGetWorkflow).Methods("GET")
	api.HandleFunc("/workflow/{id}/metrics", ds.apiGetWorkflowMetrics).Methods("GET")
	api.HandleFunc("/workflow/{id}/profile", ds.apiGetWorkflowProfile).Methods("GET")
	api.HandleFunc("/metrics/aggregate", ds.apiGetAggregateMetrics).Methods("GET")
	api.HandleFunc("/metrics/realtime", ds.apiGetRealtimeMetrics).Methods("GET")
	api.HandleFunc("/agent-pools", ds.apiGetAgentPools).Methods("GET")
	api.HandleFunc("/agents/active", ds.apiGetActiveAgents).Methods("GET")

	// WebSocket endpoint for real-time updates
	r.HandleFunc("/ws", ds.handleWebSocket)

	return r
}

// Start starts the dashboard server services (WebSocket hub and metrics broadcaster)
func (ds *DashboardServer) Start() error {
	// Start WebSocket hub
	go ds.websocketHub.Run()

	// Start metrics broadcaster
	go ds.broadcastMetrics()
	
	// Start workflow updates listener
	go ds.listenForWorkflowUpdates()

	log.Printf("Dashboard services started")
	return nil
}

// StartStandalone starts the dashboard as a standalone server (for backwards compatibility)
func (ds *DashboardServer) StartStandalone() error {
	r := ds.GetRouter()
	
	// Start services
	if err := ds.Start(); err != nil {
		return err
	}

	fmt.Printf("Dashboard server starting on %s\n", ds.addr)
	return http.ListenAndServe(ds.addr, r)
}

// Dashboard handlers

func (ds *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Time  string
	}{
		Title: "Agentainer Flow Dashboard",
		Time:  time.Now().Format(time.RFC3339),
	}

	log.Printf("Executing dashboard.html template")
	if err := ds.templates["dashboard.html"].ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Error executing dashboard.html: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ds *DashboardServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows, err := ds.workflowManager.ListWorkflows(r.Context(), "")
	if err != nil {
		// Log error but continue with empty list
		log.Printf("Error listing workflows: %v", err)
		workflows = []workflow.Workflow{}
	}

	// Ensure workflows is not nil
	if workflows == nil {
		workflows = []workflow.Workflow{}
	}

	// Convert []Workflow to []*Workflow
	workflowPtrs := make([]*workflow.Workflow, len(workflows))
	for i := range workflows {
		workflowPtrs[i] = &workflows[i]
	}

	data := struct {
		Title     string
		Workflows []*workflow.Workflow
	}{
		Title:     "Workflows",
		Workflows: workflowPtrs,
	}

	if err := ds.templates["workflows.html"].ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ds *DashboardServer) handleWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	wf, err := ds.workflowManager.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	metrics, _ := ds.metricsCollector.GetWorkflowMetrics(workflowID)
	
	// Calculate metrics for template
	var calculatedMetrics *struct {
		TotalSteps     int
		CompletedSteps int
		FailedSteps    int
		Duration       string
	}
	
	if metrics != nil {
		totalSteps := len(metrics.StepMetrics)
		completedSteps := 0
		failedSteps := 0
		
		for _, step := range metrics.StepMetrics {
			switch step.Status {
			case workflow.StepStatusCompleted:
				completedSteps++
			case workflow.StepStatusFailed:
				failedSteps++
			}
		}
		
		duration := "N/A"
		if metrics.Duration != nil {
			duration = metrics.Duration.Round(time.Second).String()
		}
		
		calculatedMetrics = &struct {
			TotalSteps     int
			CompletedSteps int
			FailedSteps    int
			Duration       string
		}{
			TotalSteps:     totalSteps,
			CompletedSteps: completedSteps,
			FailedSteps:    failedSteps,
			Duration:       duration,
		}
	}

	data := struct {
		Title    string
		Workflow *workflow.Workflow
		Metrics  interface{}
	}{
		Title:    fmt.Sprintf("Workflow: %s", wf.Name),
		Workflow: wf,
		Metrics:  calculatedMetrics,
	}

	if err := ds.templates["workflow-detail.html"].ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ds *DashboardServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	duration := 1 * time.Hour
	if d := r.URL.Query().Get("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	aggregates, _ := ds.metricsCollector.GetAggregateMetrics(r.Context(), duration)

	data := struct {
		Title      string
		Duration   string
		Aggregates map[string]interface{}
	}{
		Title:      "Metrics",
		Duration:   duration.String(),
		Aggregates: aggregates,
	}

	if err := ds.templates["metrics.html"].ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ds *DashboardServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement agent listing
	data := struct {
		Title string
	}{
		Title: "Agents",
	}

	if err := ds.templates["agents.html"].ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// API handlers

func (ds *DashboardServer) apiListWorkflows(w http.ResponseWriter, r *http.Request) {
	status := workflow.WorkflowStatus(r.URL.Query().Get("status"))
	workflows, err := ds.workflowManager.ListWorkflows(r.Context(), status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

func (ds *DashboardServer) apiGetWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	wf, err := ds.workflowManager.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func (ds *DashboardServer) apiGetWorkflowMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	metrics, err := ds.metricsCollector.GetWorkflowMetrics(workflowID)
	if err != nil {
		http.Error(w, "Metrics not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (ds *DashboardServer) apiGetAggregateMetrics(w http.ResponseWriter, r *http.Request) {
	duration := 1 * time.Hour
	if d := r.URL.Query().Get("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	aggregates, err := ds.metricsCollector.GetAggregateMetrics(r.Context(), duration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aggregates)
}

func (ds *DashboardServer) apiGetRealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	// Get current metrics snapshot
	snapshot := ds.getMetricsSnapshot()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// WebSocket handling

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

func (ds *DashboardServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &WebSocketClient{
		hub:  ds.websocketHub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	ds.websocketHub.register <- client

	go client.writePump()
	go client.readPump()
}

// broadcastMetrics sends real-time metrics to all connected clients
func (ds *DashboardServer) broadcastMetrics() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot := ds.getMetricsSnapshot()
		data, err := json.Marshal(map[string]interface{}{
			"type":      "metrics_update",
			"timestamp": time.Now(),
			"data":      snapshot,
		})
		if err != nil {
			continue
		}

		ds.websocketHub.broadcast <- data
	}
}

// getMetricsSnapshot returns current metrics snapshot
func (ds *DashboardServer) getMetricsSnapshot() map[string]interface{} {
	ctx := context.Background()

	// Get recent workflows
	workflows, _ := ds.workflowManager.ListWorkflows(ctx, "")
	runningCount := 0
	completedCount := 0
	failedCount := 0

	for _, wf := range workflows {
		switch wf.Status {
		case workflow.WorkflowStatusRunning:
			runningCount++
		case workflow.WorkflowStatusCompleted:
			completedCount++
		case workflow.WorkflowStatusFailed:
			failedCount++
		}
	}

	// Get aggregate metrics for last hour
	aggregates, _ := ds.metricsCollector.GetAggregateMetrics(ctx, 1*time.Hour)

	return map[string]interface{}{
		"workflows": map[string]int{
			"running":   runningCount,
			"completed": completedCount,
			"failed":    failedCount,
			"total":     len(workflows),
		},
		"aggregates": aggregates,
		"timestamp":  time.Now(),
	}
}

// apiGetWorkflowProfile returns performance profile for a workflow
func (ds *DashboardServer) apiGetWorkflowProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	// Create a performance profiler to retrieve the profile
	profiler := workflow.NewPerformanceProfiler(ds.redisClient, ds.metricsCollector)
	profile, err := profiler.GetProfile(r.Context(), workflowID)
	if err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	// Check export format
	format := r.URL.Query().Get("format")
	if format != "" && format != "json" {
		data, err := profiler.ExportProfile(profile, format)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		switch format {
		case "csv":
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"profile-%s.csv\"", workflowID))
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// apiGetAgentPools returns information about agent pools
func (ds *DashboardServer) apiGetAgentPools(w http.ResponseWriter, r *http.Request) {
	// Get pool manager
	poolManager := workflow.NewPoolManager(ds.agentManager, ds.redisClient)
	pools := poolManager.GetAllPools()

	poolInfo := []map[string]interface{}{}
	for image, pool := range pools {
		stats := pool.GetStats()
		poolInfo = append(poolInfo, map[string]interface{}{
			"image": image,
			"config": pool.GetConfig(),
			"stats": stats,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poolInfo)
}

// apiGetActiveAgents returns list of all agents (renamed for compatibility)
func (ds *DashboardServer) apiGetActiveAgents(w http.ResponseWriter, r *http.Request) {
	// Get token from header or use empty string
	token := r.Header.Get("X-Agent-Token")
	
	agents, err := ds.agentManager.ListAgents(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return all agents, not just active ones
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// listenForWorkflowUpdates subscribes to workflow update events and broadcasts them via WebSocket
func (ds *DashboardServer) listenForWorkflowUpdates() {
	ctx := context.Background()
	pubsub := ds.redisClient.Subscribe(ctx, "workflow:updates")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for msg := range ch {
		// Forward the update to all WebSocket clients
		ds.websocketHub.broadcast <- []byte(msg.Payload)
	}
}
