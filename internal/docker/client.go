package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	networkTypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client
type Client struct {
	cli *client.Client
	ctx context.Context
}

// NewClient creates a new Docker client with BuildKit enabled
func NewClient() (*Client, error) {
	// Set DOCKER_BUILDKIT environment variable to enable BuildKit
	// This must be done before any Docker operations
	os.Setenv("DOCKER_BUILDKIT", "1")

	// Create client with BuildKit support
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
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
func (c *Client) ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *networkTypes.NetworkingConfig, containerName string) (string, error) {
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
func (c *Client) ContainerStats(ctx context.Context, containerID string) (*ContainerStatsResult, error) {
	stats, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer stats.Body.Close()

	// Parse stats
	var statsJSON container.StatsResponse
	decoder := json.NewDecoder(stats.Body)
	if err := decoder.Decode(&statsJSON); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	result := &ContainerStatsResult{
		MemoryUsage: statsJSON.MemoryStats.Usage,
		MemoryLimit: statsJSON.MemoryStats.Limit,
	}

	// Calculate CPU percentage
	cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(statsJSON.CPUStats.SystemUsage - statsJSON.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		result.CPUPercent = (cpuDelta / systemDelta) * float64(statsJSON.CPUStats.OnlineCPUs) * 100.0
	}

	return result, nil
}

// ContainerStatsResult contains parsed container statistics
type ContainerStatsResult struct {
	CPUPercent  float64
	MemoryUsage uint64
	MemoryLimit uint64
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
	out, err := c.cli.ImagePull(c.ctx, imageName, image.PullOptions{})
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
	images, err := c.cli.ImageList(c.ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

// ImageRemove removes an image
func (c *Client) ImageRemove(imageID string, force bool) error {
	options := image.RemoveOptions{
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

// ImageBuild builds a Docker image
func (c *Client) ImageBuild(buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	response, err := c.cli.ImageBuild(c.ctx, buildContext, options)
	if err != nil {
		return types.ImageBuildResponse{}, fmt.Errorf("failed to build image: %w", err)
	}
	return response, nil
}

// ImageTag tags an image
func (c *Client) ImageTag(source, target string) error {
	if err := c.cli.ImageTag(c.ctx, source, target); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}
	return nil
}

// ImageInspectWithRaw returns detailed information about an image
func (c *Client) ImageInspectWithRaw(imageID string) (types.ImageInspect, []byte, error) {
	inspect, raw, err := c.cli.ImageInspectWithRaw(c.ctx, imageID)
	if err != nil {
		return types.ImageInspect{}, nil, fmt.Errorf("failed to inspect image: %w", err)
	}
	return inspect, raw, nil
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
func (c *Client) NetworkCreate(networkName string, options network.CreateOptions) (string, error) {
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
func (c *Client) NetworkList() ([]network.Inspect, error) {
	networks, err := c.cli.NetworkList(c.ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	return networks, nil
}

// NetworkInspect returns detailed information about a network
func (c *Client) NetworkInspect(networkID string) (network.Inspect, error) {
	net, err := c.cli.NetworkInspect(c.ctx, networkID, network.InspectOptions{})
	if err != nil {
		return network.Inspect{}, fmt.Errorf("failed to inspect network: %w", err)
	}
	return net, nil
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

// NetworkConnectWithAliases connects a container to a network with custom aliases
func (c *Client) NetworkConnectWithAliases(networkID, containerID string, aliases []string) error {
	endpointSettings := &networkTypes.EndpointSettings{
		Aliases: aliases,
	}

	if err := c.cli.NetworkConnect(c.ctx, networkID, containerID, endpointSettings); err != nil {
		return fmt.Errorf("failed to connect container to network with aliases: %w", err)
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

// Helper methods for filtering by labels

// ListContainers lists all containers
func (c *Client) ListContainers(ctx context.Context) ([]types.Container, error) {
	options := container.ListOptions{
		All: true,
	}

	containers, err := c.cli.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

// ListContainersByLabel lists containers with a specific label
func (c *Client) ListContainersByLabel(ctx context.Context, labelKey, labelValue string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", labelKey, labelValue))

	options := container.ListOptions{
		All:     true,
		Filters: filterArgs,
	}

	containers, err := c.cli.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers by label: %w", err)
	}
	return containers, nil
}

// ListVolumes lists all volumes
func (c *Client) ListVolumes(ctx context.Context) ([]*volume.Volume, error) {
	options := volume.ListOptions{}

	volumeList, err := c.cli.VolumeList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	volumes := make([]*volume.Volume, len(volumeList.Volumes))
	for i, vol := range volumeList.Volumes {
		volumes[i] = vol
	}
	return volumes, nil
}

// ListVolumesByLabel lists volumes with a specific label
func (c *Client) ListVolumesByLabel(ctx context.Context, labelKey, labelValue string) ([]*volume.Volume, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", labelKey, labelValue))

	options := volume.ListOptions{
		Filters: filterArgs,
	}

	volumeList, err := c.cli.VolumeList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes by label: %w", err)
	}

	volumes := make([]*volume.Volume, len(volumeList.Volumes))
	for i, vol := range volumeList.Volumes {
		volumes[i] = vol
	}
	return volumes, nil
}

// ListVolumesByPrefix lists volumes whose names start with the given prefix
func (c *Client) ListVolumesByPrefix(ctx context.Context, prefix string) ([]*volume.Volume, error) {
	allVolumes, err := c.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	var matchedVolumes []*volume.Volume
	for _, vol := range allVolumes {
		if strings.HasPrefix(vol.Name, prefix) {
			matchedVolumes = append(matchedVolumes, vol)
		}
	}
	return matchedVolumes, nil
}

// StopContainer stops a container by name or ID
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10 // 10 seconds timeout
	return c.ContainerStop(containerID, &timeout)
}

// RemoveContainer removes a container by name or ID
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return c.ContainerRemove(containerID, true)
}

// RemoveVolume removes a volume by name
func (c *Client) RemoveVolume(ctx context.Context, volumeName string) error {
	return c.VolumeRemove(volumeName, true)
}

// RemoveNetwork removes a network by name or ID
func (c *Client) RemoveNetwork(ctx context.Context, networkID string) error {
	return c.NetworkRemove(networkID)
}

// RunContainer creates and starts a container in one step (for init containers)
func (c *Client) RunContainer(image, name string, cmd, env []string, network string, autoRemove bool) (string, error) {
	ctx := context.Background()

	// Create container config
	config := &container.Config{
		Image: image,
		Cmd:   cmd,
		Env:   env,
	}

	// Create host config
	hostConfig := &container.HostConfig{
		AutoRemove: autoRemove,
	}

	// Create network config
	networkConfig := &networkTypes.NetworkingConfig{}
	if network != "" {
		networkConfig.EndpointsConfig = map[string]*networkTypes.EndpointSettings{
			network: {},
		}
	}

	// Create container
	resp, err := c.cli.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// WaitForContainer waits for a container to complete
func (c *Client) WaitForContainer(containerID string) error {
	ctx := context.Background()

	// Wait for container to finish
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status code %d", status.StatusCode)
		}
	}

	return nil
}

// GetContainerLogsString gets logs from a container as a string
func (c *Client) GetContainerLogsString(containerID string) (string, error) {
	logs, err := c.ContainerLogs(containerID, false)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	data, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(data), nil
}

// ExecOptions holds options for executing a command in a container
type ExecOptions struct {
	Container   string
	Command     []string
	Interactive bool
	TTY         bool
	User        string
	WorkDir     string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// Exec executes a command inside a running container
func (c *Client) Exec(ctx context.Context, opts ExecOptions) error {
	// Create exec configuration
	execConfig := container.ExecOptions{
		AttachStdin:  opts.Interactive,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          opts.TTY,
		Cmd:          opts.Command,
		User:         opts.User,
		WorkingDir:   opts.WorkDir,
	}

	// Create exec instance
	execID, err := c.cli.ContainerExecCreate(ctx, opts.Container, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec instance
	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{
		Tty: opts.TTY,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Handle I/O
	errCh := make(chan error, 1)

	// Copy output
	go func() {
		_, err := io.Copy(opts.Stdout, resp.Reader)
		errCh <- err
	}()

	// Copy input if interactive
	if opts.Interactive && opts.Stdin != nil {
		go func() {
			io.Copy(resp.Conn, opts.Stdin)
		}()
	}

	// Wait for output to complete
	if err := <-errCh; err != nil {
		return fmt.Errorf("error during exec: %w", err)
	}

	// Check exit code
	inspectResp, err := c.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", inspectResp.ExitCode)
	}

	return nil
}
