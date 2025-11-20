package cmd

import (
	"errors"
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <service>",
	Short: "Start a stopped service",
	Long: `Start a stopped service instance.

The service will be started using its existing configuration.
All settings (environment variables, volumes, network) remain the same.`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	instanceName := args[0]

	// Initialize config manager
	cfgMgr, err := initConfigManager()
	if err != nil {
		if err == types.ErrNotInitialized {
			return nil
		}
		return err
	}

	// Initialize Docker client
	dockerClient, err := initDockerClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Handle Traefik command
	handled, err := handleTraefikCommand(instanceName, TraefikActionStart, dockerClient, cfgMgr)
	if handled {
		return err
	}

	// Create service manager
	serviceMgr := getServiceManager(dockerClient, cfgMgr)

	// Try service manager first
	instance, err := serviceMgr.Get(instanceName)

	if err != nil {
		// Not found at all
		return fmt.Errorf("'%s' not found. Use 'doku list --all' to see all services", instanceName)
	}

	// Check if it's a custom project
	if instance.ServiceType == "custom-project" {
		return startProject(instanceName, dockerClient, cfgMgr)
	}

	fmt.Printf("Starting %s...\n", color.CyanString(instanceName))

	// Start the service
	if err := serviceMgr.Start(instanceName); err != nil {
		// Check if already running
		if errors.Is(err, types.ErrAlreadyRunning) {
			color.Yellow("⚠️  Service is already running")

			// Show URL if available
			if instance.Traefik.Enabled && instance.URL != "" {
				fmt.Printf("Access at: %s\n", color.GreenString(instance.URL))
			}
			return nil
		}
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Success message
	color.Green("✓ Service started successfully")

	// Show access information
	fmt.Println()
	if instance.Traefik.Enabled && instance.URL != "" {
		fmt.Printf("Access at: %s\n", color.GreenString(instance.URL))
	} else {
		fmt.Printf("Service: %s (internal only)\n", instance.ServiceType)
		if instance.Network.InternalPort > 0 {
			fmt.Printf("Port: %d\n", instance.Network.InternalPort)
		}
	}

	// Show helpful commands
	fmt.Println()
	color.New(color.Faint).Printf("Use 'doku info %s' to see full details\n", instanceName)
	color.New(color.Faint).Printf("Use 'doku logs %s -f' to view logs\n", instanceName)

	return nil
}

func startProject(projectName string, dockerClient *docker.Client, cfgMgr *config.Manager) error {
	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to initialize project manager: %w", err)
	}

	// Check if project exists
	proj, err := projectMgr.Get(projectName)
	if err != nil {
		return fmt.Errorf("'%s' not found. Use 'doku list' or 'doku project list' to see installed services", projectName)
	}

	fmt.Printf("Starting %s...\n", color.CyanString(projectName))

	// Start the project
	if err := projectMgr.Start(projectName); err != nil {
		return fmt.Errorf("failed to start project: %w", err)
	}

	// Success message
	color.Green("✓ Project started successfully")

	// Show access information
	fmt.Println()
	if proj.URL != "" {
		fmt.Printf("Access at: %s\n", color.GreenString(proj.URL))
	} else {
		fmt.Printf("Project: %s (internal only)\n", proj.Name)
		if proj.Port > 0 {
			fmt.Printf("Port: %d\n", proj.Port)
		}
	}

	// Show helpful commands
	fmt.Println()
	color.New(color.Faint).Printf("Use 'doku logs %s -f' to view logs\n", projectName)

	return nil
}
