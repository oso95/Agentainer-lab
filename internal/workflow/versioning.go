package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// WorkflowVersion represents a specific version of a workflow
type WorkflowVersion struct {
	ID            string                 `json:"id"`
	WorkflowID    string                 `json:"workflow_id"`
	Version       string                 `json:"version"`        // Semantic version
	Major         int                    `json:"major"`
	Minor         int                    `json:"minor"`
	Patch         int                    `json:"patch"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	ChangeLog     string                 `json:"changelog"`
	Config        WorkflowConfig         `json:"config"`
	Steps         []WorkflowStep         `json:"steps"`
	Metadata      map[string]string      `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	CreatedBy     string                 `json:"created_by"`
	IsLatest      bool                   `json:"is_latest"`
	IsStable      bool                   `json:"is_stable"`
	Deprecated    bool                   `json:"deprecated"`
	DeprecationInfo *DeprecationInfo     `json:"deprecation_info,omitempty"`
}

// DeprecationInfo contains information about deprecated versions
type DeprecationInfo struct {
	Reason          string    `json:"reason"`
	Alternative     string    `json:"alternative"`      // Recommended version to use
	DeprecatedAt    time.Time `json:"deprecated_at"`
	EndOfLifeAt     *time.Time `json:"end_of_life_at,omitempty"`
}

// VersionManager manages workflow versions
type VersionManager struct {
	redisClient *redis.Client
}

// NewVersionManager creates a new version manager
func NewVersionManager(redisClient *redis.Client) *VersionManager {
	return &VersionManager{
		redisClient: redisClient,
	}
}

// CreateVersion creates a new version of a workflow
func (vm *VersionManager) CreateVersion(ctx context.Context, workflowID string, version string, workflow *Workflow, changeLog string, createdBy string) (*WorkflowVersion, error) {
	// Parse semantic version
	major, minor, patch, err := parseSemanticVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}

	// Check if version already exists
	existingKey := fmt.Sprintf("workflow:%s:version:%s", workflowID, version)
	exists, err := vm.redisClient.Exists(ctx, existingKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check version existence: %w", err)
	}
	if exists > 0 {
		return nil, fmt.Errorf("version %s already exists", version)
	}

	// Create version object
	wfVersion := &WorkflowVersion{
		ID:          fmt.Sprintf("%s-v%s", workflowID, version),
		WorkflowID:  workflowID,
		Version:     version,
		Major:       major,
		Minor:       minor,
		Patch:       patch,
		Name:        workflow.Name,
		Description: workflow.Description,
		ChangeLog:   changeLog,
		Config:      workflow.Config,
		Steps:       workflow.Steps,
		Metadata:    workflow.Metadata,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
		IsLatest:    false,
		IsStable:    false,
		Deprecated:  false,
	}

	// Save version
	data, err := json.Marshal(wfVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal version: %w", err)
	}

	if err := vm.redisClient.Set(ctx, existingKey, data, 0).Err(); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Add to version list
	versionListKey := fmt.Sprintf("workflow:%s:versions", workflowID)
	if err := vm.redisClient.ZAdd(ctx, versionListKey, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: version,
	}).Err(); err != nil {
		return nil, fmt.Errorf("failed to add version to list: %w", err)
	}

	// Update latest version if this is newer
	if err := vm.updateLatestVersion(ctx, workflowID, wfVersion); err != nil {
		return nil, fmt.Errorf("failed to update latest version: %w", err)
	}

	return wfVersion, nil
}

// GetVersion retrieves a specific version of a workflow
func (vm *VersionManager) GetVersion(ctx context.Context, workflowID, version string) (*WorkflowVersion, error) {
	key := fmt.Sprintf("workflow:%s:version:%s", workflowID, version)
	data, err := vm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("version %s not found", version)
		}
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	var wfVersion WorkflowVersion
	if err := json.Unmarshal([]byte(data), &wfVersion); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &wfVersion, nil
}

