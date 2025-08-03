package templates

import (
	"github.com/agentainer/agentainer-lab/internal/workflow"
)

// GetDevOpsTemplates returns DevOps workflow templates
func GetDevOpsTemplates() []workflow.WorkflowTemplate {
	return []workflow.WorkflowTemplate{
		{
			Name:        "ci-cd-pipeline",
			Description: "Complete CI/CD pipeline with build, test, and deploy stages",
			Category:    "devops",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     10,
				Timeout:         "1h",
				FailureStrategy: "fail_fast",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "checkout",
					Name: "Checkout Code",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "alpine/git:latest",
						Command: []string{"git", "clone", "$REPO_URL", "/workspace"},
						EnvVars: map[string]string{
							"GIT_TOKEN": "$GIT_TOKEN",
						},
					},
				},
				{
					ID:        "lint",
					Name:      "Run Linters",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"checkout"},
					Config: workflow.StepConfig{
						Image:      "golangci/golangci-lint:latest",
						MaxWorkers: 3,
						Command:    []string{"golangci-lint", "run"},
					},
				},
				{
					ID:        "test",
					Name:      "Run Tests",
					Type:      workflow.StepTypeParallel,
					DependsOn: []string{"checkout"},
					Config: workflow.StepConfig{
						Image:      "golang:1.18",
						MaxWorkers: 5,
						Command:    []string{"go", "test", "./..."},
						EnvVars: map[string]string{
							"CGO_ENABLED": "0",
						},
					},
				},
				{
					ID:        "build",
					Name:      "Build Application",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"lint", "test"},
					Config: workflow.StepConfig{
						Image:   "docker:20.10",
						Command: []string{"docker", "build", "-t", "$IMAGE_NAME:$VERSION", "."},
						EnvVars: map[string]string{
							"DOCKER_BUILDKIT": "1",
						},
					},
				},
				{
					ID:        "scan",
					Name:      "Security Scan",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"build"},
					Config: workflow.StepConfig{
						Image:   "aquasec/trivy:latest",
						Command: []string{"trivy", "image", "$IMAGE_NAME:$VERSION"},
					},
				},
				{
					ID:        "push",
					Name:      "Push to Registry",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"scan"},
					Config: workflow.StepConfig{
						Image:   "docker:20.10",
						Command: []string{"docker", "push", "$IMAGE_NAME:$VERSION"},
						EnvVars: map[string]string{
							"DOCKER_REGISTRY": "$REGISTRY_URL",
						},
					},
				},
				{
					ID:        "deploy",
					Name:      "Deploy to Kubernetes",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"push"},
					Config: workflow.StepConfig{
						Image:   "bitnami/kubectl:latest",
						Command: []string{"kubectl", "apply", "-f", "/manifests/"},
						EnvVars: map[string]string{
							"KUBECONFIG": "/config/kubeconfig",
						},
						Condition: &workflow.Condition{
							Type:     workflow.ConditionTypeSimple,
							Field:    "branch",
							Operator: workflow.OpEqual,
							Value:    "main",
						},
					},
				},
			},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_url": map[string]interface{}{
						"type":        "string",
						"description": "Git repository URL",
					},
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Git branch to build",
						"default":     "main",
					},
					"image_name": map[string]interface{}{
						"type":        "string",
						"description": "Docker image name",
					},
					"version": map[string]interface{}{
						"type":        "string",
						"description": "Version tag",
					},
				},
				"required": []string{"repo_url", "image_name", "version"},
			},
			Tags: []string{"ci", "cd", "docker", "kubernetes"},
		},
		{
			Name:        "infrastructure-provisioning",
			Description: "Provision cloud infrastructure using Terraform",
			Category:    "devops",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     1,
				Timeout:         "2h",
				FailureStrategy: "fail_fast",
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "terraform-init",
					Name: "Initialize Terraform",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "hashicorp/terraform:1.1.0",
						Command: []string{"terraform", "init"},
						EnvVars: map[string]string{
							"TF_VAR_region": "$AWS_REGION",
						},
					},
				},
				{
					ID:        "terraform-plan",
					Name:      "Plan Infrastructure Changes",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"terraform-init"},
					Config: workflow.StepConfig{
						Image:   "hashicorp/terraform:1.1.0",
						Command: []string{"terraform", "plan", "-out=tfplan"},
					},
				},
				{
					ID:        "approval",
					Name:      "Manual Approval",
					Type:      workflow.StepTypeDecision,
					DependsOn: []string{"terraform-plan"},
					Config: workflow.StepConfig{
						DecisionNode: &workflow.DecisionNode{
							ID:          "approve-changes",
							Name:        "Approve Infrastructure Changes",
							Description: "Review and approve Terraform plan",
							Branches: []workflow.DecisionBranch{
								{
									ID:        "approved",
									Name:      "Approved",
									Condition: workflow.Condition{
										Type:     workflow.ConditionTypeSimple,
										Field:    "approval_status",
										Operator: workflow.OpEqual,
										Value:    "approved",
									},
									NextSteps: []string{"terraform-apply"},
								},
								{
									ID:        "rejected",
									Name:      "Rejected",
									Condition: workflow.Condition{
										Type:     workflow.ConditionTypeSimple,
										Field:    "approval_status",
										Operator: workflow.OpEqual,
										Value:    "rejected",
									},
									NextSteps: []string{},
								},
							},
						},
					},
				},
				{
					ID:        "terraform-apply",
					Name:      "Apply Infrastructure Changes",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"approval"},
					Config: workflow.StepConfig{
						Image:   "hashicorp/terraform:1.1.0",
						Command: []string{"terraform", "apply", "-auto-approve", "tfplan"},
					},
				},
				{
					ID:        "output",
					Name:      "Save Outputs",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"terraform-apply"},
					Config: workflow.StepConfig{
						Image:   "hashicorp/terraform:1.1.0",
						Command: []string{"terraform", "output", "-json"},
					},
				},
			},
			Tags: []string{"infrastructure", "terraform", "aws", "provisioning"},
		},
		{
			Name:        "backup-restore",
			Description: "Database backup and restore workflow",
			Category:    "devops",
			Version:     "1.0.0",
			Config: workflow.WorkflowConfig{
				MaxParallel:     5,
				Timeout:         "4h",
				FailureStrategy: "fail_fast",
				Schedule:        "0 2 * * *", // Daily at 2 AM
			},
			Steps: []workflow.WorkflowStep{
				{
					ID:   "create-snapshot",
					Name: "Create Database Snapshot",
					Type: workflow.StepTypeSequential,
					Config: workflow.StepConfig{
						Image:   "postgres:14",
						Command: []string{"pg_dump", "-h", "$DB_HOST", "-U", "$DB_USER", "-d", "$DB_NAME"},
						EnvVars: map[string]string{
							"PGPASSWORD": "$DB_PASSWORD",
						},
						Timeout: "1h",
					},
				},
				{
					ID:        "compress",
					Name:      "Compress Backup",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"create-snapshot"},
					Config: workflow.StepConfig{
						Image:   "alpine:latest",
						Command: []string{"gzip", "-9", "/backup/dump.sql"},
					},
				},
				{
					ID:        "encrypt",
					Name:      "Encrypt Backup",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"compress"},
					Config: workflow.StepConfig{
						Image:   "alpine:latest",
						Command: []string{"openssl", "enc", "-aes-256-cbc", "-in", "/backup/dump.sql.gz"},
						EnvVars: map[string]string{
							"ENCRYPTION_KEY": "$BACKUP_ENCRYPTION_KEY",
						},
					},
				},
				{
					ID:        "upload",
					Name:      "Upload to S3",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"encrypt"},
					Config: workflow.StepConfig{
						Image:   "amazon/aws-cli:2.4.0",
						Command: []string{"aws", "s3", "cp", "/backup/dump.sql.gz.enc", "s3://$BACKUP_BUCKET/"},
						EnvVars: map[string]string{
							"AWS_ACCESS_KEY_ID":     "$AWS_ACCESS_KEY",
							"AWS_SECRET_ACCESS_KEY": "$AWS_SECRET_KEY",
						},
					},
				},
				{
					ID:        "verify",
					Name:      "Verify Backup",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"upload"},
					Config: workflow.StepConfig{
						Image:   "alpine:latest",
						Command: []string{"sh", "/scripts/verify_backup.sh"},
					},
				},
				{
					ID:        "cleanup",
					Name:      "Cleanup Old Backups",
					Type:      workflow.StepTypeSequential,
					DependsOn: []string{"verify"},
					Config: workflow.StepConfig{
						Image:   "amazon/aws-cli:2.4.0",
						Command: []string{"sh", "/scripts/cleanup_old_backups.sh"},
						EnvVars: map[string]string{
							"RETENTION_DAYS": "30",
						},
					},
				},
			},
			Tags: []string{"backup", "database", "scheduled", "s3"},
		},
	}
}