package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/robfig/cron/v3"
)

// TriggerType defines the type of workflow trigger
type TriggerType string

const (
	TriggerTypeSchedule TriggerType = "schedule"
	TriggerTypeEvent    TriggerType = "event"
	TriggerTypeManual   TriggerType = "manual"
	TriggerTypeWebhook  TriggerType = "webhook"
)

// WorkflowTrigger defines a trigger for workflow execution
type WorkflowTrigger struct {
	ID          string            `json:"id"`
	WorkflowID  string            `json:"workflow_id"`
	Type        TriggerType       `json:"type"`
	Config      TriggerConfig     `json:"config"`
	Enabled     bool              `json:"enabled"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	NextRun     *time.Time        `json:"next_run,omitempty"`
	RunCount    int               `json:"run_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TriggerConfig holds trigger-specific configuration
type TriggerConfig struct {
	// Schedule triggers
	CronExpression string `json:"cron_expression,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	
	// Event triggers
	EventType   string                 `json:"event_type,omitempty"`
	EventFilter map[string]interface{} `json:"event_filter,omitempty"`
	
	// Webhook triggers
	WebhookPath   string `json:"webhook_path,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
	
	// Common
	SkipIfRunning bool              `json:"skip_if_running,omitempty"`
	CatchUp       bool              `json:"catch_up,omitempty"`
	InputData     map[string]interface{} `json:"input_data,omitempty"`
}

// Scheduler manages workflow scheduling and triggers
type Scheduler struct {
	workflowManager *Manager
	orchestrator    *Orchestrator
	redisClient     *redis.Client
	cron            *cron.Cron
	triggers        sync.Map // map[triggerID]*WorkflowTrigger
	mu              sync.RWMutex
}

// NewScheduler creates a new workflow scheduler
func NewScheduler(workflowManager *Manager, orchestrator *Orchestrator, redisClient *redis.Client) *Scheduler {
	// Create cron with second precision
	c := cron.New(cron.WithSeconds())
	
	s := &Scheduler{
		workflowManager: workflowManager,
		orchestrator:    orchestrator,
		redisClient:     redisClient,
		cron:            c,
	}
	
	return s
}

// Start begins the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	// Load existing triggers from Redis
	if err := s.loadTriggers(ctx); err != nil {
		return fmt.Errorf("failed to load triggers: %w", err)
	}
	
	// Start cron scheduler
	s.cron.Start()
	
	// Start event listener
	go s.eventListener(ctx)
	
	log.Println("Workflow scheduler started")
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Workflow scheduler stopped")
}

// CreateTrigger creates a new workflow trigger
func (s *Scheduler) CreateTrigger(ctx context.Context, workflowID string, triggerType TriggerType, config TriggerConfig) (*WorkflowTrigger, error) {
	// Validate workflow exists
	if _, err := s.workflowManager.GetWorkflow(ctx, workflowID); err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}
	
	trigger := &WorkflowTrigger{
		ID:         fmt.Sprintf("trigger-%d", time.Now().UnixNano()),
		WorkflowID: workflowID,
		Type:       triggerType,
		Config:     config,
		Enabled:    true,
		RunCount:   0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	
	// Validate and setup trigger based on type
	switch triggerType {
	case TriggerTypeSchedule:
		if err := s.setupScheduleTrigger(trigger); err != nil {
			return nil, err
		}
	case TriggerTypeEvent:
		if err := s.validateEventTrigger(trigger); err != nil {
			return nil, err
		}
	case TriggerTypeWebhook:
		if err := s.validateWebhookTrigger(trigger); err != nil {
			return nil, err
		}
	}
	
	// Save trigger
	if err := s.saveTrigger(ctx, trigger); err != nil {
		return nil, fmt.Errorf("failed to save trigger: %w", err)
	}
	
	// Store in memory
	s.triggers.Store(trigger.ID, trigger)
	
	return trigger, nil
}