// GetLatestVersion retrieves the latest version of a workflow
func (vm *VersionManager) GetLatestVersion(ctx context.Context, workflowID string) (*WorkflowVersion, error) {
	key := fmt.Sprintf("workflow:%s:latest", workflowID)
	version, err := vm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no versions found for workflow %s", workflowID)
		}
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	return vm.GetVersion(ctx, workflowID, version)
}

// GetStableVersion retrieves the latest stable version
func (vm *VersionManager) GetStableVersion(ctx context.Context, workflowID string) (*WorkflowVersion, error) {
	key := fmt.Sprintf("workflow:%s:stable", workflowID)
	version, err := vm.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no stable version found for workflow %s", workflowID)
		}
		return nil, fmt.Errorf("failed to get stable version: %w", err)
	}

	return vm.GetVersion(ctx, workflowID, version)
}

// ListVersions lists all versions of a workflow
func (vm *VersionManager) ListVersions(ctx context.Context, workflowID string) ([]*WorkflowVersion, error) {
	key := fmt.Sprintf("workflow:%s:versions", workflowID)
	versions, err := vm.redisClient.ZRevRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	var wfVersions []*WorkflowVersion
	for _, version := range versions {
		wfVersion, err := vm.GetVersion(ctx, workflowID, version)
		if err != nil {
			continue
		}
		wfVersions = append(wfVersions, wfVersion)
	}

	return wfVersions, nil
}

// MarkAsStable marks a version as stable
func (vm *VersionManager) MarkAsStable(ctx context.Context, workflowID, version string) error {
	// Get version
	wfVersion, err := vm.GetVersion(ctx, workflowID, version)
	if err != nil {
		return err
	}

	// Update stable flag
	wfVersion.IsStable = true

	// Save updated version
	data, err := json.Marshal(wfVersion)
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	key := fmt.Sprintf("workflow:%s:version:%s", workflowID, version)
	if err := vm.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	// Update stable pointer
	stableKey := fmt.Sprintf("workflow:%s:stable", workflowID)
	if err := vm.redisClient.Set(ctx, stableKey, version, 0).Err(); err != nil {
		return fmt.Errorf("failed to update stable pointer: %w", err)
	}

	return nil
}

// DeprecateVersion marks a version as deprecated
func (vm *VersionManager) DeprecateVersion(ctx context.Context, workflowID, version string, info DeprecationInfo) error {
	// Get version
	wfVersion, err := vm.GetVersion(ctx, workflowID, version)
	if err != nil {
		return err
	}

	// Update deprecation status
	wfVersion.Deprecated = true
	wfVersion.DeprecationInfo = &info

	// Save updated version
	data, err := json.Marshal(wfVersion)
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	key := fmt.Sprintf("workflow:%s:version:%s", workflowID, version)
	if err := vm.redisClient.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

// CompareVersions compares two versions and returns the differences
func (vm *VersionManager) CompareVersions(ctx context.Context, workflowID, version1, version2 string) (*VersionComparison, error) {
	v1, err := vm.GetVersion(ctx, workflowID, version1)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s: %w", version1, err)
	}

	v2, err := vm.GetVersion(ctx, workflowID, version2)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s: %w", version2, err)
	}

	comparison := &VersionComparison{
		Version1: version1,
		Version2: version2,
		Changes:  []VersionChange{},
	}

	// Compare steps
	stepMap1 := make(map[string]*WorkflowStep)
	for i := range v1.Steps {
		stepMap1[v1.Steps[i].ID] = &v1.Steps[i]
	}

	stepMap2 := make(map[string]*WorkflowStep)
	for i := range v2.Steps {
		stepMap2[v2.Steps[i].ID] = &v2.Steps[i]
	}

	// Find added steps
	for id, step := range stepMap2 {
		if _, exists := stepMap1[id]; !exists {
			comparison.Changes = append(comparison.Changes, VersionChange{
				Type:        ChangeTypeAdded,
				Component:   "step",
				ComponentID: id,
				Description: fmt.Sprintf("Added step: %s", step.Name),
			})
		}
	}

	// Find removed steps
	for id, step := range stepMap1 {
		if _, exists := stepMap2[id]; !exists {
			comparison.Changes = append(comparison.Changes, VersionChange{
				Type:        ChangeTypeRemoved,
				Component:   "step",
				ComponentID: id,
				Description: fmt.Sprintf("Removed step: %s", step.Name),
			})
		}
	}

	// Find modified steps
	for id, step2 := range stepMap2 {
		if step1, exists := stepMap1[id]; exists {
			// Compare step configurations
			if !stepsEqual(step1, step2) {
				comparison.Changes = append(comparison.Changes, VersionChange{
					Type:        ChangeTypeModified,
					Component:   "step",
					ComponentID: id,
					Description: fmt.Sprintf("Modified step: %s", step2.Name),
				})
			}
		}
	}

	// Compare workflow config
	if !configsEqual(&v1.Config, &v2.Config) {
		comparison.Changes = append(comparison.Changes, VersionChange{
			Type:        ChangeTypeModified,
			Component:   "config",
			Description: "Workflow configuration changed",
		})
	}

	return comparison, nil
}

