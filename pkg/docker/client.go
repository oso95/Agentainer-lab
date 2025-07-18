package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

func NewClient(host string) (*client.Client, error) {
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	if _, err := cli.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping docker daemon: %w", err)
	}

	return cli, nil
}