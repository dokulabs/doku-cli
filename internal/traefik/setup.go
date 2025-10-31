package traefik

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

const (
	TraefikContainerName = "doku-traefik"
	TraefikImage         = "traefik:v2.10"
	TraefikVersion       = "2.10"
)

// Manager handles Traefik setup and configuration
type Manager struct {
	dockerClient *docker.Client
	configDir    string
	certsDir     string
	domain       string
	protocol     string
}

// NewManager creates a new Traefik manager
func NewManager(dockerClient *docker.Client, configDir, certsDir, domain, protocol string) *Manager {
	return &Manager{
		dockerClient: dockerClient,
		configDir:    configDir,
		certsDir:     certsDir,
		domain:       domain,
		protocol:     protocol,
	}
}

// Setup sets up Traefik (configuration + container)
func (m *Manager) Setup() error {
	// Generate static configuration file
	if err := m.GenerateConfig(); err != nil {
		return fmt.Errorf("failed to generate Traefik config: %w", err)
	}

	// Generate dynamic configuration file
	if err := m.GenerateDynamicConfig(); err != nil {
		return fmt.Errorf("failed to generate Traefik dynamic config: %w", err)
	}

	// Start Traefik container
	if err := m.StartContainer(); err != nil {
		return fmt.Errorf("failed to start Traefik container: %w", err)
	}

	return nil
}

// StartContainer starts the Traefik container
func (m *Manager) StartContainer() error {
	// Check if container already exists
	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return err
	}

	if exists {
		fmt.Println("Traefik container already exists, restarting...")
		return m.RestartContainer()
	}

	// Pull Traefik image
	fmt.Printf("Pulling Traefik image %s...\n", TraefikImage)
	if err := m.dockerClient.ImagePull(TraefikImage); err != nil {
		return fmt.Errorf("failed to pull Traefik image: %w", err)
	}

	// Prepare container configuration
	config := &container.Config{
		Image: TraefikImage,
		Labels: map[string]string{
			"managed-by":     "doku",
			"doku.component": "traefik",
		},
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: m.createMounts(),
		PortBindings: m.createPortBindings(),
	}

	// Network configuration
	networkConfig := m.createNetworkConfig()

	// Create container
	containerID, err := m.dockerClient.ContainerCreate(
		config,
		hostConfig,
		networkConfig,
		TraefikContainerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create Traefik container: %w", err)
	}

	// Start container
	if err := m.dockerClient.ContainerStart(containerID); err != nil {
		return fmt.Errorf("failed to start Traefik container: %w", err)
	}

	fmt.Printf("✓ Traefik started successfully\n")
	fmt.Printf("  Dashboard: %s://%s.%s\n", m.protocol, "traefik", m.domain)

	return nil
}

// StopContainer stops the Traefik container
func (m *Manager) StopContainer() error {
	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("Traefik container not found")
	}

	timeout := 10
	return m.dockerClient.ContainerStop(TraefikContainerName, &timeout)
}

// RestartContainer restarts the Traefik container
func (m *Manager) RestartContainer() error {
	timeout := 10
	return m.dockerClient.ContainerRestart(TraefikContainerName, &timeout)
}

// RemoveContainer removes the Traefik container
func (m *Manager) RemoveContainer() error {
	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return err
	}

	if !exists {
		return nil // Already removed
	}

	return m.dockerClient.ContainerRemove(TraefikContainerName, true)
}

// IsRunning checks if Traefik container is running
func (m *Manager) IsRunning() (bool, error) {
	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	info, err := m.dockerClient.ContainerInspect(TraefikContainerName)
	if err != nil {
		return false, err
	}

	return info.State.Running, nil
}

// createMounts creates volume mounts for Traefik container
func (m *Manager) createMounts() []mount.Mount {
	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   "/var/run/docker.sock",
			Target:   "/var/run/docker.sock",
			ReadOnly: true,
		},
		{
			Type:     mount.TypeBind,
			Source:   filepath.Join(m.configDir, "traefik.yml"),
			Target:   "/etc/traefik/traefik.yml",
			ReadOnly: true,
		},
		{
			Type:     mount.TypeBind,
			Source:   filepath.Join(m.configDir, "dynamic.yml"),
			Target:   "/etc/traefik/dynamic.yml",
			ReadOnly: true,
		},
	}

	// Add certificates mount if using HTTPS
	if m.protocol == "https" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.certsDir,
			Target:   "/certs",
			ReadOnly: true,
		})
	}

	return mounts
}

// createPortBindings creates port bindings for Traefik
func (m *Manager) createPortBindings() nat.PortMap {
	bindings := nat.PortMap{
		"80/tcp": {
			{HostIP: "0.0.0.0", HostPort: "80"},
		},
		"443/tcp": {
			{HostIP: "0.0.0.0", HostPort: "443"},
		},
	}

	return bindings
}

// createNetworkConfig creates network configuration for Traefik
func (m *Manager) createNetworkConfig() *network.NetworkingConfig {
	return &network.NetworkingConfig{
		// Traefik will connect to doku-network later
	}
}

// GetDashboardURL returns the Traefik dashboard URL
func (m *Manager) GetDashboardURL() string {
	return fmt.Sprintf("%s://traefik.%s", m.protocol, m.domain)
}

// EnsureRunning ensures Traefik is running, starts it if not
func (m *Manager) EnsureRunning() error {
	running, err := m.IsRunning()
	if err != nil {
		return err
	}

	if running {
		return nil
	}

	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return err
	}

	if exists {
		// Container exists but not running, start it
		return m.dockerClient.ContainerStart(TraefikContainerName)
	}

	// Container doesn't exist, set it up
	return m.Setup()
}

// GetStatus returns the status of Traefik
func (m *Manager) GetStatus() (map[string]interface{}, error) {
	exists, err := m.dockerClient.ContainerExists(TraefikContainerName)
	if err != nil {
		return nil, err
	}

	if !exists {
		return map[string]interface{}{
			"installed": false,
			"running":   false,
		}, nil
	}

	info, err := m.dockerClient.ContainerInspect(TraefikContainerName)
	if err != nil {
		return nil, err
	}

	status := map[string]interface{}{
		"installed":     true,
		"running":       info.State.Running,
		"container_id":  info.ID[:12],
		"image":         info.Config.Image,
		"started_at":    info.State.StartedAt,
		"dashboard_url": m.GetDashboardURL(),
	}

	return status, nil
}

// UpdateConfig updates Traefik configuration and restarts the container
func (m *Manager) UpdateConfig() error {
	// Generate new configuration
	if err := m.GenerateConfig(); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Restart container to apply new config
	return m.RestartContainer()
}

// GetConfigPath returns the path to the Traefik configuration file
func (m *Manager) GetConfigPath() string {
	return filepath.Join(m.configDir, "traefik.yml")
}

// ConfigExists checks if Traefik configuration file exists
func (m *Manager) ConfigExists() bool {
	_, err := os.Stat(m.GetConfigPath())
	return err == nil
}
