package workflow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// ConditionType defines the type of condition
type ConditionType string

const (
	ConditionTypeSimple     ConditionType = "simple"      // Simple comparison
	ConditionTypeExpression ConditionType = "expression"  // Complex expression
	ConditionTypeCustom     ConditionType = "custom"      // Custom function
)

// Operator defines comparison operators
type Operator string

const (
	OpEqual            Operator = "=="
	OpNotEqual         Operator = "!="
	OpGreater          Operator = ">"
	OpGreaterOrEqual   Operator = ">="
	OpLess             Operator = "<"
	OpLessOrEqual      Operator = "<="
	OpContains         Operator = "contains"
	OpNotContains      Operator = "not_contains"
	OpIn               Operator = "in"
	OpNotIn            Operator = "not_in"
	OpMatches          Operator = "matches"  // Regex match
)

// Condition defines a workflow condition
type Condition struct {
	ID          string                 `json:"id"`
	Type        ConditionType          `json:"type"`
	Field       string                 `json:"field,omitempty"`       // State field to check
	Operator    Operator               `json:"operator,omitempty"`    // Comparison operator
	Value       interface{}            `json:"value,omitempty"`       // Value to compare against
	Expression  string                 `json:"expression,omitempty"`  // Complex expression
	And         []Condition            `json:"and,omitempty"`         // AND conditions
	Or          []Condition            `json:"or,omitempty"`          // OR conditions
	Not         *Condition             `json:"not,omitempty"`         // NOT condition
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// BranchConfig defines configuration for conditional branching
type BranchConfig struct {
	Condition   Condition              `json:"condition"`
	TrueSteps   []string               `json:"true_steps,omitempty"`   // Steps to execute if true
	FalseSteps  []string               `json:"false_steps,omitempty"`  // Steps to execute if false
	TrueWorkflow  string               `json:"true_workflow,omitempty"` // Sub-workflow if true
	FalseWorkflow string               `json:"false_workflow,omitempty"` // Sub-workflow if false
}

// DecisionNode represents a decision point in the workflow
type DecisionNode struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Branches    []DecisionBranch       `json:"branches"`
	Default     string                 `json:"default,omitempty"`      // Default branch if no match
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// DecisionBranch represents one branch in a decision node
type DecisionBranch struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Condition   Condition              `json:"condition"`
	NextSteps   []string               `json:"next_steps,omitempty"`
	Workflow    string                 `json:"workflow,omitempty"`     // Sub-workflow to execute
	Priority    int                    `json:"priority"`               // Higher priority evaluated first
}

// ConditionEvaluator evaluates conditions against workflow state
type ConditionEvaluator struct {
	stateManager *StateManager
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(stateManager *StateManager) *ConditionEvaluator {
	return &ConditionEvaluator{
		stateManager: stateManager,
	}
}

// Evaluate evaluates a condition against workflow state
func (ce *ConditionEvaluator) Evaluate(ctx context.Context, workflowID string, condition Condition) (bool, error) {
	// Get workflow state
	state, err := ce.stateManager.GetState(ctx, workflowID)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow state: %w", err)
	}

	return ce.evaluateCondition(condition, state)
}

// evaluateCondition recursively evaluates a condition
func (ce *ConditionEvaluator) evaluateCondition(condition Condition, state map[string]interface{}) (bool, error) {
	switch condition.Type {
	case ConditionTypeSimple:
		return ce.evaluateSimpleCondition(condition, state)
	case ConditionTypeExpression:
		return ce.evaluateExpression(condition.Expression, state)
	case ConditionTypeCustom:
		return ce.evaluateCustomCondition(condition, state)
	default:
		// Handle logical operators
		if len(condition.And) > 0 {
			return ce.evaluateAndConditions(condition.And, state)
		}
		if len(condition.Or) > 0 {
			return ce.evaluateOrConditions(condition.Or, state)
		}
		if condition.Not != nil {
			result, err := ce.evaluateCondition(*condition.Not, state)
			return !result, err
		}
		return false, fmt.Errorf("invalid condition type: %s", condition.Type)
	}
}

// evaluateSimpleCondition evaluates a simple field comparison
func (ce *ConditionEvaluator) evaluateSimpleCondition(condition Condition, state map[string]interface{}) (bool, error) {
	// Get field value from state
	fieldValue := ce.getFieldValue(condition.Field, state)
	
	// Compare based on operator
	switch condition.Operator {
	case OpEqual:
		return ce.compareEqual(fieldValue, condition.Value), nil
	case OpNotEqual:
		return !ce.compareEqual(fieldValue, condition.Value), nil
	case OpGreater:
		return ce.compareNumeric(fieldValue, condition.Value, ">"), nil
	case OpGreaterOrEqual:
		return ce.compareNumeric(fieldValue, condition.Value, ">="), nil
	case OpLess:
		return ce.compareNumeric(fieldValue, condition.Value, "<"), nil
	case OpLessOrEqual:
		return ce.compareNumeric(fieldValue, condition.Value, "<="), nil
	case OpContains:
		return ce.contains(fieldValue, condition.Value), nil
	case OpNotContains:
		return !ce.contains(fieldValue, condition.Value), nil
	case OpIn:
		return ce.inArray(fieldValue, condition.Value), nil
	case OpNotIn:
		return !ce.inArray(fieldValue, condition.Value), nil
	case OpMatches:
		return ce.matchesRegex(fieldValue, condition.Value), nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", condition.Operator)
	}
}