// updateLatestVersion updates the latest version pointer if needed
func (vm *VersionManager) updateLatestVersion(ctx context.Context, workflowID string, newVersion *WorkflowVersion) error {
	latestKey := fmt.Sprintf("workflow:%s:latest", workflowID)
	currentLatest, err := vm.redisClient.Get(ctx, latestKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	// If no latest version or new version is higher
	if err == redis.Nil || compareVersions(newVersion.Version, currentLatest) > 0 {
		// Update latest pointer
		if err := vm.redisClient.Set(ctx, latestKey, newVersion.Version, 0).Err(); err != nil {
			return err
		}

		// Update isLatest flag
		newVersion.IsLatest = true
		data, _ := json.Marshal(newVersion)
		versionKey := fmt.Sprintf("workflow:%s:version:%s", workflowID, newVersion.Version)
		vm.redisClient.Set(ctx, versionKey, data, 0)

		// Remove isLatest from previous version
		if currentLatest != "" {
			if oldVersion, err := vm.GetVersion(ctx, workflowID, currentLatest); err == nil {
				oldVersion.IsLatest = false
				data, _ := json.Marshal(oldVersion)
				oldKey := fmt.Sprintf("workflow:%s:version:%s", workflowID, currentLatest)
				vm.redisClient.Set(ctx, oldKey, data, 0)
			}
		}
	}

	return nil
}

// VersionComparison contains the differences between two versions
type VersionComparison struct {
	Version1 string          `json:"version1"`
	Version2 string          `json:"version2"`
	Changes  []VersionChange `json:"changes"`
}

// VersionChange represents a single change between versions
type VersionChange struct {
	Type        ChangeType `json:"type"`
	Component   string     `json:"component"`
	ComponentID string     `json:"component_id,omitempty"`
	Description string     `json:"description"`
}

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeRemoved  ChangeType = "removed"
	ChangeTypeModified ChangeType = "modified"
)

// parseSemanticVersion parses a semantic version string
func parseSemanticVersion(version string) (major, minor, patch int, err error) {
	_, err = fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	return
}

// compareVersions compares two semantic versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	major1, minor1, patch1, _ := parseSemanticVersion(v1)
	major2, minor2, patch2, _ := parseSemanticVersion(v2)

	if major1 != major2 {
		if major1 < major2 {
			return -1
		}
		return 1
	}

	if minor1 != minor2 {
		if minor1 < minor2 {
			return -1
		}
		return 1
	}

	if patch1 != patch2 {
		if patch1 < patch2 {
			return -1
		}
		return 1
	}

	return 0
}

// stepsEqual compares two workflow steps for equality
func stepsEqual(s1, s2 *WorkflowStep) bool {
	// Simplified comparison - in production would be more thorough
	return s1.Name == s2.Name &&
		s1.Type == s2.Type &&
		s1.Config.Image == s2.Config.Image
}

// configsEqual compares two workflow configs for equality
func configsEqual(c1, c2 *WorkflowConfig) bool {
	// Simplified comparison
	return c1.MaxParallel == c2.MaxParallel &&
		c1.Timeout == c2.Timeout &&
		c1.FailureStrategy == c2.FailureStrategy
}
