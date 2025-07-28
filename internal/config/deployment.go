package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"gopkg.in/yaml.v3"
)

// DeploymentConfig represents a YAML deployment configuration
type DeploymentConfig struct {
	APIVersion string               `yaml:"apiVersion"`
	Kind       string               `yaml:"kind"`
	Metadata   DeploymentMetadata   `yaml:"metadata"`
	Spec       DeploymentSpec       `yaml:"spec"`
}

// DeploymentMetadata contains deployment metadata
type DeploymentMetadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// DeploymentSpec contains the deployment specification
type DeploymentSpec struct {
	Agents []AgentSpec `yaml:"agents"`
}

// AgentSpec defines a single agent configuration
type AgentSpec struct {
	Name         string                 `yaml:"name"`
	Image        string                 `yaml:"image"`
	Replicas     int                    `yaml:"replicas,omitempty"`
	Env          map[string]string      `yaml:"env,omitempty"`
	Resources    ResourceSpec           `yaml:"resources,omitempty"`
	Volumes      []VolumeSpec           `yaml:"volumes,omitempty"`
	HealthCheck  *HealthCheckSpec       `yaml:"healthCheck,omitempty"`
	Persistence  *PersistenceSpec       `yaml:"persistence,omitempty"`
	AutoRestart  bool                   `yaml:"autoRestart,omitempty"`
	Token        string                 `yaml:"token,omitempty"`
	Dependencies []string               `yaml:"dependencies,omitempty"`
}

// ResourceSpec defines resource limits
type ResourceSpec struct {
	Memory string `yaml:"memory,omitempty"` // e.g., "512Mi", "2Gi"
	CPU    string `yaml:"cpu,omitempty"`    // e.g., "500m", "2"
}

// VolumeSpec defines volume mounting
type VolumeSpec struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
	ReadOnly  bool   `yaml:"readOnly,omitempty"`
}

// HealthCheckSpec defines health check configuration
type HealthCheckSpec struct {
	Endpoint string `yaml:"endpoint"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout,omitempty"`
	Retries  int    `yaml:"retries,omitempty"`
}

// PersistenceSpec defines persistence configuration
type PersistenceSpec struct {
	Enabled     bool   `yaml:"enabled"`
	RetryPolicy string `yaml:"retryPolicy,omitempty"` // "exponential", "linear", "fixed"
}

// LoadDeploymentConfig loads and parses a YAML deployment file
func LoadDeploymentConfig(filename string) (*DeploymentConfig, error) {
	// Expand environment variables in filename
	filename = os.ExpandEnv(filename)
	
	// Handle relative paths
	if !filepath.IsAbs(filename) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		filename = filepath.Join(cwd, filename)
	}

	// Read file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment file: %w", err)
	}

	// Expand environment variables in file content
	content := os.ExpandEnv(string(data))

	// Parse YAML
	var config DeploymentConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid deployment config: %w", err)
	}

	return &config, nil
}

// Validate checks if the deployment configuration is valid
func (d *DeploymentConfig) Validate() error {
	if d.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if d.Kind != "AgentDeployment" {
		return fmt.Errorf("kind must be 'AgentDeployment', got '%s'", d.Kind)
	}
	if d.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if len(d.Spec.Agents) == 0 {
		return fmt.Errorf("at least one agent must be specified")
	}

	// Validate each agent
	agentNames := make(map[string]bool)
	for i, agent := range d.Spec.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agent[%d]: name is required", i)
		}
		if agent.Image == "" {
			return fmt.Errorf("agent[%d]: image is required", i)
		}
		if agentNames[agent.Name] {
			return fmt.Errorf("duplicate agent name: %s", agent.Name)
		}
		agentNames[agent.Name] = true

		// Validate replicas
		if agent.Replicas < 0 {
			return fmt.Errorf("agent[%s]: replicas cannot be negative", agent.Name)
		}
		if agent.Replicas == 0 {
			agent.Replicas = 1 // Default to 1
		}

		// Validate dependencies
		for _, dep := range agent.Dependencies {
			if !agentNames[dep] {
				return fmt.Errorf("agent[%s]: dependency '%s' not found", agent.Name, dep)
			}
		}
	}

	return nil
}