// getFieldValue retrieves a field value from state (supports nested fields)
func (ce *ConditionEvaluator) getFieldValue(field string, state map[string]interface{}) interface{} {
	parts := strings.Split(field, ".")
	current := interface{}(state)
	
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}
	
	return current
}

// compareEqual compares two values for equality
func (ce *ConditionEvaluator) compareEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// compareNumeric performs numeric comparison
func (ce *ConditionEvaluator) compareNumeric(a, b interface{}, op string) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	
	if !aOk || !bOk {
		return false
	}
	
	switch op {
	case ">":
		return aFloat > bFloat
	case ">=":
		return aFloat >= bFloat
	case "<":
		return aFloat < bFloat
	case "<=":
		return aFloat <= bFloat
	default:
		return false
	}
}

// contains checks if a contains b
func (ce *ConditionEvaluator) contains(a, b interface{}) bool {
	aStr, aOk := a.(string)
	bStr, bOk := b.(string)
	
	if aOk && bOk {
		return strings.Contains(aStr, bStr)
	}
	
	// Check array contains
	if arr, ok := a.([]interface{}); ok {
		for _, item := range arr {
			if ce.compareEqual(item, b) {
				return true
			}
		}
	}
	
	return false
}

// inArray checks if a is in array b
func (ce *ConditionEvaluator) inArray(a, b interface{}) bool {
	arr, ok := b.([]interface{})
	if !ok {
		return false
	}
	
	for _, item := range arr {
		if ce.compareEqual(a, item) {
			return true
		}
	}
	
	return false
}

// matchesRegex checks if a matches regex pattern b
func (ce *ConditionEvaluator) matchesRegex(a, b interface{}) bool {
	// Implementation would use regexp package
	// Simplified for now
	return false
}

// evaluateAndConditions evaluates AND conditions
func (ce *ConditionEvaluator) evaluateAndConditions(conditions []Condition, state map[string]interface{}) (bool, error) {
	for _, cond := range conditions {
		result, err := ce.evaluateCondition(cond, state)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

// evaluateOrConditions evaluates OR conditions
func (ce *ConditionEvaluator) evaluateOrConditions(conditions []Condition, state map[string]interface{}) (bool, error) {
	for _, cond := range conditions {
		result, err := ce.evaluateCondition(cond, state)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

// evaluateExpression evaluates a complex expression
func (ce *ConditionEvaluator) evaluateExpression(expression string, state map[string]interface{}) (bool, error) {
	// This would use an expression evaluator library
	// For now, simplified implementation
	return false, fmt.Errorf("expression evaluation not implemented")
}

// evaluateCustomCondition evaluates a custom condition
func (ce *ConditionEvaluator) evaluateCustomCondition(condition Condition, state map[string]interface{}) (bool, error) {
	// This would call a custom function
	// For now, return error
	return false, fmt.Errorf("custom conditions not implemented")
}

// toFloat64 converts interface to float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// EvaluateDecisionNode evaluates a decision node and returns the selected branch
func (ce *ConditionEvaluator) EvaluateDecisionNode(ctx context.Context, workflowID string, node DecisionNode) (*DecisionBranch, error) {
	// Sort branches by priority
	sortedBranches := make([]DecisionBranch, len(node.Branches))
	copy(sortedBranches, node.Branches)
	
	// Evaluate branches in priority order
	for i := 0; i < len(sortedBranches); i++ {
		for j := i + 1; j < len(sortedBranches); j++ {
			if sortedBranches[j].Priority > sortedBranches[i].Priority {
				sortedBranches[i], sortedBranches[j] = sortedBranches[j], sortedBranches[i]
			}
		}
	}
	
	// Evaluate conditions
	for _, branch := range sortedBranches {
		result, err := ce.Evaluate(ctx, workflowID, branch.Condition)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate branch %s: %w", branch.ID, err)
		}
		
		if result {
			return &branch, nil
		}
	}
	
	// Return default branch if no match
	if node.Default != "" {
		for _, branch := range node.Branches {
			if branch.ID == node.Default {
				return &branch, nil
			}
		}
	}
	
	return nil, fmt.Errorf("no matching branch found for decision node %s", node.ID)
}
