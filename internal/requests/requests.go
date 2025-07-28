package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// RequestStatus represents the state of a request
type RequestStatus string

const (
	StatusPending    RequestStatus = "pending"
	StatusProcessing RequestStatus = "processing"
	StatusCompleted  RequestStatus = "completed"
	StatusFailed     RequestStatus = "failed"
)

// Request represents a stored HTTP request
type Request struct {
	ID            string            `json:"id"`
	AgentID       string            `json:"agent_id"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	Headers       map[string]string `json:"headers"`
	Body          []byte            `json:"body"`
	Status        RequestStatus     `json:"status"`
	RetryCount    int               `json:"retry_count"`
	MaxRetries    int               `json:"max_retries"`
	CreatedAt     time.Time         `json:"created_at"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
	Response      *Response         `json:"response,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// Response represents a stored HTTP response
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
}

// Manager handles request persistence and replay
type Manager struct {
	redisClient *redis.Client
}

// NewManager creates a new request manager
func NewManager(redisClient *redis.Client) *Manager {
	return &Manager{
		redisClient: redisClient,
	}
}

// StoreRequest saves a request for an agent
func (m *Manager) StoreRequest(ctx context.Context, agentID string, req *http.Request) (*Request, error) {
	// Read and store the body
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore the body for further processing
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Extract headers
	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Create request object
	request := &Request{
		ID:         uuid.New().String(),
		AgentID:    agentID,
		Method:     req.Method,
		Path:       req.URL.Path,
		Headers:    headers,
		Body:       bodyBytes,
		Status:     StatusPending,
		RetryCount: 0,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	// Store in Redis
	key := fmt.Sprintf("agent:%s:requests:%s", agentID, request.ID)
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := m.redisClient.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		return nil, fmt.Errorf("failed to store request: %w", err)
	}

	// Add to pending queue
	queueKey := fmt.Sprintf("agent:%s:requests:pending", agentID)
	if err := m.redisClient.RPush(ctx, queueKey, request.ID).Err(); err != nil {
		return nil, fmt.Errorf("failed to add to pending queue: %w", err)
	}

	return request, nil
}

// StoreResponse updates a request with its response
func (m *Manager) StoreResponse(ctx context.Context, agentID, requestID string, resp *http.Response) error {
	// Read response body
	var bodyBytes []byte
	if resp.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		// Restore the body
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Extract headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Create response object
	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       bodyBytes,
		ReceivedAt: time.Now(),
	}

	// Update request with response
	key := fmt.Sprintf("agent:%s:requests:%s", agentID, requestID)
	
	// Get existing request
	data, err := m.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("failed to get request: %w", err)
	}

	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Update request
	now := time.Now()
	request.Response = response
	request.Status = StatusCompleted
	request.ProcessedAt = &now

	// Save updated request
	updatedData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal updated request: %w", err)
	}

	if err := m.redisClient.Set(ctx, key, updatedData, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	// Remove from pending queue
	queueKey := fmt.Sprintf("agent:%s:requests:pending", agentID)
	if err := m.redisClient.LRem(ctx, queueKey, 1, requestID).Err(); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to remove from pending queue: %v\n", err)
	}

	// Add to completed queue
	completedKey := fmt.Sprintf("agent:%s:requests:completed", agentID)
	if err := m.redisClient.RPush(ctx, completedKey, requestID).Err(); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to add to completed queue: %v\n", err)
	}

	return nil
}

// GetPendingRequests returns all pending requests for an agent
func (m *Manager) GetPendingRequests(ctx context.Context, agentID string) ([]*Request, error) {
	queueKey := fmt.Sprintf("agent:%s:requests:pending", agentID)
	
	// Get all request IDs from the queue
	requestIDs, err := m.redisClient.LRange(ctx, queueKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending queue: %w", err)
	}

	var requests []*Request
	for _, reqID := range requestIDs {
		key := fmt.Sprintf("agent:%s:requests:%s", agentID, reqID)
		data, err := m.redisClient.Get(ctx, key).Bytes()
		if err != nil {
			// Skip if request not found
			continue
		}

		var request Request
		if err := json.Unmarshal(data, &request); err != nil {
			// Skip invalid requests
			continue
		}

		requests = append(requests, &request)
	}

	return requests, nil
}

// MarkRequestFailed marks a request as failed
func (m *Manager) MarkRequestFailed(ctx context.Context, agentID, requestID string, err error) error {
	key := fmt.Sprintf("agent:%s:requests:%s", agentID, requestID)
	
	// Get existing request
	data, getErr := m.redisClient.Get(ctx, key).Bytes()
	if getErr != nil {
		return fmt.Errorf("failed to get request: %w", getErr)
	}

	var request Request
	if unmarshalErr := json.Unmarshal(data, &request); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal request: %w", unmarshalErr)
	}

	// Update request
	request.Status = StatusFailed
	request.Error = err.Error()
	request.RetryCount++

	// If we haven't exceeded max retries, keep it in pending
	if request.RetryCount < request.MaxRetries {
		request.Status = StatusPending
	} else {
		// Move to dead letter queue
		deadLetterKey := fmt.Sprintf("agent:%s:requests:failed", agentID)
		if pushErr := m.redisClient.RPush(ctx, deadLetterKey, requestID).Err(); pushErr != nil {
			fmt.Printf("Warning: failed to add to dead letter queue: %v\n", pushErr)
		}

		// Remove from pending
		queueKey := fmt.Sprintf("agent:%s:requests:pending", agentID)
		if remErr := m.redisClient.LRem(ctx, queueKey, 1, requestID).Err(); remErr != nil {
			fmt.Printf("Warning: failed to remove from pending queue: %v\n", remErr)
		}
	}

	// Save updated request
	updatedData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal updated request: %w", marshalErr)
	}

	if setErr := m.redisClient.Set(ctx, key, updatedData, 24*time.Hour).Err(); setErr != nil {
		return fmt.Errorf("failed to update request: %w", setErr)
	}

	return nil
}