// ConvertToAgentConfigs converts AgentSpec to agent configurations
func (a *AgentSpec) ConvertToAgentConfigs() ([]AgentConfig, error) {
	configs := []AgentConfig{}
	
	replicas := a.Replicas
	if replicas == 0 {
		replicas = 1
	}

	for i := 0; i < replicas; i++ {
		name := a.Name
		if replicas > 1 {
			name = fmt.Sprintf("%s-%d", a.Name, i+1)
		}

		// Convert resources
		var cpuLimit, memLimit int64
		if a.Resources.CPU != "" {
			cpu, err := ParseCPU(a.Resources.CPU)
			if err != nil {
				return nil, fmt.Errorf("invalid CPU limit: %w", err)
			}
			cpuLimit = cpu
		}
		if a.Resources.Memory != "" {
			mem, err := ParseMemory(a.Resources.Memory)
			if err != nil {
				return nil, fmt.Errorf("invalid memory limit: %w", err)
			}
			memLimit = mem
		}

		// Convert volumes
		volumes := []agent.VolumeMapping{}
		for _, v := range a.Volumes {
			volumes = append(volumes, agent.VolumeMapping{
				HostPath:      v.Host,
				ContainerPath: v.Container,
				ReadOnly:      v.ReadOnly,
			})
		}

		// Convert health check if specified
		var healthCheck *agent.HealthCheckConfig
		if a.HealthCheck != nil {
			healthCheck = &agent.HealthCheckConfig{
				Endpoint: a.HealthCheck.Endpoint,
				Interval: a.HealthCheck.Interval,
				Timeout:  a.HealthCheck.Timeout,
				Retries:  a.HealthCheck.Retries,
			}
		}

		config := AgentConfig{
			Name:        name,
			Image:       a.Image,
			EnvVars:     a.Env,
			CPULimit:    cpuLimit,
			MemoryLimit: memLimit,
			AutoRestart: a.AutoRestart,
			Token:       a.Token,
			Volumes:     volumes,
			HealthCheck: healthCheck,
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// AgentConfig represents a single agent configuration
type AgentConfig struct {
	Name        string
	Image       string
	EnvVars     map[string]string
	CPULimit    int64
	MemoryLimit int64
	AutoRestart bool
	Token       string
	Volumes     []agent.VolumeMapping
	HealthCheck *agent.HealthCheckConfig
}

// ParseCPU parses CPU limit strings
// Accepts formats:
//   - "0.5" or ".5" = half a CPU core  
//   - "1" or "1.0" = 1 CPU core
//   - "2" or "2.0" = 2 CPU cores
//   - "500m" = 500 millicores (0.5 CPU) - for k8s compatibility
func ParseCPU(cpu string) (int64, error) {
	cpu = strings.TrimSpace(cpu)
	if cpu == "" {
		return 0, nil
	}

	// Handle millicpu notation for k8s compatibility (e.g., "500m" = 0.5 CPU)
	if strings.HasSuffix(cpu, "m") {
		milliStr := strings.TrimSuffix(cpu, "m")
		var milli int64
		_, err := fmt.Sscanf(milliStr, "%d", &milli)
		if err != nil {
			return 0, fmt.Errorf("invalid millicpu value: %s", cpu)
		}
		// Convert millicpu to nanocpu (1 CPU = 1e9 nanocpu)
		return milli * 1e6, nil
	}

	// Handle decimal notation (e.g., "0.5", "1", "2.5")
	var cores float64
	_, err := fmt.Sscanf(cpu, "%f", &cores)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value: %s (use formats like 0.5, 1, 2)", cpu)
	}
	
	if cores <= 0 {
		return 0, fmt.Errorf("CPU value must be positive: %s", cpu)
	}
	
	return int64(cores * 1e9), nil
}

// ParseMemory parses memory limit strings
// Accepts formats:
//   - "512M" or "512m" = 512 megabytes
//   - "2G" or "2g" = 2 gigabytes  
//   - "1.5G" or "1.5g" = 1.5 gigabytes
//   - "512Mi" = 512 mebibytes (k8s style)
//   - "2Gi" = 2 gibibytes (k8s style)
func ParseMemory(mem string) (int64, error) {
	mem = strings.TrimSpace(mem)
	if mem == "" {
		return 0, nil
	}

	// Convert to uppercase for case-insensitive comparison
	upperMem := strings.ToUpper(mem)
	
	// Define suffixes with their multipliers
	// Support both simple (M, G) and k8s-style (Mi, Gi) formats
	suffixes := []struct{
		suffix string
		multiplier int64
	}{
		// K8s-style binary units (more precise)
		{"GI", 1024 * 1024 * 1024},
		{"MI", 1024 * 1024},
		{"KI", 1024},
		// Simple units (what most users expect)
		{"G", 1000 * 1000 * 1000},
		{"M", 1000 * 1000},
		{"K", 1000},
	}

	for _, s := range suffixes {
		if strings.HasSuffix(upperMem, s.suffix) {
			valueStr := mem[:len(mem)-len(s.suffix)]
			var value float64
			_, err := fmt.Sscanf(valueStr, "%f", &value)
			if err != nil {
				return 0, fmt.Errorf("invalid memory value: %s", mem)
			}
			if value <= 0 {
				return 0, fmt.Errorf("memory value must be positive: %s", mem)
			}
			return int64(value * float64(s.multiplier)), nil
		}
	}

	// No suffix means bytes (for backward compatibility)
	var bytes int64
	_, err := fmt.Sscanf(mem, "%d", &bytes)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s (use formats like 512M, 2G, 1.5G)", mem)
	}
	return bytes, nil
}