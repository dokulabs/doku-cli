package service

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/pkg/types"
)

// Manager handles service instance management
type Manager struct {
	dockerClient *docker.Client
	configMgr    *config.Manager
}

// NewManager creates a new service manager
func NewManager(dockerClient *docker.Client, configMgr *config.Manager) *Manager {
	return &Manager{
		dockerClient: dockerClient,
		configMgr:    configMgr,
	}
}

// List returns all service instances
func (m *Manager) List() ([]*types.Instance, error) {
	return m.configMgr.ListInstances()
}

// Get retrieves a specific instance
func (m *Manager) Get(instanceName string) (*types.Instance, error) {
	return m.configMgr.GetInstance(instanceName)
}

// Start starts a stopped service instance
func (m *Manager) Start(instanceName string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Check if already running
	if instance.Status == types.StatusRunning {
		return fmt.Errorf("instance '%s' is already running", instanceName)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.startMultiContainerService(instance)
	}

	// Start single container
	if err := m.dockerClient.ContainerStart(instance.ContainerName); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Update status
	instance.Status = types.StatusRunning
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instanceName, instance)
}

// Stop stops a running service instance
func (m *Manager) Stop(instanceName string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Check if already stopped
	if instance.Status == types.StatusStopped {
		return fmt.Errorf("instance '%s' is already stopped", instanceName)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.stopMultiContainerService(instance)
	}

	// Stop single container
	timeout := 10
	if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Update status
	instance.Status = types.StatusStopped
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instanceName, instance)
}

// Restart restarts a service instance
func (m *Manager) Restart(instanceName string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.restartMultiContainerService(instance)
	}

	// Restart single container
	timeout := 10
	if err := m.dockerClient.ContainerRestart(instance.ContainerName, &timeout); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}

	// Update timestamp
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instanceName, instance)
}

// Remove removes a service instance (stops and deletes)
func (m *Manager) Remove(instanceName string, force bool) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.removeMultiContainerService(instance, force)
	}

	// Stop container first if running and not forcing
	if instance.Status == types.StatusRunning && !force {
		timeout := 10
		if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	// Disconnect from network
	networkMgr := docker.NewNetworkManager(m.dockerClient)
	if err := networkMgr.DisconnectContainer("doku-network", instance.ContainerName, force); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to disconnect from network: %v\n", err)
	}

	// Remove container
	if err := m.dockerClient.ContainerRemove(instance.ContainerName, force); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Remove associated volumes
	if err := m.removeVolumes(instance); err != nil {
		fmt.Printf("Warning: failed to remove some volumes: %v\n", err)
	}

	// Remove from config
	return m.configMgr.RemoveInstance(instanceName)
}

// GetLogs retrieves logs from a service instance
func (m *Manager) GetLogs(instanceName string, follow bool) (string, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return "", fmt.Errorf("instance not found: %w", err)
	}

	// Get logs as ReadCloser
	logsReader, err := m.dockerClient.ContainerLogs(instance.ContainerName, follow)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer logsReader.Close()

	// Read all logs into string
	buf := make([]byte, 4096)
	var logs string
	for {
		n, err := logsReader.Read(buf)
		if n > 0 {
			logs += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return logs, nil
}

// GetStatus retrieves the current status of an instance
func (m *Manager) GetStatus(instanceName string) (types.ServiceStatus, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return types.StatusUnknown, fmt.Errorf("instance not found: %w", err)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.getMultiContainerStatus(instance)
	}

	// Check actual container status
	info, err := m.dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		return types.StatusUnknown, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Update status if different
	var status types.ServiceStatus
	if info.State.Running {
		status = types.StatusRunning
	} else if info.State.Dead || info.State.OOMKilled {
		status = types.StatusFailed
	} else {
		status = types.StatusStopped
	}

	// Update config if status changed
	if status != instance.Status {
		instance.Status = status
		instance.UpdatedAt = time.Now()
		m.configMgr.UpdateInstance(instanceName, instance)
	}

	return status, nil
}

// GetStats retrieves resource usage statistics
func (m *Manager) GetStats(instanceName string) (container.StatsResponseReader, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return container.StatsResponseReader{}, fmt.Errorf("instance not found: %w", err)
	}

	stats, err := m.dockerClient.ContainerStats(instance.ContainerName)
	if err != nil {
		return container.StatsResponseReader{}, fmt.Errorf("failed to get container stats: %w", err)
	}

	return stats, nil
}

