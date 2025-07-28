package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/agentainer/agentainer-lab/internal/config"
	"github.com/agentainer/agentainer-lab/internal/health"
	"github.com/agentainer/agentainer-lab/internal/logging"
	"github.com/agentainer/agentainer-lab/internal/requests"
	"github.com/agentainer/agentainer-lab/internal/storage"
	"github.com/agentainer/agentainer-lab/pkg/metrics"
)

type Server struct {
	config           *config.Config
	agentMgr         *agent.Manager
	storage          *storage.Storage
	metricsCollector *metrics.Collector
	requestMgr       *requests.Manager
	healthMonitor    *health.Monitor
	dockerClient     *client.Client
}

type DeployRequest struct {
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	EnvVars     map[string]string      `json:"env_vars"`
	CPULimit    int64                  `json:"cpu_limit"`
	MemoryLimit int64                  `json:"memory_limit"`
	AutoRestart bool                   `json:"auto_restart"`
	Token       string                 `json:"token"`
	Ports       []agent.PortMapping    `json:"ports"`
	Volumes     []agent.VolumeMapping  `json:"volumes"`
	HealthCheck *agent.HealthCheckConfig `json:"health_check,omitempty"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(config *config.Config, agentMgr *agent.Manager, storage *storage.Storage, metricsCollector *metrics.Collector, redisClient *redis.Client, dockerClient *client.Client) *Server {
	return &Server{
		config:           config,
		agentMgr:         agentMgr,
		storage:          storage,
		metricsCollector: metricsCollector,
		requestMgr:       requests.NewManager(redisClient),
		healthMonitor:    health.NewMonitor(agentMgr, redisClient),
		dockerClient:     dockerClient,
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()
	
	// Apply logging middleware to all routes
	r.Use(s.loggingMiddleware)
	
	// Public endpoints (no auth required)
	r.HandleFunc("/health", s.healthHandler).Methods("GET")
	
	// Proxy routes - catch-all for agent requests (no auth required)
	r.PathPrefix("/agent/{id}/").HandlerFunc(s.proxyToAgentHandler)
	
	// Protected API endpoints - create a subrouter with auth middleware
	api := r.PathPrefix("/").Subrouter()
	api.Use(s.authMiddleware)
	
	api.HandleFunc("/agents", s.deployAgentHandler).Methods("POST")
	api.HandleFunc("/agents", s.listAgentsHandler).Methods("GET")
	api.HandleFunc("/agents/{id}", s.getAgentHandler).Methods("GET")
	api.HandleFunc("/agents/{id}/start", s.startAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}/stop", s.stopAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}/restart", s.restartAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}/pause", s.pauseAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}/resume", s.resumeAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}", s.removeAgentHandler).Methods("DELETE")
	api.HandleFunc("/agents/{id}/logs", s.getLogsHandler).Methods("GET")
	api.HandleFunc("/agents/{id}/invoke", s.invokeAgentHandler).Methods("POST")
	api.HandleFunc("/agents/{id}/metrics", s.getMetricsHandler).Methods("GET")
	
	// Request management endpoints
	api.HandleFunc("/agents/{id}/requests", s.getAgentRequestsHandler).Methods("GET")
	api.HandleFunc("/agents/{id}/requests/{reqId}", s.getRequestHandler).Methods("GET")
	api.HandleFunc("/agents/{id}/requests/{reqId}/replay", s.replayRequestHandler).Methods("POST")
	
	// Health monitoring endpoints
	api.HandleFunc("/agents/{id}/health", s.getAgentHealthHandler).Methods("GET")
	api.HandleFunc("/health/agents", s.getAllHealthStatusesHandler).Methods("GET")
	
	// Metrics endpoints
	api.HandleFunc("/agents/{id}/metrics/history", s.getMetricsHistoryHandler).Methods("GET")

	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	
	// Security warnings for proof-of-concept
	fmt.Println("🚨 ================================================")
	fmt.Println("⚠️  AGENTAINER LAB - PROOF OF CONCEPT")
	fmt.Println("🚨 ================================================")
	fmt.Println("   WARNING: This is experimental software")
	fmt.Println("   - For local testing and feedback only")
	fmt.Println("   - Uses default authentication tokens")
	fmt.Println("   - Minimal security controls")
	fmt.Println("   - Do NOT expose to external networks")
	fmt.Println("🚨 ================================================")
	fmt.Printf("Server starting on %s\n", addr)
	
	// Start health monitoring
	go func() {
		if err := s.healthMonitor.Start(context.Background()); err != nil {
			fmt.Printf("Failed to start health monitor: %v\n", err)
		}
	}()
	
	// Start metrics collection
	go func() {
		if err := s.metricsCollector.Start(context.Background()); err != nil {
			fmt.Printf("Failed to start metrics collector: %v\n", err)
		}
	}()
	
	return http.ListenAndServe(addr, r)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Service is healthy",
		Data: map[string]string{
			"status": "ok",
		},
	})
}

func (s *Server) deployAgentHandler(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Basic input validation for proof-of-concept
	if req.Name == "" || req.Image == "" {
		s.sendError(w, http.StatusBadRequest, "Name and image are required")
		return
	}
	
	// Limit name length to prevent abuse
	if len(req.Name) > 64 {
		s.sendError(w, http.StatusBadRequest, "Agent name too long (max 64 characters)")
		return
	}
	
	// Limit image name length
	if len(req.Image) > 256 {
		s.sendError(w, http.StatusBadRequest, "Image name too long (max 256 characters)")
		return
	}
	
	// Limit number of environment variables
	if len(req.EnvVars) > 50 {
		s.sendError(w, http.StatusBadRequest, "Too many environment variables (max 50)")
		return
	}

	if req.Token == "" {
		req.Token = s.config.Security.DefaultToken
	}

	agent, err := s.agentMgr.Deploy(r.Context(), req.Name, req.Image, req.EnvVars, req.CPULimit, req.MemoryLimit, req.AutoRestart, req.Token, req.Ports, req.Volumes, req.HealthCheck)
	if err != nil {
		// Log error
		logging.Error("api", "Failed to deploy agent", map[string]interface{}{
			"name": req.Name,
			"image": req.Image,
			"error": err.Error(),
		})
		
		// Audit log
		logging.AuditLog(logging.AuditEntry{
			UserID:     s.getUserID(r),
			Action:     "deploy_agent",
			Resource:   "agent",
			ResourceID: req.Name,
			Result:     "failure",
			Details:    map[string]interface{}{"error": err.Error()},
			IP:         s.getClientIP(r),
			UserAgent:  r.UserAgent(),
		})
		
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to deploy agent: %v", err))
		return
	}

	// Log success
	logging.Info("api", "Agent deployed successfully", map[string]interface{}{
		"agent_id": agent.ID,
		"name": agent.Name,
		"image": agent.Image,
	})
	
	// Audit log
	logging.AuditLog(logging.AuditEntry{
		UserID:     s.getUserID(r),
		Action:     "deploy_agent",
		Resource:   "agent",
		ResourceID: agent.ID,
		Result:     "success",
		Details:    map[string]interface{}{"name": agent.Name, "image": agent.Image},
		IP:         s.getClientIP(r),
		UserAgent:  r.UserAgent(),
	})

	s.sendResponse(w, http.StatusCreated, Response{
		Success: true,
		Message: "Agent deployed successfully",
		Data:    agent,
	})
}

func (s *Server) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// API lists all agents regardless of token (same as CLI)
	agents, err := s.agentMgr.ListAgents("")
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list agents: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agents retrieved successfully",
		Data:    agents,
	})
}


func (s *Server) getAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	agent, err := s.agentMgr.GetAgent(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Agent not found: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent retrieved successfully",
		Data:    agent,
	})
}

func (s *Server) startAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	
	// Basic agent ID validation
	if len(agentID) > 128 {
		s.sendError(w, http.StatusBadRequest, "Invalid agent ID")
		return
	}

	if err := s.agentMgr.Start(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start agent: %v", err))
		return
	}

	// Start health monitoring if configured
	agent, _ := s.agentMgr.GetAgent(agentID)
	if agent != nil && agent.HealthCheck != nil {
		config := health.CheckConfig{
			Endpoint: agent.HealthCheck.Endpoint,
			Interval: parseDuration(agent.HealthCheck.Interval, 30*time.Second),
			Timeout:  parseDuration(agent.HealthCheck.Timeout, 5*time.Second),
			Retries:  agent.HealthCheck.Retries,
		}
		s.healthMonitor.StartMonitoring(agentID, config)
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent started successfully",
	})
}

func (s *Server) stopAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if err := s.agentMgr.Stop(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop agent: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent stopped successfully",
	})
}

func (s *Server) restartAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if err := s.agentMgr.Restart(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to restart agent: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent restarted successfully",
	})
}

func (s *Server) pauseAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if err := s.agentMgr.Pause(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to pause agent: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent paused successfully",
	})
}

func (s *Server) resumeAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if err := s.agentMgr.Resume(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to resume agent: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent resumed successfully",
	})
}

func (s *Server) removeAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	// Get agent info before removal for response
	agent, err := s.agentMgr.GetAgent(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Agent not found: %v", err))
		return
	}

	if err := s.agentMgr.Remove(r.Context(), agentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove agent: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: fmt.Sprintf("Agent '%s' (ID: %s) removed successfully", agent.Name, agentID),
		Data: map[string]string{
			"agent_id": agentID,
			"agent_name": agent.Name,
		},
	})
}

func (s *Server) getLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	follow := r.URL.Query().Get("follow") == "true"

	logs, err := s.agentMgr.GetLogs(r.Context(), agentID, follow)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get logs: %v", err))
		return
	}
	defer logs.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	
	io.Copy(w, logs)
}

func (s *Server) invokeAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	agentObj, err := s.agentMgr.GetAgent(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Agent not found: %v", err))
		return
	}

	if agentObj.Status != agent.StatusRunning {
		s.sendError(w, http.StatusBadRequest, "Agent is not running")
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent invoked successfully",
		Data: map[string]string{
			"agent_id": agentID,
			"status":   "invoked",
		},
	})
}

func (s *Server) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	metrics, err := s.metricsCollector.GetMetrics(agentID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get metrics: %v", err))
		return
	}

	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Metrics retrieved successfully",
		Data:    metrics,
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/web/") {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token != "" && strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		if token == "" {
			s.sendError(w, http.StatusUnauthorized, "Missing authorization token")
			return
		}

		if token != s.config.Security.DefaultToken {
			s.sendError(w, http.StatusUnauthorized, "Invalid authorization token")
			return
		}

		ctx := context.WithValue(r.Context(), "authToken", token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[%s] %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sendResponse(w http.ResponseWriter, statusCode int, response Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) proxyToAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	
	// Get agent details
	agentObj, err := s.agentMgr.GetAgent(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Agent not found: %v", err))
		return
	}
	
	// Store request if persistence is enabled (for both running and stopped agents)
	var requestID string
	isReplay := r.Header.Get("X-Agentainer-Replay") == "true"
	
	if s.config.Features.RequestPersistence && !isReplay {
		ctx := r.Context()
		storedReq, err := s.requestMgr.StoreRequest(ctx, agentID, r)
		if err != nil {
			// Log but don't fail the request
			fmt.Printf("Warning: Failed to store request: %v\n", err)
		} else {
			requestID = storedReq.ID
			// Add request ID to headers for tracking
			r.Header.Set("X-Agentainer-Request-ID", requestID)
		}
	} else if isReplay {
		// For replays, get the request ID from header
		requestID = r.Header.Get("X-Agentainer-Request-ID")
	}
	
	// Check if agent is running
	if agentObj.Status != agent.StatusRunning {
		if s.config.Features.RequestPersistence && requestID != "" {
			// We already stored the request above
			s.sendResponse(w, http.StatusAccepted, Response{
				Success: true,
				Message: "Agent is not running. Request queued for replay when agent starts.",
				Data: map[string]string{
					"request_id": requestID,
					"status":     "pending",
				},
			})
			return
		}
		
		s.sendError(w, http.StatusServiceUnavailable, "Agent is not running")
		return
	}
	
	// In the new architecture, we connect to the agent using its hostname
	// on the internal network. The agent ID is used as the hostname.
	// Default agent port is 8000.
	targetURL, err := url.Parse(fmt.Sprintf("http://%s:8000", agentObj.ID))
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to parse target URL")
		return
	}
	
	// Modify the request path to remove the /agent/{id} prefix
	originalPath := r.URL.Path
	r.URL.Path = strings.TrimPrefix(originalPath, fmt.Sprintf("/agent/%s", agentID))
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
	
	// Create custom transport to intercept response
	transport := &interceptTransport{
		base:       http.DefaultTransport,
		requestMgr: s.requestMgr,
		agentID:    agentID,
		requestID:  requestID,
	}
	
	// Create reverse proxy with custom transport
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = transport
	
	// Forward the request
	proxy.ServeHTTP(w, r)
}

// interceptTransport wraps http.RoundTripper to capture responses
type interceptTransport struct {
	base       http.RoundTripper
	requestMgr *requests.Manager
	agentID    string
	requestID  string
}

func (t *interceptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Forward the request
	resp, err := t.base.RoundTrip(req)
	
	// Handle successful response
	if t.requestID != "" && resp != nil && err == nil {
		ctx := context.Background()
		if storeErr := t.requestMgr.StoreResponse(ctx, t.agentID, t.requestID, resp); storeErr != nil {
			// Log but don't fail
			fmt.Printf("Warning: Failed to store response: %v\n", storeErr)
		}
	}
	
	// Handle connection failures (agent crashed or network issues)
	if t.requestID != "" && err != nil {
		ctx := context.Background()
		// Check if this is a connection error (agent likely crashed)
		if strings.Contains(err.Error(), "connection refused") || 
		   strings.Contains(err.Error(), "no such host") ||
		   strings.Contains(err.Error(), "dial tcp") {
			fmt.Printf("Agent %s appears to have crashed during request %s: %v\n", 
				t.agentID, t.requestID, err)
			// The request remains in pending state and will be retried when agent restarts
		} else {
			// Other errors mark the request as failed
			if markErr := t.requestMgr.MarkRequestFailed(ctx, t.agentID, t.requestID, err); markErr != nil {
				fmt.Printf("Warning: Failed to mark request as failed: %v\n", markErr)
			}
		}
	}
	
	return resp, err
}

func (s *Server) sendError(w http.ResponseWriter, statusCode int, message string) {
	s.sendResponse(w, statusCode, Response{
		Success: false,
		Message: message,
	})
}

// Request management handlers

func (s *Server) getAgentRequestsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	
	// Verify agent exists
	if _, err := s.agentMgr.GetAgent(agentID); err != nil {
		s.sendError(w, http.StatusNotFound, "Agent not found")
		return
	}
	
	// Get pending requests
	ctx := r.Context()
	pendingReqs, err := s.requestMgr.GetPendingRequests(ctx, agentID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get requests: %v", err))
		return
	}
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Requests retrieved successfully",
		Data: map[string]interface{}{
			"agent_id": agentID,
			"pending":  pendingReqs,
			"count":    len(pendingReqs),
		},
	})
}

func (s *Server) getRequestHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	requestID := vars["reqId"]
	
	// Get request from storage
	key := fmt.Sprintf("agent:%s:requests:%s", agentID, requestID)
	data, err := s.storage.Get(r.Context(), key)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Request not found")
		return
	}
	
	var request requests.Request
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to parse request")
		return
	}
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Request retrieved successfully",
		Data:    request,
	})
}

func (s *Server) replayRequestHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	requestID := vars["reqId"]
	
	// Get request from storage
	key := fmt.Sprintf("agent:%s:requests:%s", agentID, requestID)
	data, err := s.storage.Get(r.Context(), key)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Request not found")
		return
	}
	
	var storedReq requests.Request
	if err := json.Unmarshal([]byte(data), &storedReq); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to parse request")
		return
	}
	
	// Check if agent is running
	agent, err := s.agentMgr.GetAgent(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Agent not found")
		return
	}
	
	if agent.Status != "running" {
		s.sendError(w, http.StatusServiceUnavailable, "Agent is not running")
		return
	}
	
	// Recreate the HTTP request
	targetURL := fmt.Sprintf("http://%s:8000%s", agentID, storedReq.Path)
	httpReq, err := http.NewRequest(storedReq.Method, targetURL, bytes.NewReader(storedReq.Body))
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}
	
	// Restore headers
	for k, v := range storedReq.Headers {
		httpReq.Header.Set(k, v)
	}
	
	// Execute the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		// Mark as failed
		ctx := r.Context()
		s.requestMgr.MarkRequestFailed(ctx, agentID, requestID, err)
		s.sendError(w, http.StatusBadGateway, fmt.Sprintf("Failed to replay request: %v", err))
		return
	}
	defer resp.Body.Close()
	
	// Store the new response
	ctx := r.Context()
	if err := s.requestMgr.StoreResponse(ctx, agentID, requestID, resp); err != nil {
		fmt.Printf("Warning: Failed to store replay response: %v\n", err)
	}
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Request replayed successfully",
		Data: map[string]interface{}{
			"request_id":  requestID,
			"status_code": resp.StatusCode,
		},
	})
}

func (s *Server) getAgentHealthHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	
	status, err := s.healthMonitor.GetStatus(agentID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "No health data for agent")
		return
	}
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Agent health status",
		Data:    status,
	})
}

func (s *Server) getAllHealthStatusesHandler(w http.ResponseWriter, r *http.Request) {
	statuses := s.healthMonitor.GetAllStatuses()
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "All agent health statuses",
		Data:    statuses,
	})
}

func (s *Server) getMetricsHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]
	
	// Parse duration parameter (default: 1 hour)
	durationStr := r.URL.Query().Get("duration")
	duration := 1 * time.Hour
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}
	
	// Limit to 24 hours max
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
	}
	
	history, err := s.metricsCollector.GetMetricsHistory(agentID, duration)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get metrics history: %v", err))
		return
	}
	
	s.sendResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Metrics history retrieved successfully",
		Data: map[string]interface{}{
			"agent_id": agentID,
			"duration": duration.String(),
			"metrics":  history,
		},
	})
}

// parseDuration parses a duration string, returning defaultDur if parsing fails
func parseDuration(s string, defaultDur time.Duration) time.Duration {
	if s == "" {
		return defaultDur
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return defaultDur
	}
	return dur
}

// getUserID extracts user ID from the request (from token)
func (s *Server) getUserID(r *http.Request) string {
	// In a real implementation, you'd decode the JWT token
	// For now, just use the token as user ID
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return "anonymous"
}

// getClientIP extracts the client IP from the request
func (s *Server) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Use the first IP in the chain
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	
	return ip
}