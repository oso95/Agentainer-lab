package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
)

// MapStepState tracks the state of a map step execution
type MapStepState struct {
	TotalItems     int           `json:"total_items"`
	CompletedItems int           `json:"completed_items"`
	FailedItems    int           `json:"failed_items"`
	Results        []interface{} `json:"results"`
}

// StateManager handles workflow state persistence with thread-safe operations
type StateManager struct {
	redisClient *redis.Client
	mu          sync.RWMutex
}

// NewStateManager creates a new state manager
func NewStateManager(redisClient *redis.Client) *StateManager {
	return &StateManager{
		redisClient: redisClient,
	}
}

// Set stores a value in workflow state
func (sm *StateManager) Set(ctx context.Context, workflowID, key string, value interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Store in Redis hash
	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	return sm.redisClient.HSet(ctx, hashKey, key, data).Err()
}

// Get retrieves a value from workflow state and returns it directly
func (sm *StateManager) Get(ctx context.Context, workflowID, key string) (interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	data, err := sm.redisClient.HGet(ctx, hashKey, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get value: %w", err)
	}

	// Deserialize value
	var value interface{}
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return value, nil
}

// GetInto retrieves a value from workflow state into a destination
func (sm *StateManager) GetInto(ctx context.Context, workflowID, key string, dest interface{}) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	data, err := sm.redisClient.HGet(ctx, hashKey, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("key not found")
	} else if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	// Deserialize value
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// SetValue is an alias for Set for backward compatibility
func (sm *StateManager) SetValue(ctx context.Context, workflowID string, key string, value interface{}) error {
	return sm.Set(ctx, workflowID, key, value)
}

// GetState retrieves all state for a workflow
func (sm *StateManager) GetState(ctx context.Context, workflowID string) (map[string]interface{}, error) {
	return sm.GetAll(ctx, workflowID)
}

// GetAll retrieves all state for a workflow
func (sm *StateManager) GetAll(ctx context.Context, workflowID string) (map[string]interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	data, err := sm.redisClient.HGetAll(ctx, hashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get all state: %w", err)
	}

	state := make(map[string]interface{})
	for key, value := range data {
		var val interface{}
		if err := json.Unmarshal([]byte(value), &val); err != nil {
			// Store as string if unmarshal fails
			state[key] = value
		} else {
			state[key] = val
		}
	}

	return state, nil
}

// SetMapState stores the state for a map step
func (sm *StateManager) SetMapState(ctx context.Context, workflowID, stepID string, mapState *MapStepState) error {
	key := fmt.Sprintf("map_%s_state", stepID)
	return sm.Set(ctx, workflowID, key, mapState)
}

// GetMapState retrieves the state for a map step
func (sm *StateManager) GetMapState(ctx context.Context, workflowID, stepID string) (*MapStepState, error) {
	key := fmt.Sprintf("map_%s_state", stepID)
	var mapState MapStepState
	err := sm.GetInto(ctx, workflowID, key, &mapState)
	if err != nil {
		return nil, err
	}
	return &mapState, nil
}

// Delete removes a key from workflow state
func (sm *StateManager) Delete(ctx context.Context, workflowID, key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	return sm.redisClient.HDel(ctx, hashKey, key).Err()
}

// Clear removes all state for a workflow
func (sm *StateManager) Clear(ctx context.Context, workflowID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	return sm.redisClient.Del(ctx, hashKey).Err()
}

// Atomic operations for thread-safe state updates

// Increment atomically increments a numeric value
func (sm *StateManager) Increment(ctx context.Context, workflowID, key string, delta int64) (int64, error) {
	hashKey := fmt.Sprintf("workflow:%s:state", workflowID)
	fieldKey := fmt.Sprintf("%s:counter", key)
	
	return sm.redisClient.HIncrBy(ctx, hashKey, fieldKey, delta).Result()
}

