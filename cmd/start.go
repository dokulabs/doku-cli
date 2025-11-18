package cmd

import (
	"errors"
	"fmt"

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

	// Get instance to check if it exists
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list --all' to see all services", instanceName)
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