// RefreshStatus updates the status of all instances
func (m *Manager) RefreshStatus() error {
	instances, err := m.configMgr.ListInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		_, err := m.GetStatus(instance.Name)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to refresh status for %s: %v\n", instance.Name, err)
		}
	}

	return nil
}

// removeVolumes removes volumes associated with an instance
func (m *Manager) removeVolumes(instance *types.Instance) error {
	// Get volumes for this instance
	containerInfo, err := m.dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		return err
	}

	// Remove named volumes
	for _, mount := range containerInfo.Mounts {
		if mount.Type == "volume" {
			// Only remove volumes managed by doku (starting with "doku-")
			if len(mount.Name) > 5 && mount.Name[:5] == "doku-" {
				if err := m.dockerClient.VolumeRemove(mount.Name, false); err != nil {
					fmt.Printf("Warning: failed to remove volume %s: %v\n", mount.Name, err)
				}
			}
		}
	}

	return nil
}

// GetConnectionInfo returns connection information for a service
func (m *Manager) GetConnectionInfo(instanceName string) (*types.ConnectionInfo, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	info := &types.ConnectionInfo{
		Host:     instance.Name,
		Port:     instance.Network.InternalPort,
		URL:      instance.URL,
		Protocol: instance.Traefik.Protocol,
		Env:      make(map[string]string),
	}

	// Add common environment variables for connection
	if instance.Traefik.Protocol == "http" || instance.Traefik.Protocol == "https" {
		info.Env["SERVICE_URL"] = instance.URL
	} else {
		info.Env["SERVICE_HOST"] = instance.Name
		info.Env["SERVICE_PORT"] = fmt.Sprintf("%d", instance.Network.InternalPort)
	}

	// Add service-specific connection env vars
	for k, v := range instance.Environment {
		// Include common connection-related env vars
		lowerKey := string([]rune(k))
		if containsAny(lowerKey, []string{"user", "username", "password", "database", "db"}) {
			info.Env[k] = v
		}
	}

	return info, nil
}

// containsAny checks if a string contains any of the substrings
func containsAny(s string, substrs []string) bool {
	s = toLower(s)
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase
func toLower(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			runes[i] = r + 32
		}
	}
	return string(runes)
}

// contains checks if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

// indexOfSubstring returns the index of substring in string
func indexOfSubstring(s, substr string) int {
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Multi-Container Service Methods

// startMultiContainerService starts all containers in a multi-container service
func (m *Manager) startMultiContainerService(instance *types.Instance) error {
	for i := range instance.Containers {
		container := &instance.Containers[i]

		if err := m.dockerClient.ContainerStart(container.ContainerID); err != nil {
			return fmt.Errorf("failed to start container %s: %w", container.Name, err)
		}

		container.Status = "running"
		fmt.Printf("Started container: %s\n", container.Name)

		// Brief pause between containers
		time.Sleep(time.Second)
	}

	// Update overall instance status
	instance.Status = types.StatusRunning
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instance.Name, instance)
}

// stopMultiContainerService stops all containers in a multi-container service
func (m *Manager) stopMultiContainerService(instance *types.Instance) error {
	// Stop containers in reverse order
	for i := len(instance.Containers) - 1; i >= 0; i-- {
		container := &instance.Containers[i]

		timeout := 10
		if err := m.dockerClient.ContainerStop(container.ContainerID, &timeout); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", container.Name, err)
		}

		container.Status = "stopped"
		fmt.Printf("Stopped container: %s\n", container.Name)
	}

	// Update overall instance status
	instance.Status = types.StatusStopped
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instance.Name, instance)
}

// restartMultiContainerService restarts all containers in a multi-container service
func (m *Manager) restartMultiContainerService(instance *types.Instance) error {
	for i := range instance.Containers {
		container := &instance.Containers[i]

		timeout := 10
		if err := m.dockerClient.ContainerRestart(container.ContainerID, &timeout); err != nil {
			return fmt.Errorf("failed to restart container %s: %w", container.Name, err)
		}

		fmt.Printf("Restarted container: %s\n", container.Name)

		// Brief pause between containers
		time.Sleep(time.Second)
	}

	// Update timestamp
	instance.UpdatedAt = time.Now()

	return m.configMgr.UpdateInstance(instance.Name, instance)
}