// AppendToList atomically appends to a list
func (sm *StateManager) AppendToList(ctx context.Context, workflowID, key string, value interface{}) error {
	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	listKey := fmt.Sprintf("workflow:%s:state:list:%s", workflowID, key)
	return sm.redisClient.RPush(ctx, listKey, data).Err()
}

// GetList retrieves a list from state
func (sm *StateManager) GetList(ctx context.Context, workflowID, key string) ([]interface{}, error) {
	listKey := fmt.Sprintf("workflow:%s:state:list:%s", workflowID, key)
	
	data, err := sm.redisClient.LRange(ctx, listKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	result := make([]interface{}, len(data))
	for i, item := range data {
		var val interface{}
		if err := json.Unmarshal([]byte(item), &val); err != nil {
			result[i] = item
		} else {
			result[i] = val
		}
	}

	return result, nil
}

// AddToSet atomically adds to a set
func (sm *StateManager) AddToSet(ctx context.Context, workflowID, key string, value interface{}) error {
	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	setKey := fmt.Sprintf("workflow:%s:state:set:%s", workflowID, key)
	return sm.redisClient.SAdd(ctx, setKey, data).Err()
}

// GetSet retrieves a set from state
func (sm *StateManager) GetSet(ctx context.Context, workflowID, key string) ([]interface{}, error) {
	setKey := fmt.Sprintf("workflow:%s:state:set:%s", workflowID, key)
	
	data, err := sm.redisClient.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get set: %w", err)
	}

	result := make([]interface{}, len(data))
	for i, item := range data {
		var val interface{}
		if err := json.Unmarshal([]byte(item), &val); err != nil {
			result[i] = item
		} else {
			result[i] = val
		}
	}

	return result, nil
}

// CompareAndSwap performs an atomic compare-and-swap operation
func (sm *StateManager) CompareAndSwap(ctx context.Context, workflowID, key string, oldValue, newValue interface{}) (bool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get current value
	current, err := sm.Get(ctx, workflowID, key)
	if err != nil {
		if err.Error() == "key not found" && oldValue == nil {
			// Key doesn't exist and we expected nil
			return true, sm.Set(ctx, workflowID, key, newValue)
		}
		return false, err
	}

	// Compare values
	oldData, _ := json.Marshal(oldValue)
	currentData, _ := json.Marshal(current)
	
	if string(oldData) != string(currentData) {
		return false, nil
	}

	// Set new value
	return true, sm.Set(ctx, workflowID, key, newValue)
}

// WorkflowContext provides a convenient interface for workflow state operations
type WorkflowContext struct {
	WorkflowID   string
	StateManager *StateManager
}

// NewWorkflowContext creates a new workflow context
func NewWorkflowContext(workflowID string, stateManager *StateManager) *WorkflowContext {
	return &WorkflowContext{
		WorkflowID:   workflowID,
		StateManager: stateManager,
	}
}

// Set stores a value in workflow state
func (wc *WorkflowContext) Set(ctx context.Context, key string, value interface{}) error {
	return wc.StateManager.Set(ctx, wc.WorkflowID, key, value)
}

// Get retrieves a value from workflow state
func (wc *WorkflowContext) Get(ctx context.Context, key string, dest interface{}) error {
	return wc.StateManager.GetInto(ctx, wc.WorkflowID, key, dest)
}

// Increment atomically increments a counter
func (wc *WorkflowContext) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return wc.StateManager.Increment(ctx, wc.WorkflowID, key, delta)
}

// AppendToList appends to a list
func (wc *WorkflowContext) AppendToList(ctx context.Context, key string, value interface{}) error {
	return wc.StateManager.AppendToList(ctx, wc.WorkflowID, key, value)
}

// AddToSet adds to a set
func (wc *WorkflowContext) AddToSet(ctx context.Context, key string, value interface{}) error {
	return wc.StateManager.AddToSet(ctx, wc.WorkflowID, key, value)
}