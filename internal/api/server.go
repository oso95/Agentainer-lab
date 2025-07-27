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

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/agentainer/agentainer-lab/internal/config"
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
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(config *config.Config, agentMgr *agent.Manager, storage *storage.Storage, metricsCollector *metrics.Collector, redisClient *redis.Client) *Server {
	return &Server{
		config:           config,
		agentMgr:         agentMgr,
		storage:          storage,
		metricsCollector: metricsCollector,
		requestMgr:       requests.NewManager(redisClient),
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()

	r.HandleFunc("/health", s.healthHandler).Methods("GET")
	r.HandleFunc("/agents", s.deployAgentHandler).Methods("POST")
	r.HandleFunc("/agents", s.listAgentsHandler).Methods("GET")
	r.HandleFunc("/agents/{id}", s.getAgentHandler).Methods("GET")
	r.HandleFunc("/agents/{id}/start", s.startAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}/stop", s.stopAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}/restart", s.restartAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}/pause", s.pauseAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}/resume", s.resumeAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}", s.removeAgentHandler).Methods("DELETE")
	r.HandleFunc("/agents/{id}/logs", s.getLogsHandler).Methods("GET")
	r.HandleFunc("/agents/{id}/invoke", s.invokeAgentHandler).Methods("POST")
	r.HandleFunc("/agents/{id}/metrics", s.getMetricsHandler).Methods("GET")
	
	// Request management endpoints
	r.HandleFunc("/agents/{id}/requests", s.getAgentRequestsHandler).Methods("GET")
	r.HandleFunc("/agents/{id}/requests/{reqId}", s.getRequestHandler).Methods("GET")
	r.HandleFunc("/agents/{id}/requests/{reqId}/replay", s.replayRequestHandler).Methods("POST")
	
	// Proxy routes - catch-all for agent requests
	r.PathPrefix("/agent/{id}/").HandlerFunc(s.proxyToAgentHandler)

	r.Use(s.authMiddleware)
	r.Use(s.loggingMiddleware)

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

	agent, err := s.agentMgr.Deploy(r.Context(), req.Name, req.Image, req.EnvVars, req.CPULimit, req.MemoryLimit, req.AutoRestart, req.Token, req.Ports, req.Volumes)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to deploy agent: %v", err))
		return
	}

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

	metrics, err := s.metricsCollector.GetAgentMetrics(r.Context(), agentID)
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
		if r.URL.Path == "/health" {
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
	
	// Check if agent is running
	if agentObj.Status != agent.StatusRunning {
		// If agent is not running, check if request persistence is enabled
		if s.config.Features.RequestPersistence {
			// Store the request for later replay
			ctx := r.Context()
			storedReq, err := s.requestMgr.StoreRequest(ctx, agentID, r)
			if err != nil {
				s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store request: %v", err))
				return
			}
			
			s.sendResponse(w, http.StatusAccepted, Response{
				Success: true,
				Message: "Agent is not running. Request queued for replay when agent starts.",
				Data: map[string]string{
					"request_id": storedReq.ID,
					"status":     string(storedReq.Status),
				},
			})
			return
		}
		
		s.sendError(w, http.StatusServiceUnavailable, "Agent is not running")
		return
	}
	
	// Store request if persistence is enabled (skip if this is a replay)
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
	
	// In the new architecture, we connect to the agent using its container name
	// on the internal network. Default agent port is 8000.
	containerPort := 8000 // Standard agent port
	
	// Create target URL using the agent ID as hostname (set in container config)
	targetURL, err := url.Parse(fmt.Sprintf("http://%s:%d", agentObj.ID, containerPort))
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
	
	// Store the response if we have a request ID
	if t.requestID != "" && resp != nil {
		ctx := context.Background()
		if storeErr := t.requestMgr.StoreResponse(ctx, t.agentID, t.requestID, resp); storeErr != nil {
			// Log but don't fail
			fmt.Printf("Warning: Failed to store response: %v\n", storeErr)
		}
	}
	
	// Mark request as failed if there was an error
	if t.requestID != "" && err != nil {
		ctx := context.Background()
		if markErr := t.requestMgr.MarkRequestFailed(ctx, t.agentID, t.requestID, err); markErr != nil {
			fmt.Printf("Warning: Failed to mark request as failed: %v\n", markErr)
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