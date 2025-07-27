package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

// BuildProgress represents the progress of a Docker build
type BuildProgress struct {
	Status string
	Stream string
	Error  string
}

// ImageBuilder handles Docker image building operations
type ImageBuilder struct {
	client *client.Client
}

// NewImageBuilder creates a new image builder
func NewImageBuilder(dockerClient *client.Client) *ImageBuilder {
	return &ImageBuilder{
		client: dockerClient,
	}
}

// IsDockerfile checks if the given path is a Dockerfile
func IsDockerfile(path string) bool {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	
	// Must be a file, not a directory
	if info.IsDir() {
		return false
	}
	
	// Check if filename suggests it's a Dockerfile
	filename := filepath.Base(path)
	if strings.HasPrefix(strings.ToLower(filename), "dockerfile") {
		return true
	}
	
	// Check file content for Dockerfile commands
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Check for common Dockerfile commands
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "FROM ") ||
			strings.HasPrefix(upperLine, "RUN ") ||
			strings.HasPrefix(upperLine, "CMD ") ||
			strings.HasPrefix(upperLine, "EXPOSE ") ||
			strings.HasPrefix(upperLine, "ENV ") {
			return true
		}
		// Only check first few non-comment lines
		break
	}
	
	return false
}

// GenerateImageName generates a unique image name from the agent name
func GenerateImageName(agentName string) string {
	// Convert to lowercase and replace spaces/special chars
	imageName := strings.ToLower(agentName)
	imageName = strings.ReplaceAll(imageName, " ", "-")
	imageName = strings.ReplaceAll(imageName, "_", "-")
	
	// Add timestamp for uniqueness
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("agentainer-%s:%s", imageName, timestamp)
}

// BuildImage builds a Docker image from a Dockerfile
func (b *ImageBuilder) BuildImage(ctx context.Context, dockerfilePath, imageName string, progressChan chan<- string) error {
	defer close(progressChan)
	
	// Get the directory containing the Dockerfile
	contextDir := filepath.Dir(dockerfilePath)
	dockerfileName := filepath.Base(dockerfilePath)
	
	// Create tar archive of the build context
	progressChan <- "Preparing build context..."
	buildContext, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: []string{".git", "node_modules", "__pycache__"},
	})
	if err != nil {
		return fmt.Errorf("failed to create build context: %w", err)
	}
	defer buildContext.Close()
	
	// Prepare build options
	buildOptions := types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: dockerfileName,
		Remove:     true,
		PullParent: true,
	}
	
	progressChan <- fmt.Sprintf("Building image '%s' from %s...", imageName, dockerfilePath)
	
	// Start the build
	response, err := b.client.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer response.Body.Close()
	
	// Stream build output
	decoder := json.NewDecoder(response.Body)
	for {
		var message map[string]interface{}
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading build output: %w", err)
		}
		
		// Extract and send progress messages
		if stream, ok := message["stream"].(string); ok {
			stream = strings.TrimSpace(stream)
			if stream != "" {
				progressChan <- stream
			}
		}
		
		// Check for errors
		if errorDetail, ok := message["errorDetail"].(map[string]interface{}); ok {
			if errorMsg, ok := errorDetail["message"].(string); ok {
				return fmt.Errorf("build error: %s", errorMsg)
			}
		}
	}
	
	progressChan <- fmt.Sprintf("Successfully built image: %s", imageName)
	return nil
}

// CheckImageExists checks if a Docker image exists locally
func (b *ImageBuilder) CheckImageExists(ctx context.Context, imageName string) bool {
	_, _, err := b.client.ImageInspectWithRaw(ctx, imageName)
	return err == nil
}

// PreventDuplicateImage ensures the image name is unique
func (b *ImageBuilder) PreventDuplicateImage(ctx context.Context, imageName string) (string, error) {
	// If image doesn't exist, use as-is
	if !b.CheckImageExists(ctx, imageName) {
		return imageName, nil
	}
	
	// Generate alternative names
	baseName := imageName
	if idx := strings.LastIndex(imageName, ":"); idx > 0 {
		baseName = imageName[:idx]
	}
	
	// Try up to 10 variations
	for i := 1; i <= 10; i++ {
		timestamp := time.Now().Format("20060102-150405")
		newName := fmt.Sprintf("%s:%s-v%d", baseName, timestamp, i)
		if !b.CheckImageExists(ctx, newName) {
			return newName, nil
		}
	}
	
	return "", fmt.Errorf("could not generate unique image name after 10 attempts")
}