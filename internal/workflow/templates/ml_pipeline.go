package templates

import (
	"github.com/agentainer/agentainer-lab/internal/workflow"
)

// GetMLPipelineTemplates returns machine learning pipeline templates
func GetMLPipelineTemplates() []workflow.WorkflowTemplate {
	return []workflow.WorkflowTemplate{
		{
			Name:        "model-training-pipeline",
			Description: "Complete ML model training pipeline with data prep, training, and evaluation",
			Category:    "ml-pipeline",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     5,
				Timeout:         "4h",
				FailureStrategy: "fail_fast",
				EnableProfiling: true,
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "download-data",
					Name: "Download Dataset",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "curlimages/curl:7.81.0",
						Command: []string{"curl", "-o", "/data/dataset.tar.gz", "$DATASET_URL"},
						EnvVars: map[string]string{
							"DATASET_URL": "",
						},
						Timeout: "30m",
					},
				},
				{
					ID:        "extract-data",
					Name:      "Extract Dataset",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"download-data"},
					Config: workflow.StepConfig{
						Image:   "busybox:latest",
						Command: []string{"tar", "-xzf", "/data/dataset.tar.gz", "-C", "/data"},
					},
				},
				{
					ID:        "preprocess-data",
					Name:      "Preprocess Data",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"extract-data"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/preprocess.py"},
						EnvVars: map[string]string{
							"INPUT_DIR":  "/data/raw",
							"OUTPUT_DIR": "/data/processed",
						},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    2,
							MemoryLimit: 4096 * 1024 * 1024, // Convert MB to bytes
						},
					},
				},
				{
					ID:        "split-data",
					Name:      "Split Train/Test Data",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"preprocess-data"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/split_data.py"},
						EnvVars: map[string]string{
							"TRAIN_RATIO": "0.8",
							"VAL_RATIO":   "0.1",
							"TEST_RATIO":  "0.1",
						},
					},
				},
				{
					ID:        "train-model",
					Name:      "Train Model",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"split-data"},
					Config: workflow.StepConfig{
						Image:   "tensorflow/tensorflow:2.8.0-gpu",
						Command: []string{"python", "/scripts/train.py"},
						EnvVars: map[string]string{
							"MODEL_TYPE":  "$MODEL_TYPE",
							"EPOCHS":      "$EPOCHS",
							"BATCH_SIZE":  "$BATCH_SIZE",
							"LEARNING_RATE": "$LEARNING_RATE",
						},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    8,
							MemoryLimit: 16384 * 1024 * 1024, // Convert MB to bytes
							GPULimit:    1,
						},
						Timeout: "2h",
					},
				},
				{
					ID:        "evaluate-model",
					Name:      "Evaluate Model",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"train-model"},
					Config: workflow.StepConfig{
						Image:   "tensorflow/tensorflow:2.8.0",
						Command: []string{"python", "/scripts/evaluate.py"},
						EnvVars: map[string]string{
							"MODEL_PATH": "/models/trained_model",
							"TEST_DATA":  "/data/test",
						},
					},
				},
				{
					ID:        "save-model",
					Name:      "Save Model",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"evaluate-model"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/save_model.py"},
						EnvVars: map[string]string{
							"MODEL_PATH":   "/models/trained_model",
							"REGISTRY_URL": "$MODEL_REGISTRY_URL",
						},
					},
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dataset_url": map[string]interface{}{
						"type":        "string",
						"description": "URL to download the dataset",
					},
					"model_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of model to train",
						"enum":        []string{"cnn", "lstm", "transformer", "bert"},
					},
					"hyperparameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"epochs": map[string]interface{}{
								"type":    "integer",
								"default": 10,
							},
							"batch_size": map[string]interface{}{
								"type":    "integer",
								"default": 32,
							},
							"learning_rate": map[string]interface{}{
								"type":    "number",
								"default": 0.001,
							},
						},
					},
				},
				"required": []string{"dataset_url", "model_type"},
			},
			Tags: []string{"ml", "training", "tensorflow", "gpu"},
		},
		{
			Name:        "hyperparameter-tuning",
			Description: "Distributed hyperparameter tuning with multiple parallel experiments",
			Category:    "ml-pipeline",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     20,
				Timeout:         "8h",
				FailureStrategy: "continue",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "generate-configs",
					Name: "Generate Hyperparameter Configurations",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/generate_hp_configs.py"},
						EnvVars: map[string]string{
							"SEARCH_SPACE": "$SEARCH_SPACE_JSON",
							"NUM_TRIALS":   "$NUM_TRIALS",
						},
					},
				},
				{
					ID:        "run-experiments",
					Name:      "Run Parallel Experiments",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"generate-configs"},
					Config: workflow.StepConfig{
						Image:      "pytorch/pytorch:1.10.0-cuda11.3-cudnn8-runtime",
						MaxWorkers: 10,
						Command:    []string{"python", "/scripts/train_experiment.py"},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    4,
							MemoryLimit: 8192 * 1024 * 1024, // Convert MB to bytes
							GPULimit:    1,
						},
						ExecutionMode: "pooled",
						PoolConfig: &workflow.PoolConfig{
							MinSize: 5,
							MaxSize: 10,
							WarmUp:  true,
						},
					},
				},
				{
					ID:        "analyze-results",
					Name:      "Analyze Results",
					Type:      workflow.StepTypeReduce,
					DependsOn: []string{"run-experiments"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/analyze_results.py"},
					},
				},
				{
					ID:        "select-best",
					Name:      "Select Best Configuration",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"analyze-results"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/select_best.py"},
					},
				},
			},
			Tags: []string{"ml", "hyperparameter-tuning", "distributed", "gpu"},
		},
		{
			Name:        "inference-pipeline",
			Description: "Scalable model inference pipeline for batch predictions",
			Category:    "ml-pipeline",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     50,
				Timeout:         "2h",
				FailureStrategy: "continue",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "load-model",
					Name: "Load Model",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/load_model.py"},
						EnvVars: map[string]string{
							"MODEL_URL": "$MODEL_URL",
						},
					},
				},
				{
					ID:        "batch-inference",
					Name:      "Batch Inference",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"load-model"},
					Config: workflow.StepConfig{
						Image:      "tensorflow/serving:2.8.0",
						MaxWorkers: 20,
						Command:    []string{"python", "/scripts/batch_predict.py"},
						ResourceLimits: &workflow.ResourceLimits{
							CPULimit:    2,
							MemoryLimit: 4096 * 1024 * 1024, // Convert MB to bytes
						},
						ExecutionMode: "pooled",
						PoolConfig: &workflow.PoolConfig{
							MinSize: 10,
							MaxSize: 20,
							WarmUp:  true,
						},
					},
				},
				{
					ID:        "post-process",
					Name:      "Post-process Predictions",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"batch-inference"},
					Config: workflow.StepConfig{
						Image:      "python:3.9",
						MaxWorkers: 10,
						Command:    []string{"python", "/scripts/postprocess.py"},
					},
				},
				{
					ID:        "save-results",
					Name:      "Save Results",
					Type:      workflow.StepTypeReduce,
					DependsOn: []string{"post-process"},
					Config: workflow.StepConfig{
						Image:   "python:3.9",
						Command: []string{"python", "/scripts/save_results.py"},
						EnvVars: map[string]string{
							"OUTPUT_FORMAT": "$OUTPUT_FORMAT",
						},
					},
				},
			},
			Tags: []string{"ml", "inference", "batch-processing", "scalable"},
		},
	}
}