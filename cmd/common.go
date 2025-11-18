package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/constants"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
)

// initConfigManager creates and initializes a config manager
// Returns an error if the config manager cannot be created or if Doku is not initialized
func initConfigManager() (*config.Manager, error) {
	cfgMgr, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("⚠️  Doku is not initialized. Run 'doku init' first.")
		return nil, types.ErrNotInitialized
	}

	return cfgMgr, nil
}

// initDockerClient creates and returns a Docker client
func initDockerClient() (*docker.Client, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return dockerClient, nil
}

// getServiceManager creates a service manager with the given Docker client and config manager
func getServiceManager(dockerClient *docker.Client, cfgMgr *config.Manager) *service.Manager {
	return service.NewManager(dockerClient, cfgMgr)
}

// TraefikAction represents an action to perform on Traefik
type TraefikAction string

const (
	TraefikActionStart   TraefikAction = "start"
	TraefikActionStop    TraefikAction = "stop"
	TraefikActionRestart TraefikAction = "restart"
	TraefikActionInfo    TraefikAction = "info"
)

// handleTraefikCommand handles Traefik-specific commands (start, stop, restart)
// Returns true if the instance was Traefik and was handled, false otherwise
func handleTraefikCommand(instanceName string, action TraefikAction, dockerClient *docker.Client, cfgMgr *config.Manager) (handled bool, err error) {
	// Check if this is a Traefik command
	if instanceName != "traefik" && instanceName != "doku-traefik" {
		return false, nil
	}

	containerName := constants.TraefikContainerName

	// Check if container exists
	exists, err := dockerClient.ContainerExists(containerName)
	if err != nil || !exists {
		return true, fmt.Errorf("Traefik container not found. Run 'doku init' first")
	}

	// Get container info
	containerInfo, err := dockerClient.ContainerInspect(containerName)
	if err != nil {
		return true, fmt.Errorf("failed to inspect Traefik container: %w", err)
	}

	// Perform action
	switch action {
	case TraefikActionStart:
		// Check if already running
		if containerInfo.State.Running {
			color.Yellow("⚠️  Traefik is already running")
			cfg, err := cfgMgr.Get()
			if err != nil {
				return true, fmt.Errorf("failed to get configuration: %w", err)
			}
			fmt.Printf("Dashboard: %s://traefik.%s\n", cfg.Preferences.Protocol, cfg.Preferences.Domain)
			return true, nil
		}

		fmt.Println("Starting Traefik...")
		if err := dockerClient.ContainerStart(containerInfo.ID); err != nil {
			return true, fmt.Errorf("failed to start Traefik: %w", err)
		}

		color.Green("✓ Traefik started successfully")
		cfg, err := cfgMgr.Get()
		if err != nil {
			return true, fmt.Errorf("failed to get configuration: %w", err)
		}
		fmt.Printf("Dashboard: %s://traefik.%s\n", cfg.Preferences.Protocol, cfg.Preferences.Domain)
		return true, nil

	case TraefikActionStop:
		// Check if already stopped
		if !containerInfo.State.Running {
			color.Yellow("⚠️  Traefik is already stopped")
			return true, nil
		}

		color.Yellow("⚠️  Warning: Stopping Traefik will make all services inaccessible")
		fmt.Println("Stopping Traefik...")

		timeout := constants.DefaultContainerTimeout
		if err := dockerClient.ContainerStop(containerName, &timeout); err != nil {
			return true, fmt.Errorf("failed to stop Traefik: %w", err)
		}

		color.Green("✓ Traefik stopped successfully")
		color.New(color.Faint).Println("Use 'doku start traefik' to start it again")
		return true, nil

	case TraefikActionRestart:
		fmt.Println("Restarting Traefik...")

		timeout := constants.DefaultContainerTimeout
		if err := dockerClient.ContainerRestart(containerName, &timeout); err != nil {
			return true, fmt.Errorf("failed to restart Traefik: %w", err)
		}

		color.Green("✓ Traefik restarted successfully")
		cfg, err := cfgMgr.Get()
		if err != nil {
			return true, fmt.Errorf("failed to get configuration: %w", err)
		}
		fmt.Printf("Dashboard: %s://traefik.%s\n", cfg.Preferences.Protocol, cfg.Preferences.Domain)
		return true, nil

	default:
		return true, fmt.Errorf("unknown Traefik action: %s", action)
	}
}
