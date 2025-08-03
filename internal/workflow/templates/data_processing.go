package templates

import (
	"github.com/agentainer/agentainer-lab/internal/workflow"
)

// GetDataProcessingTemplates returns data processing workflow templates
func GetDataProcessingTemplates() []workflow.WorkflowTemplate {
	return []workflow.WorkflowTemplate{
		{
			Name:        "batch-file-processor",
			Description: "Process files in batches with parallel workers",
			Category:    "data-processing",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     10,
				Timeout:         "1h",
				FailureStrategy: "continue",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "list-files",
					Name: "List Files",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "busybox:latest",
						Command: []string{"find", "/data", "-type", "f"},
						EnvVars: map[string]string{
							"INPUT_DIR": "/data",
						},
					},
				},
				{
					ID:        "process-files",
					Name:      "Process Files",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"list-files"},
					Config: workflow.StepConfig{
						Image:      "python:3.9-slim",
						MaxWorkers: 10,
						Command:    []string{"python", "/scripts/process_file.py"},
						EnvVars: map[string]string{
							"BATCH_SIZE": "100",
						},
					},
				},
				{
					ID:        "aggregate-results",
					Name:      "Aggregate Results",
					Type:      workflow.StepTypeReduce,
					DependsOn: []string{"process-files"},
					Config: workflow.StepConfig{
						Image:   "python:3.9-slim",
						Command: []string{"python", "/scripts/aggregate.py"},
					},
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input_dir": map[string]interface{}{
						"type":        "string",
						"description": "Directory containing input files",
					},
					"output_dir": map[string]interface{}{
						"type":        "string",
						"description": "Directory for output files",
					},
					"batch_size": map[string]interface{}{
						"type":        "integer",
						"description": "Number of files to process per batch",
						"default":     100,
					},
				},
				"required": []string{"input_dir", "output_dir"},
			},
			Tags: []string{"batch", "parallel", "file-processing"},
		},
		{
			Name:        "etl-pipeline",
			Description: "Extract, Transform, Load data pipeline",
			Category:    "data-processing",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     5,
				Timeout:         "2h",
				FailureStrategy: "fail_fast",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "extract",
					Name: "Extract Data",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image: "postgres:13",
						Command: []string{
							"psql", "-h", "$SOURCE_HOST", "-U", "$SOURCE_USER",
							"-c", "COPY (SELECT * FROM $SOURCE_TABLE) TO STDOUT WITH CSV HEADER",
						},
						EnvVars: map[string]string{
							"PGPASSWORD": "$SOURCE_PASSWORD",
						},
						Timeout: "30m",
					},
				},
				{
					ID:        "transform",
					Name:      "Transform Data",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"extract"},
					Config: workflow.StepConfig{
						Image:   "apache/spark:3.2.0",
						Command: []string{"spark-submit", "/scripts/transform.py"},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    4,
							MemoryLimit: 8192 * 1024 * 1024, // Convert MB to bytes
						},
					},
				},
				{
					ID:        "validate",
					Name:      "Validate Data",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"transform"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/validate.py"},
					},
				},
				{
					ID:        "load",
					Name:      "Load Data",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"validate"},
					Config: workflow.StepConfig{
						Image: "postgres:13",
						Command: []string{
							"psql", "-h", "$TARGET_HOST", "-U", "$TARGET_USER",
							"-c", "COPY $TARGET_TABLE FROM STDIN WITH CSV HEADER",
						},
						EnvVars: map[string]string{
							"PGPASSWORD": "$TARGET_PASSWORD",
						},
					},
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"host":     map[string]interface{}{"type": "string"},
							"database": map[string]interface{}{"type": "string"},
							"table":    map[string]interface{}{"type": "string"},
							"user":     map[string]interface{}{"type": "string"},
							"password": map[string]interface{}{"type": "string"},
						},
						"required": []string{"host", "database", "table", "user", "password"},
					},
					"target": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"host":     map[string]interface{}{"type": "string"},
							"database": map[string]interface{}{"type": "string"},
							"table":    map[string]interface{}{"type": "string"},
							"user":     map[string]interface{}{"type": "string"},
							"password": map[string]interface{}{"type": "string"},
						},
						"required": []string{"host", "database", "table", "user", "password"},
					},
				},
				"required": []string{"source", "target"},
			},
			Tags: []string{"etl", "database", "migration"},
		},
		{
			Name:        "stream-processor",
			Description: "Real-time stream processing pipeline",
			Category:    "data-processing",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     20,
				Timeout:         "24h",
				FailureStrategy: "continue",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "kafka-consumer",
					Name: "Kafka Consumer",
					Type: workflow.StepTypeParallel,
					Config: workflow.StepConfig{
						Image:      "confluentinc/cp-kafka:7.0.1",
						MaxWorkers: 10,
						Command:    []string{"kafka-console-consumer", "--bootstrap-server", "$KAFKA_BROKERS"},
						EnvVars: map[string]string{
							"KAFKA_TOPIC": "$INPUT_TOPIC",
						},
					},
				},
				{
					ID:        "process-stream",
					Name:      "Process Stream",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"kafka-consumer"},
					Config: workflow.StepConfig{
						Image:      "flink:1.14",
						MaxWorkers: 5,
						Command:    []string{"flink", "run", "/app/stream-processor.jar"},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    2,
							MemoryLimit: 4096 * 1024 * 1024, // Convert MB to bytes
						},
					},
				},
				{
					ID:        "kafka-producer",
					Name:      "Kafka Producer",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"process-stream"},
					Config: workflow.StepConfig{
						Image:   "confluentinc/cp-kafka:7.0.1",
						Command: []string{"kafka-console-producer", "--bootstrap-server", "$KAFKA_BROKERS"},
						EnvVars: map[string]string{
							"KAFKA_TOPIC": "$OUTPUT_TOPIC",
						},
					},
				},
			},
			Tags: []string{"streaming", "kafka", "real-time"},
		},
	}
}