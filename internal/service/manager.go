package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
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

// List returns all service instances (including custom projects)
func (m *Manager) List() ([]*types.Instance, error) {
	// Get catalog-based instances
	instances, err := m.configMgr.ListInstances()
	if err != nil {
		return nil, err
	}

	// Get custom projects and convert to instances
	cfg, err := m.configMgr.Get()
	if err != nil {
		return nil, err
	}

	// Convert projects to instances for display
	for _, project := range cfg.Projects {
		instance := &types.Instance{
			Name:          project.Name,
			ServiceType:   "custom-project",
			Version:       "",
			ContainerName: project.ContainerName,
			ContainerID:   project.ContainerID,
			Status:        project.Status,
			URL:           project.URL,
			CreatedAt:     project.CreatedAt,
			Environment:   project.Environment,
			Network: types.NetworkConfig{
				InternalPort: project.Port,
			},
			Traefik: types.TraefikInstanceConfig{
				Enabled: project.URL != "",
			},
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// Get retrieves a specific instance (checks both catalog services and custom projects)
func (m *Manager) Get(instanceName string) (*types.Instance, error) {
	// First check catalog services (Instances)
	instance, err := m.configMgr.GetInstance(instanceName)
	if err == nil {
		return instance, nil
	}

	// If not found in Instances, check custom projects
	cfg, err := m.configMgr.Get()
	if err != nil {
		return nil, err
	}

	project, exists := cfg.Projects[instanceName]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found", instanceName)
	}

	// Convert Project to Instance for consistent handling
	instance = &types.Instance{
		Name:          project.Name,
		ServiceType:   "custom-project",
		Version:       "",
		ContainerName: project.ContainerName,
		ContainerID:   project.ContainerID,
		Status:        project.Status,
		URL:           project.URL,
		CreatedAt:     project.CreatedAt,
		Environment:   project.Environment,
		Network: types.NetworkConfig{
			InternalPort: project.Port,
		},
		Traefik: types.TraefikInstanceConfig{
			Enabled: project.URL != "",
		},
	}

	return instance, nil
}

// Start starts a stopped service instance
func (m *Manager) Start(instanceName string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Check if already running
	if instance.Status == types.StatusRunning {
		return fmt.Errorf("%w: %s", types.ErrAlreadyRunning, instanceName)
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
		return fmt.Errorf("%w: %s", types.ErrAlreadyStopped, instanceName)
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
	return m.RestartWithInit(instanceName, false, nil)
}

// RestartWithInit restarts a service instance with optional init container execution
func (m *Manager) RestartWithInit(instanceName string, runInit bool, catalogMgr *catalog.Manager) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.restartMultiContainerServiceWithInit(instance, runInit, catalogMgr)
	}

	// Single container services don't support init containers
	if runInit {
		color.Yellow("⚠️  --run-init is only supported for multi-container services")
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

// Recreate recreates a service container to apply configuration changes (like environment variables)
// This stops, removes, and recreates the container with environment from the env file
func (m *Manager) Recreate(instanceName string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Multi-container services not supported yet
	if instance.IsMultiContainer {
		return fmt.Errorf("recreation not supported for multi-container services yet")
	}

	// Get container info to preserve configuration
	containerInfo, err := m.dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Load environment from env file (primary source)
	envMgr := envfile.NewManager(m.configMgr.GetDokuDir())
	envPath := envMgr.GetServiceEnvPath(instanceName, "")
	env, err := envMgr.Load(envPath)
	if err != nil {
		// Fall back to instance.Environment for backward compatibility
		env = instance.Environment
	}

	// Build environment array
	if len(env) > 0 {
		envArray := make([]string, 0, len(env))
		for key, value := range env {
			envArray = append(envArray, fmt.Sprintf("%s=%s", key, value))
		}
		containerInfo.Config.Env = envArray
	}

	// Stop the container
	timeout := 10
	if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Disconnect from network
	networkMgr := docker.NewNetworkManager(m.dockerClient)
	if err := networkMgr.DisconnectContainer("doku-network", instance.ContainerName, true); err != nil {
		fmt.Printf("Warning: failed to disconnect from network: %v\n", err)
	}

	// Remove the container (but preserve volumes)
	if err := m.dockerClient.ContainerRemove(instance.ContainerName, false); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Recreate the container with updated configuration
	if err := m.recreateContainer(instance, &containerInfo); err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	// Update config
	instance.UpdatedAt = time.Now()
	return m.configMgr.UpdateInstance(instanceName, instance)
}

// RecreateWithImage recreates a container with a new image (for upgrades)
func (m *Manager) RecreateWithImage(instanceName string, newImage string) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Multi-container services not supported yet
	if instance.IsMultiContainer {
		return fmt.Errorf("image upgrade not supported for multi-container services yet")
	}

	// Get container info to preserve configuration
	containerInfo, err := m.dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Update the image
	containerInfo.Config.Image = newImage

	// Load environment from env file (primary source)
	envMgr := envfile.NewManager(m.configMgr.GetDokuDir())
	envPath := envMgr.GetServiceEnvPath(instanceName, "")
	env, err := envMgr.Load(envPath)
	if err != nil {
		// Fall back to instance.Environment for backward compatibility
		env = instance.Environment
	}

	// Build environment array
	if len(env) > 0 {
		envArray := make([]string, 0, len(env))
		for key, value := range env {
			envArray = append(envArray, fmt.Sprintf("%s=%s", key, value))
		}
		containerInfo.Config.Env = envArray
	}

	// Stop the container if running
	timeout := 10
	if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
		// Ignore error if container is already stopped
		fmt.Printf("Note: Container may already be stopped: %v\n", err)
	}

	// Disconnect from network
	networkMgr := docker.NewNetworkManager(m.dockerClient)
	if err := networkMgr.DisconnectContainer("doku-network", instance.ContainerName, true); err != nil {
		fmt.Printf("Warning: failed to disconnect from network: %v\n", err)
	}

	// Remove the container (but preserve volumes)
	if err := m.dockerClient.ContainerRemove(instance.ContainerName, false); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Recreate the container with the new image
	if err := m.recreateContainer(instance, &containerInfo); err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	// Update config
	instance.UpdatedAt = time.Now()
	return m.configMgr.UpdateInstance(instanceName, instance)
}

// RestartWithPort restarts a service instance with a new host port mapping
// This requires recreating the container since port mappings cannot be changed on existing containers
func (m *Manager) RestartWithPort(instanceName string, newPort int) error {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Multi-container services not supported yet
	if instance.IsMultiContainer {
		return fmt.Errorf("port mapping changes not supported for multi-container services")
	}

	// Get container info to preserve configuration
	containerInfo, err := m.dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Stop the container
	timeout := 10
	if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Disconnect from network
	networkMgr := docker.NewNetworkManager(m.dockerClient)
	if err := networkMgr.DisconnectContainer("doku-network", instance.ContainerName, true); err != nil {
		fmt.Printf("Warning: failed to disconnect from network: %v\n", err)
	}

	// Remove the container (but preserve volumes)
	if err := m.dockerClient.ContainerRemove(instance.ContainerName, false); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Update instance configuration with new port
	instance.Network.HostPort = newPort

	// Recreate the container with new port configuration
	if err := m.recreateContainer(instance, &containerInfo); err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	// Update config
	instance.UpdatedAt = time.Now()
	return m.configMgr.UpdateInstance(instanceName, instance)
}

// Remove removes a service instance (stops and deletes)
func (m *Manager) Remove(instanceName string, force bool, removeVolumes bool) error {
	// Use Get() which checks both Instances and Projects
	instance, err := m.Get(instanceName)
	if err != nil {
		return err
	}

	// Check if it's a custom project
	isCustomProject := instance.ServiceType == "custom-project"

	// Handle multi-container services
	if instance.IsMultiContainer {
		return m.removeMultiContainerService(instance, force, removeVolumes)
	}

	// Check if container exists
	containerExists, err := m.dockerClient.ContainerExists(instance.ContainerName)
	if err != nil {
		fmt.Printf("Warning: failed to check container existence: %v\n", err)
		// Continue anyway - we'll try to clean up what we can
	}

	if !containerExists {
		// Container was already removed (manually or by error)
		fmt.Printf("⚠️  Container %s does not exist (may have been removed manually)\n", instance.ContainerName)
		if removeVolumes {
			fmt.Println("Cleaning up configuration and volumes...")
		} else {
			fmt.Println("Cleaning up configuration...")
		}
	} else {
		// Stop container first if running and not forcing
		if instance.Status == types.StatusRunning && !force {
			timeout := 10
			if err := m.dockerClient.ContainerStop(instance.ContainerName, &timeout); err != nil {
				fmt.Printf("Warning: failed to stop container: %v\n", err)
				// Continue with removal
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
			fmt.Printf("Warning: failed to remove container: %v\n", err)
			// Continue to clean up config even if container removal fails
		}

		// Remove associated volumes only if user agreed
		if removeVolumes {
			if err := m.removeVolumes(instance); err != nil {
				fmt.Printf("Warning: failed to remove some volumes: %v\n", err)
			}
		}
	}

	// Remove from config - always do this to clean up state
	if isCustomProject {
		return m.configMgr.RemoveProject(instanceName)
	}
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

	// Read all logs into string using strings.Builder for efficiency
	buf := make([]byte, 4096)
	var logs strings.Builder
	for {
		n, err := logsReader.Read(buf)
		if n > 0 {
			logs.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return logs.String(), nil
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
func (m *Manager) GetStats(instanceName string) (*docker.ContainerStatsResult, error) {
	instance, err := m.configMgr.GetInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	ctx := context.Background()
	stats, err := m.dockerClient.ContainerStats(ctx, instance.ContainerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
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

// containsAny checks if a string contains any of the substrings (case-insensitive)
func containsAny(s string, substrs []string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
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
	return m.restartMultiContainerServiceWithInit(instance, false, nil)
}

// restartMultiContainerServiceWithInit restarts all containers with optional init container execution
func (m *Manager) restartMultiContainerServiceWithInit(instance *types.Instance, runInit bool, catalogMgr *catalog.Manager) error {
	// Run init containers if requested
	if runInit && catalogMgr != nil {
		// Get service spec to find init containers
		spec, err := catalogMgr.GetServiceVersion(instance.ServiceType, instance.Version)
		if err != nil {
			return fmt.Errorf("failed to get service spec: %w", err)
		}

		if len(spec.InitContainers) > 0 {
			if err := m.runInitContainers(spec, instance.Name); err != nil {
				return fmt.Errorf("failed to run init containers: %w", err)
			}
		} else {
			color.Yellow("⚠️  No init containers defined for this service")
		}
	}

	// Restart all containers
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
func (m *Manager) removeMultiContainerService(instance *types.Instance, force bool, removeVolumes bool) error {
	networkMgr := docker.NewNetworkManager(m.dockerClient)

	// Remove containers in reverse order
	for i := len(instance.Containers) - 1; i >= 0; i-- {
		container := &instance.Containers[i]

		// Check if container exists
		containerExists, err := m.dockerClient.ContainerExists(container.ContainerID)
		if err != nil {
			fmt.Printf("Warning: failed to check if container %s exists: %v\n", container.Name, err)
		}

		if !containerExists {
			fmt.Printf("⚠️  Container %s does not exist (may have been removed manually)\n", container.Name)
			continue
		}

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
			fmt.Printf("Warning: failed to remove container %s: %v\n", container.Name, err)
			// Continue with other containers instead of returning error
		} else {
			fmt.Printf("Removed container: %s\n", container.Name)
		}
	}

	// Remove associated volumes only if user agreed
	if removeVolumes {
		if err := m.removeMultiContainerVolumes(instance); err != nil {
			fmt.Printf("Warning: failed to remove some volumes: %v\n", err)
		}
	}

	// Remove from config - always do this to clean up state
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

// recreateContainer recreates a container with new port configuration
func (m *Manager) recreateContainer(instance *types.Instance, oldContainerInfo *dockerTypes.ContainerJSON) error {
	// Import nat package for port handling
	var portBindings nat.PortMap
	var exposedPorts nat.PortSet

	// Create port bindings if host port is specified
	if instance.Network.HostPort > 0 {
		portBindings = nat.PortMap{}
		exposedPorts = nat.PortSet{}

		containerPortSpec := nat.Port(fmt.Sprintf("%d/tcp", instance.Network.InternalPort))

		exposedPorts[containerPortSpec] = struct{}{}
		portBindings[containerPortSpec] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: fmt.Sprintf("%d", instance.Network.HostPort),
			},
		}
	}

	// Create container config using preserved settings
	containerConfig := &container.Config{
		Image:        oldContainerInfo.Config.Image,
		Env:          oldContainerInfo.Config.Env,
		Labels:       oldContainerInfo.Config.Labels,
		ExposedPorts: exposedPorts,
		Cmd:          oldContainerInfo.Config.Cmd,
		Entrypoint:   oldContainerInfo.Config.Entrypoint,
		WorkingDir:   oldContainerInfo.Config.WorkingDir,
		User:         oldContainerInfo.Config.User,
	}

	// Convert MountPoints to Mounts - handle volume mounts correctly
	mounts := make([]mount.Mount, 0, len(oldContainerInfo.Mounts))
	for _, mp := range oldContainerInfo.Mounts {
		// For volume type mounts, use the volume name from Name field
		// For bind type mounts, use the source path
		source := mp.Source
		if mp.Type == mount.TypeVolume && mp.Name != "" {
			source = mp.Name
		}

		mounts = append(mounts, mount.Mount{
			Type:     mp.Type,
			Source:   source,
			Target:   mp.Destination,
			ReadOnly: !mp.RW,
		})
	}

	// Create host config using preserved settings
	hostConfig := &container.HostConfig{
		RestartPolicy: oldContainerInfo.HostConfig.RestartPolicy,
		Mounts:        mounts,
		LogConfig:     oldContainerInfo.HostConfig.LogConfig,
		PortBindings:  portBindings,
		Resources:     oldContainerInfo.HostConfig.Resources,
	}

	// Create container
	containerID, err := m.dockerClient.ContainerCreate(
		containerConfig,
		hostConfig,
		nil,
		instance.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Update container ID
	instance.ContainerID = containerID

	// Connect to network with aliases
	networkMgr := docker.NewNetworkManager(m.dockerClient)

	// Restore network aliases from labels
	aliases := []string{instance.ServiceType}
	if instance.Name != instance.ServiceType {
		aliases = append(aliases, instance.Name)
	}

	if err := networkMgr.ConnectContainerWithAliases("doku-network", containerID, aliases); err != nil {
		// Cleanup on failure
		m.dockerClient.ContainerRemove(instance.ContainerName, true)
		return fmt.Errorf("failed to connect to network: %w", err)
	}

	// Start container
	if err := m.dockerClient.ContainerStart(containerID); err != nil {
		// Cleanup on failure
		networkMgr.DisconnectContainer("doku-network", instance.ContainerName, true)
		m.dockerClient.ContainerRemove(instance.ContainerName, true)
		return fmt.Errorf("failed to start container: %w", err)
	}

	instance.Status = types.StatusRunning
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

	// Read all logs into string using strings.Builder for efficiency
	buf := make([]byte, 4096)
	var logs strings.Builder
	for {
		n, err := logsReader.Read(buf)
		if n > 0 {
			logs.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return logs.String(), nil
}

// runInitContainers runs init containers in dependency order
// Init containers run once to completion (e.g., migrations, setup scripts)
func (m *Manager) runInitContainers(spec *types.ServiceSpec, instanceName string) error {
	fmt.Println()
	color.Cyan("Running init containers...")
	fmt.Println()

	// Sort init containers by dependencies
	sorted, err := m.sortInitContainers(spec.InitContainers)
	if err != nil {
		return err
	}

	// Run each init container in order
	for _, initContainer := range sorted {
		fmt.Printf("Running %s...\n", initContainer.Name)

		// Prepare command
		cmd := initContainer.Command
		if len(cmd) == 0 {
			return fmt.Errorf("init container %s has no command", initContainer.Name)
		}

		// Prepare environment
		env := make([]string, 0, len(initContainer.Environment))
		for k, v := range initContainer.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		// Run container with --rm flag (auto-remove after completion)
		containerName := fmt.Sprintf("doku-%s-init-%s", instanceName, initContainer.Name)

		// Check if image exists locally first
		imageExists, err := m.dockerClient.ImageExists(initContainer.Image)
		if err != nil {
			return fmt.Errorf("failed to check image existence for init container %s: %w", initContainer.Image, err)
		}

		if imageExists {
			fmt.Printf("  Using cached init image %s\n", initContainer.Image)
		} else {
			// Pull image if not in cache
			fmt.Printf("  Pulling init image %s...\n", initContainer.Image)
			if err := m.dockerClient.ImagePull(initContainer.Image); err != nil {
				return fmt.Errorf("failed to pull init container image %s: %w", initContainer.Image, err)
			}
		}

		// Create and start the container
		containerID, err := m.dockerClient.RunContainer(
			initContainer.Image,
			containerName,
			cmd,
			env,
			"doku-network",
			true, // auto-remove after completion
		)
		if err != nil {
			return fmt.Errorf("failed to run init container %s: %w", initContainer.Name, err)
		}

		// Wait for container to complete
		if err := m.dockerClient.WaitForContainer(containerID); err != nil {
			// Get logs for debugging
			logs, _ := m.dockerClient.GetContainerLogsString(containerID)
			return fmt.Errorf("init container %s failed: %w\nLogs:\n%s", initContainer.Name, err, logs)
		}

		color.Green("✓ %s completed", initContainer.Name)
	}

	fmt.Println()
	return nil
}

// sortInitContainers sorts init containers by dependencies
func (m *Manager) sortInitContainers(initContainers []types.InitContainer) ([]types.InitContainer, error) {
	// Build dependency graph
	graph := make(map[string][]string)
	containerMap := make(map[string]types.InitContainer)

	for _, container := range initContainers {
		graph[container.Name] = container.DependsOn
		containerMap[container.Name] = container
	}

	// Topological sort
	var result []types.InitContainer
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected in init containers: %s", name)
		}
		if visited[name] {
			return nil
		}

		visiting[name] = true

		// Visit dependencies first
		for _, dep := range graph[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		result = append(result, containerMap[name])

		return nil
	}

	// Visit all containers
	for _, container := range initContainers {
		if !visited[container.Name] {
			if err := visit(container.Name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