// removeMultiContainerService removes all containers in a multi-container service
func (m *Manager) removeMultiContainerService(instance *types.Instance, force bool) error {
	networkMgr := docker.NewNetworkManager(m.dockerClient)

	// Remove containers in reverse order
	for i := len(instance.Containers) - 1; i >= 0; i-- {
		container := &instance.Containers[i]

		// Stop container if running and not forcing
		if container.Status == "running" && !force {
			timeout := 10
			if err := m.dockerClient.ContainerStop(container.ContainerID, &timeout); err != nil {
				fmt.Printf("Warning: failed to stop container %s: %v\n", container.Name, err)
			}
		}

		// Disconnect from network
		if err := networkMgr.DisconnectContainer("doku-network", container.FullName, force); err != nil {
			fmt.Printf("Warning: failed to disconnect %s from network: %v\n", container.Name, err)
		}

		// Remove container
		if err := m.dockerClient.ContainerRemove(container.ContainerID, force); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", container.Name, err)
		}

		fmt.Printf("Removed container: %s\n", container.Name)
	}

	// Remove associated volumes
	if err := m.removeMultiContainerVolumes(instance); err != nil {
		fmt.Printf("Warning: failed to remove some volumes: %v\n", err)
	}

	// Remove from config
	return m.configMgr.RemoveInstance(instance.Name)
}

// getMultiContainerStatus checks the status of all containers in a multi-container service
func (m *Manager) getMultiContainerStatus(instance *types.Instance) (types.ServiceStatus, error) {
	runningCount := 0
	stoppedCount := 0
	failedCount := 0

	for i := range instance.Containers {
		container := &instance.Containers[i]

		info, err := m.dockerClient.ContainerInspect(container.ContainerID)
		if err != nil {
			container.Status = "unknown"
			continue
		}

		if info.State.Running {
			container.Status = "running"
			runningCount++
		} else if info.State.Dead || info.State.OOMKilled {
			container.Status = "failed"
			failedCount++
		} else {
			container.Status = "stopped"
			stoppedCount++
		}
	}

	// Determine overall status
	var status types.ServiceStatus
	if failedCount > 0 {
		status = types.StatusFailed
	} else if runningCount == len(instance.Containers) {
		status = types.StatusRunning
	} else if stoppedCount == len(instance.Containers) {
		status = types.StatusStopped
	} else {
		// Partially running
		status = types.StatusRunning
	}

	// Update config if status changed
	if status != instance.Status {
		instance.Status = status
		instance.UpdatedAt = time.Now()
		m.configMgr.UpdateInstance(instance.Name, instance)
	}

	return status, nil
}

// removeMultiContainerVolumes removes volumes for a multi-container service
func (m *Manager) removeMultiContainerVolumes(instance *types.Instance) error {
	for _, container := range instance.Containers {
		// Try to inspect container to get volume info
		containerInfo, err := m.dockerClient.ContainerInspect(container.ContainerID)
		if err != nil {
			continue
		}

		// Remove named volumes
		for _, mount := range containerInfo.Mounts {
			if mount.Type == "volume" {
				// Only remove volumes managed by doku
				if len(mount.Name) > 5 && mount.Name[:5] == "doku-" {
					if err := m.dockerClient.VolumeRemove(mount.Name, false); err != nil {
						fmt.Printf("Warning: failed to remove volume %s: %v\n", mount.Name, err)
					}
				}
			}
		}
	}

	return nil
}

// GetContainerLogs retrieves logs from a specific container in a multi-container service
func (m *Manager) GetContainerLogs(instanceName, containerName string, follow bool) (string, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return "", fmt.Errorf("instance not found: %w", err)
	}

	if !instance.IsMultiContainer {
		return "", fmt.Errorf("instance '%s' is not a multi-container service", instanceName)
	}

	// Find the container
	var targetContainer *types.ContainerInfo
	for i := range instance.Containers {
		if instance.Containers[i].Name == containerName {
			targetContainer = &instance.Containers[i]
			break
		}
	}

	if targetContainer == nil {
		return "", fmt.Errorf("container '%s' not found in service '%s'", containerName, instanceName)
	}

	// Get logs
	logsReader, err := m.dockerClient.ContainerLogs(targetContainer.ContainerID, follow)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer logsReader.Close()

	// Read all logs into string
	buf := make([]byte, 4096)
	var logs string
	for {
		n, err := logsReader.Read(buf)
		if n > 0 {
			logs += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return logs, nil
}
