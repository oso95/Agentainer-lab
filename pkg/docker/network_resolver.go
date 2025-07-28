package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
)

// NetworkResolver helps resolve container hostnames and IPs
type NetworkResolver struct {
	client *client.Client
}

// NewNetworkResolver creates a new network resolver
func NewNetworkResolver(dockerClient *client.Client) *NetworkResolver {
	return &NetworkResolver{
		client: dockerClient,
	}
}

// GetContainerEndpoint returns the endpoint URL for a container
// It tries to get the container IP on the specified network
func (r *NetworkResolver) GetContainerEndpoint(ctx context.Context, containerID string, networkName string, port int) (string, error) {
	// Inspect the container to get its network settings
	containerInfo, err := r.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Try to get IP from the specified network
	if networkSettings, ok := containerInfo.NetworkSettings.Networks[networkName]; ok {
		if networkSettings.IPAddress != "" {
			return fmt.Sprintf("http://%s:%d", networkSettings.IPAddress, port), nil
		}
	}

	// Fallback: try to use container name/ID
	// This will only work if the caller is on the same network
	return fmt.Sprintf("http://%s:%d", containerInfo.Config.Hostname, port), nil
}

// GetContainerIP returns just the IP address of a container on a network
func (r *NetworkResolver) GetContainerIP(ctx context.Context, containerID string, networkName string) (string, error) {
	containerInfo, err := r.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	if networkSettings, ok := containerInfo.NetworkSettings.Networks[networkName]; ok {
		if networkSettings.IPAddress != "" {
			return networkSettings.IPAddress, nil
		}
	}

	return "", fmt.Errorf("container not found on network %s", networkName)
}