package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client
type Client struct {
	cli *client.Client
	ctx context.Context
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{
		cli: cli,
		ctx: context.Background(),
	}, nil
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

// Ping checks if Docker daemon is reachable
func (c *Client) Ping() error {
	_, err := c.cli.Ping(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to ping Docker daemon: %w", err)
	}
	return nil
}

// Version returns Docker daemon version information
func (c *Client) Version() (types.Version, error) {
	version, err := c.cli.ServerVersion(c.ctx)
	if err != nil {
		return types.Version{}, fmt.Errorf("failed to get Docker version: %w", err)
	}
	return version, nil
}

// IsDockerAvailable checks if Docker is available and running
func (c *Client) IsDockerAvailable() bool {
	return c.Ping() == nil
}

// Container Operations

// ContainerCreate creates a new container
func (c *Client) ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (string, error) {
	resp, err := c.cli.ContainerCreate(c.ctx, config, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// ContainerStart starts a container
func (c *Client) ContainerStart(containerID string) error {
	if err := c.cli.ContainerStart(c.ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// ContainerStop stops a container
func (c *Client) ContainerStop(containerID string, timeout *int) error {
	options := container.StopOptions{}
	if timeout != nil {
		options.Timeout = timeout
	}

	if err := c.cli.ContainerStop(c.ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

// ContainerRemove removes a container
func (c *Client) ContainerRemove(containerID string, force bool) error {
	options := container.RemoveOptions{
		Force: force,
	}

	if err := c.cli.ContainerRemove(c.ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

// ContainerRestart restarts a container
func (c *Client) ContainerRestart(containerID string, timeout *int) error {
	options := container.StopOptions{}
	if timeout != nil {
		options.Timeout = timeout
	}

	if err := c.cli.ContainerRestart(c.ctx, containerID, options); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return nil
}

// ContainerInspect returns detailed information about a container
func (c *Client) ContainerInspect(containerID string) (types.ContainerJSON, error) {
	info, err := c.cli.ContainerInspect(c.ctx, containerID)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("failed to inspect container: %w", err)
	}
	return info, nil
}

// ContainerList lists all containers
func (c *Client) ContainerList(all bool) ([]types.Container, error) {
	options := container.ListOptions{
		All: all,
	}

	containers, err := c.cli.ContainerList(c.ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

// ContainerLogs retrieves logs from a container
func (c *Client) ContainerLogs(containerID string, follow bool) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	}

	logs, err := c.cli.ContainerLogs(c.ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	return logs, nil
}

// ContainerStats returns resource usage statistics for a container
func (c *Client) ContainerStats(containerID string) (types.ContainerStats, error) {
	stats, err := c.cli.ContainerStats(c.ctx, containerID, false)
	if err != nil {
		return types.ContainerStats{}, fmt.Errorf("failed to get container stats: %w", err)
	}
	return stats, nil
}

// ContainerExists checks if a container exists
func (c *Client) ContainerExists(containerName string) (bool, error) {
	containers, err := c.ContainerList(true)
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			// Docker prefixes names with "/"
			if name == "/"+containerName || name == containerName {
				return true, nil
			}
		}
	}
	return false, nil
}

// Image Operations

// ImagePull pulls an image from a registry
func (c *Client) ImagePull(imageName string) error {
	out, err := c.cli.ImagePull(c.ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer out.Close()

	// Copy output to stdout to show pull progress
	_, err = io.Copy(os.Stdout, out)
	return err
}

// ImageList lists available images
func (c *Client) ImageList() ([]image.Summary, error) {
	images, err := c.cli.ImageList(c.ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

// ImageRemove removes an image
func (c *Client) ImageRemove(imageID string, force bool) error {
	options := types.ImageRemoveOptions{
		Force: force,
	}

	_, err := c.cli.ImageRemove(c.ctx, imageID, options)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}
	return nil
}

// ImageExists checks if an image exists locally
func (c *Client) ImageExists(imageName string) (bool, error) {
	images, err := c.ImageList()
	if err != nil {
		return false, err
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				return true, nil
			}
		}
	}
	return false, nil
}

// Volume Operations

// VolumeCreate creates a new volume
func (c *Client) VolumeCreate(volumeName string, labels map[string]string) (*volume.Volume, error) {
	vol, err := c.cli.VolumeCreate(c.ctx, volume.CreateOptions{
		Name:   volumeName,
		Labels: labels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}
	return &vol, nil
}

// VolumeRemove removes a volume
func (c *Client) VolumeRemove(volumeName string, force bool) error {
	if err := c.cli.VolumeRemove(c.ctx, volumeName, force); err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}
	return nil
}

// VolumeList lists all volumes
func (c *Client) VolumeList() ([]*volume.Volume, error) {
	volumeList, err := c.cli.VolumeList(c.ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	volumes := make([]*volume.Volume, len(volumeList.Volumes))
	for i, vol := range volumeList.Volumes {
		volumes[i] = vol
	}
	return volumes, nil
}

// VolumeInspect returns detailed information about a volume
func (c *Client) VolumeInspect(volumeName string) (*volume.Volume, error) {
	vol, err := c.cli.VolumeInspect(c.ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect volume: %w", err)
	}
	return &vol, nil
}

// VolumeExists checks if a volume exists
func (c *Client) VolumeExists(volumeName string) (bool, error) {
	_, err := c.VolumeInspect(volumeName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Network Operations

// NetworkCreate creates a new network
func (c *Client) NetworkCreate(networkName string, options types.NetworkCreate) (string, error) {
	resp, err := c.cli.NetworkCreate(c.ctx, networkName, options)
	if err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}
	return resp.ID, nil
}

// NetworkRemove removes a network
func (c *Client) NetworkRemove(networkID string) error {
	if err := c.cli.NetworkRemove(c.ctx, networkID); err != nil {
		return fmt.Errorf("failed to remove network: %w", err)
	}
	return nil
}

// NetworkList lists all networks
func (c *Client) NetworkList() ([]types.NetworkResource, error) {
	networks, err := c.cli.NetworkList(c.ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	return networks, nil
}

// NetworkInspect returns detailed information about a network
func (c *Client) NetworkInspect(networkID string) (types.NetworkResource, error) {
	network, err := c.cli.NetworkInspect(c.ctx, networkID, types.NetworkInspectOptions{})
	if err != nil {
		return types.NetworkResource{}, fmt.Errorf("failed to inspect network: %w", err)
	}
	return network, nil
}

// NetworkExists checks if a network exists
func (c *Client) NetworkExists(networkName string) (bool, error) {
	networks, err := c.NetworkList()
	if err != nil {
		return false, err
	}

	for _, net := range networks {
		if net.Name == networkName {
			return true, nil
		}
	}
	return false, nil
}

// NetworkConnect connects a container to a network
func (c *Client) NetworkConnect(networkID, containerID string) error {
	if err := c.cli.NetworkConnect(c.ctx, networkID, containerID, nil); err != nil {
		return fmt.Errorf("failed to connect container to network: %w", err)
	}
	return nil
}

// NetworkDisconnect disconnects a container from a network
func (c *Client) NetworkDisconnect(networkID, containerID string, force bool) error {
	if err := c.cli.NetworkDisconnect(c.ctx, networkID, containerID, force); err != nil {
		return fmt.Errorf("failed to disconnect container from network: %w", err)
	}
	return nil
}
