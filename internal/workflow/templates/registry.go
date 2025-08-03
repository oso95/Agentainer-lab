package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentainer/agentainer-lab/internal/workflow"
	"github.com/go-redis/redis/v8"
)

// TemplateRegistry manages workflow templates
type TemplateRegistry struct {
	redisClient         *redis.Client
	subWorkflowExecutor *workflow.SubWorkflowExecutor
	templates           map[string]workflow.WorkflowTemplate
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry(redisClient *redis.Client, subWorkflowExecutor *workflow.SubWorkflowExecutor) *TemplateRegistry {
	registry := &TemplateRegistry{
		redisClient:         redisClient,
		subWorkflowExecutor: subWorkflowExecutor,
		templates:           make(map[string]workflow.WorkflowTemplate),
	}

	// Load built-in templates
	registry.loadBuiltInTemplates()

	return registry
}

// loadBuiltInTemplates loads all built-in templates
func (tr *TemplateRegistry) loadBuiltInTemplates() {
	// Load data processing templates
	for _, template := range GetDataProcessingTemplates() {
		tr.templates[template.Name] = template
	}

	// Load ML pipeline templates
	for _, template := range GetMLPipelineTemplates() {
		tr.templates[template.Name] = template
	}

	// Load DevOps templates
	for _, template := range GetDevOpsTemplates() {
		tr.templates[template.Name] = template
	}
}

// RegisterTemplate registers a new template
func (tr *TemplateRegistry) RegisterTemplate(ctx context.Context, template workflow.WorkflowTemplate) error {
	// Validate template
	if err := tr.validateTemplate(template); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Store in registry
	tr.templates[template.Name] = template

	// Create as workflow template
	if tr.subWorkflowExecutor != nil {
		return tr.subWorkflowExecutor.CreateWorkflowTemplate(ctx, &template)
	}

	return nil
}

// GetTemplate retrieves a template by name
func (tr *TemplateRegistry) GetTemplate(name string) (*workflow.WorkflowTemplate, error) {
	template, exists := tr.templates[name]
	if !exists {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return &template, nil
}

// ListTemplates returns all available templates
func (tr *TemplateRegistry) ListTemplates() []workflow.WorkflowTemplate {
	templates := make([]workflow.WorkflowTemplate, 0, len(tr.templates))
	for _, template := range tr.templates {
		templates = append(templates, template)
	}
	return templates
}

// ListTemplatesByCategory returns templates in a specific category
func (tr *TemplateRegistry) ListTemplatesByCategory(category string) []workflow.WorkflowTemplate {
	templates := []workflow.WorkflowTemplate{}
	for _, template := range tr.templates {
		if template.Category == category {
			templates = append(templates, template)
		}
	}
	return templates
}

// SearchTemplates searches templates by tags
func (tr *TemplateRegistry) SearchTemplates(tags []string) []workflow.WorkflowTemplate {
	templates := []workflow.WorkflowTemplate{}
	for _, template := range tr.templates {
		if tr.hasMatchingTags(template.Tags, tags) {
			templates = append(templates, template)
		}
	}
	return templates
}

// InstantiateTemplate creates a workflow instance from a template
func (tr *TemplateRegistry) InstantiateTemplate(ctx context.Context, templateName, instanceName string, inputData map[string]interface{}) (*workflow.Workflow, error) {
	template, err := tr.GetTemplate(templateName)
	if err != nil {
		return nil, err
	}

	// Validate input against schema
	if err := tr.validateInput(template.InputSchema, inputData); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Create workflow from template
	if tr.subWorkflowExecutor != nil {
		return tr.subWorkflowExecutor.InstantiateTemplate(ctx, templateName, instanceName, inputData)
	}

	// Fallback: create manually
	workflow := &workflow.Workflow{
		Name:        instanceName,
		Description: fmt.Sprintf("Instance of %s", template.Description),
		Config:      template.Config,
		Steps:       template.Steps,
		State:       inputData,
		Metadata: map[string]string{
			"template_name":    templateName,
			"template_version": template.Version,
		},
	}

	return workflow, nil
}

// GetCategories returns all available template categories
func (tr *TemplateRegistry) GetCategories() []string {
	categoryMap := make(map[string]bool)
	for _, template := range tr.templates {
		categoryMap[template.Category] = true
	}

	categories := make([]string, 0, len(categoryMap))
	for category := range categoryMap {
		categories = append(categories, category)
	}
	return categories
}

// GetTemplatesByVersion returns all versions of a template
func (tr *TemplateRegistry) GetTemplatesByVersion(name string) []workflow.WorkflowTemplate {
	templates := []workflow.WorkflowTemplate{}
	prefix := name + "@"
	
	for templateName, template := range tr.templates {
		if templateName == name || strings.HasPrefix(templateName, prefix) {
			templates = append(templates, template)
		}
	}
	return templates
}

// ExportTemplate exports a template in various formats
func (tr *TemplateRegistry) ExportTemplate(templateName, format string) ([]byte, error) {
	template, err := tr.GetTemplate(templateName)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return json.Marshal(template)
	case "yaml":
		// Would use yaml.Marshal if yaml package is available
		return nil, fmt.Errorf("YAML export not implemented")
	case "helm":
		return tr.exportAsHelmChart(template)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Helper methods

func (tr *TemplateRegistry) validateTemplate(template workflow.WorkflowTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Category == "" {
		return fmt.Errorf("template category is required")
	}
	if template.Version == "" {
		return fmt.Errorf("template version is required")
	}
	if len(template.Steps) == 0 {
		return fmt.Errorf("template must have at least one step")
	}
	return nil
}

func (tr *TemplateRegistry) validateInput(schema map[string]interface{}, input map[string]interface{}) error {
	// Simplified validation - in production would use a JSON schema validator
	if schema == nil {
		return nil
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	required, _ := schema["required"].([]string)
	
	// Check required fields
	for _, field := range required {
		if _, exists := input[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate field types
	for field, value := range input {
		if propSchema, exists := properties[field].(map[string]interface{}); exists {
			if err := tr.validateFieldType(field, value, propSchema); err != nil {
				return err
			}
		}
	}

	return nil
}

func (tr *TemplateRegistry) validateFieldType(field string, value interface{}, schema map[string]interface{}) error {
	expectedType, _ := schema["type"].(string)
	
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s must be a string", field)
		}
	case "integer":
		switch value.(type) {
		case int, int32, int64, float64:
			// Accept numeric types
		default:
			return fmt.Errorf("field %s must be an integer", field)
		}
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Accept numeric types
		default:
			return fmt.Errorf("field %s must be a number", field)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %s must be a boolean", field)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field %s must be an object", field)
		}
	}

	return nil
}

func (tr *TemplateRegistry) hasMatchingTags(templateTags, searchTags []string) bool {
	for _, searchTag := range searchTags {
		found := false
		for _, templateTag := range templateTags {
			if templateTag == searchTag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (tr *TemplateRegistry) exportAsHelmChart(template *workflow.WorkflowTemplate) ([]byte, error) {
	// Simplified Helm chart generation
	helmChart := fmt.Sprintf(`apiVersion: v2
name: %s
description: %s
version: %s
type: application

# This would be a complete Helm chart in production
`, template.Name, template.Description, template.Version)

	return []byte(helmChart), nil
}

// TemplateMetrics tracks template usage
type TemplateMetrics struct {
	TemplateName   string `json:"template_name"`
	UsageCount     int    `json:"usage_count"`
	SuccessCount   int    `json:"success_count"`
	FailureCount   int    `json:"failure_count"`
	AvgDuration    int64  `json:"avg_duration_ms"`
	LastUsed       string `json:"last_used"`
}

// GetTemplateMetrics returns usage metrics for templates
func (tr *TemplateRegistry) GetTemplateMetrics(ctx context.Context) ([]TemplateMetrics, error) {
	// This would query actual usage data from Redis
	// Simplified for the example
	metrics := []TemplateMetrics{}
	
	for name := range tr.templates {
		metrics = append(metrics, TemplateMetrics{
			TemplateName: name,
			UsageCount:   0,
			SuccessCount: 0,
			FailureCount: 0,
			AvgDuration:  0,
			LastUsed:     "",
		})
	}
	
	return metrics, nil
}