// setupScheduleTrigger sets up a cron-based trigger
func (s *Scheduler) setupScheduleTrigger(trigger *WorkflowTrigger) error {
	if trigger.Config.CronExpression == "" {
		return fmt.Errorf("cron expression is required for schedule triggers")
	}
	
	// Parse cron expression
	schedule, err := cron.ParseStandard(trigger.Config.CronExpression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	
	// Calculate next run time
	now := time.Now()
	next := schedule.Next(now)
	trigger.NextRun = &next
	
	// Add to cron scheduler
	entryID, err := s.cron.AddFunc(trigger.Config.CronExpression, func() {
		s.ExecuteTrigger(context.Background(), trigger.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}
	
	// Store cron entry ID in metadata
	if trigger.Metadata == nil {
		trigger.Metadata = make(map[string]string)
	}
	trigger.Metadata["cron_entry_id"] = fmt.Sprintf("%d", entryID)
	
	return nil
}

// validateEventTrigger validates event trigger configuration
func (s *Scheduler) validateEventTrigger(trigger *WorkflowTrigger) error {
	if trigger.Config.EventType == "" {
		return fmt.Errorf("event type is required for event triggers")
	}
	return nil
}

// validateWebhookTrigger validates webhook trigger configuration
func (s *Scheduler) validateWebhookTrigger(trigger *WorkflowTrigger) error {
	if trigger.Config.WebhookPath == "" {
		return fmt.Errorf("webhook path is required for webhook triggers")
	}
	return nil
}

// ExecuteTrigger executes a workflow based on trigger
func (s *Scheduler) ExecuteTrigger(ctx context.Context, triggerID string) {
	// Get trigger
	triggerInterface, exists := s.triggers.Load(triggerID)
	if !exists {
		log.Printf("Trigger %s not found", triggerID)
		return
	}
	
	trigger := triggerInterface.(*WorkflowTrigger)
	
	// Check if enabled
	if !trigger.Enabled {
		log.Printf("Trigger %s is disabled", triggerID)
		return
	}
	
	// Check if should skip if already running
	if trigger.Config.SkipIfRunning {
		workflows, err := s.workflowManager.ListWorkflows(ctx, WorkflowStatusRunning)
		if err == nil {
			for _, w := range workflows {
				if w.ID == trigger.WorkflowID {
					log.Printf("Skipping trigger %s - workflow already running", triggerID)
					return
				}
			}
		}
	}
	
	// Create new workflow execution
	workflow, err := s.workflowManager.GetWorkflow(ctx, trigger.WorkflowID)
	if err != nil {
		log.Printf("Failed to get workflow for trigger %s: %v", triggerID, err)
		return
	}
	
	// Clone workflow for new execution
	newWorkflow := &Workflow{
		ID:          fmt.Sprintf("%s-%d", workflow.ID, time.Now().UnixNano()),
		Name:        fmt.Sprintf("%s (triggered)", workflow.Name),
		Description: workflow.Description,
		Status:      WorkflowStatusPending,
		Config:      workflow.Config,
		Steps:       workflow.Steps,
		State:       make(map[string]interface{}),
		Metadata:    workflow.Metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Add trigger info to metadata
	if newWorkflow.Metadata == nil {
		newWorkflow.Metadata = make(map[string]string)
	}
	newWorkflow.Metadata["trigger_id"] = triggerID
	newWorkflow.Metadata["trigger_type"] = string(trigger.Type)
	
	// Add input data to state
	if trigger.Config.InputData != nil {
		for k, v := range trigger.Config.InputData {
			newWorkflow.State[k] = v
		}
	}
	
	// Save new workflow
	if err := s.workflowManager.SaveWorkflow(ctx, newWorkflow); err != nil {
		log.Printf("Failed to save triggered workflow: %v", err)
		return
	}
	
	// Execute workflow
	go func() {
		if err := s.orchestrator.ExecuteWorkflow(context.Background(), newWorkflow.ID); err != nil {
			log.Printf("Failed to execute triggered workflow: %v", err)
		}
	}()
	
	// Update trigger stats
	now := time.Now()
	trigger.LastRun = &now
	trigger.RunCount++
	
	// Calculate next run for schedule triggers
	if trigger.Type == TriggerTypeSchedule && trigger.Config.CronExpression != "" {
		schedule, _ := cron.ParseStandard(trigger.Config.CronExpression)
		next := schedule.Next(now)
		trigger.NextRun = &next
	}
	
	trigger.UpdatedAt = now
	
	// Save updated trigger
	s.saveTrigger(ctx, trigger)
	
	log.Printf("Executed trigger %s for workflow %s", triggerID, trigger.WorkflowID)
}

// eventListener listens for events that can trigger workflows
func (s *Scheduler) eventListener(ctx context.Context) {
	// Subscribe to Redis events
	pubsub := s.redisClient.Subscribe(ctx, "workflow:events")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			// Parse event
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("Failed to parse event: %v", err)
				continue
			}
			
			// Check all event triggers
			s.triggers.Range(func(key, value interface{}) bool {
				trigger := value.(*WorkflowTrigger)
				
				if trigger.Type == TriggerTypeEvent && trigger.Enabled {
					if s.matchesEventFilter(event, trigger.Config.EventType, trigger.Config.EventFilter) {
						s.ExecuteTrigger(ctx, trigger.ID)
					}
				}
				
				return true
			})
		}
	}
}

// matchesEventFilter checks if an event matches trigger filters
func (s *Scheduler) matchesEventFilter(event map[string]interface{}, eventType string, filter map[string]interface{}) bool {
	// Check event type
	if et, ok := event["type"].(string); ok && et != eventType {
		return false
	}
	
	// Check additional filters
	for key, expectedValue := range filter {
		if actualValue, exists := event[key]; !exists || actualValue != expectedValue {
			return false
		}
	}
	
	return true
}

// ListTriggers lists all triggers for a workflow
func (s *Scheduler) ListTriggers(ctx context.Context, workflowID string) ([]*WorkflowTrigger, error) {
	var triggers []*WorkflowTrigger
	
	s.triggers.Range(func(key, value interface{}) bool {
		trigger := value.(*WorkflowTrigger)
		if trigger.WorkflowID == workflowID {
			triggers = append(triggers, trigger)
		}
		return true
	})
	
	return triggers, nil
}

// EnableTrigger enables a trigger
func (s *Scheduler) EnableTrigger(ctx context.Context, triggerID string) error {
	triggerInterface, exists := s.triggers.Load(triggerID)
	if !exists {
		return fmt.Errorf("trigger not found")
	}
	
	trigger := triggerInterface.(*WorkflowTrigger)
	trigger.Enabled = true
	trigger.UpdatedAt = time.Now()
	
	return s.saveTrigger(ctx, trigger)
}

// DisableTrigger disables a trigger
func (s *Scheduler) DisableTrigger(ctx context.Context, triggerID string) error {
	triggerInterface, exists := s.triggers.Load(triggerID)
	if !exists {
		return fmt.Errorf("trigger not found")
	}
	
	trigger := triggerInterface.(*WorkflowTrigger)
	trigger.Enabled = false
	trigger.UpdatedAt = time.Now()
	
	return s.saveTrigger(ctx, trigger)
}

// saveTrigger saves a trigger to Redis
func (s *Scheduler) saveTrigger(ctx context.Context, trigger *WorkflowTrigger) error {
	data, err := json.Marshal(trigger)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger: %w", err)
	}
	
	key := fmt.Sprintf("trigger:%s", trigger.ID)
	if err := s.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save trigger to Redis: %w", err)
	}
	
	// Add to triggers list
	if err := s.redisClient.SAdd(ctx, "triggers:list", trigger.ID).Err(); err != nil {
		return fmt.Errorf("failed to add trigger to list: %w", err)
	}
	
	return nil
}

// loadTriggers loads all triggers from Redis
func (s *Scheduler) loadTriggers(ctx context.Context) error {
	triggerIDs, err := s.redisClient.SMembers(ctx, "triggers:list").Result()
	if err != nil {
		return fmt.Errorf("failed to get trigger list: %w", err)
	}
	
	for _, id := range triggerIDs {
		key := fmt.Sprintf("trigger:%s", id)
		data, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		
		var trigger WorkflowTrigger
		if err := json.Unmarshal([]byte(data), &trigger); err != nil {
			continue
		}
		
		// Re-setup schedule triggers
		if trigger.Type == TriggerTypeSchedule && trigger.Enabled {
			s.setupScheduleTrigger(&trigger)
		}
		
		s.triggers.Store(trigger.ID, &trigger)
	}
	
	return nil
}