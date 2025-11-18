package cmd

import (
	"errors"
	"fmt"

	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <service>",
	Short: "Stop a running service",
	Long: `Stop a running service instance.

The service container will be stopped but not removed.
All data in volumes is preserved and the service can be restarted.`,
	Args: cobra.ExactArgs(1),
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
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
	handled, err := handleTraefikCommand(instanceName, TraefikActionStop, dockerClient, cfgMgr)
	if handled {
		return err
	}

	// Create service manager
	serviceMgr := getServiceManager(dockerClient, cfgMgr)

	// Get instance to check if it exists
	_, err = serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	fmt.Printf("Stopping %s...\n", color.CyanString(instanceName))

	// Stop the service
	if err := serviceMgr.Stop(instanceName); err != nil {
		// Check if already stopped
		if errors.Is(err, types.ErrAlreadyStopped) {
			color.Yellow("⚠️  Service is already stopped")
			return nil
		}
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Success message
	color.Green("✓ Service stopped successfully")

	// Show helpful commands
	fmt.Println()
	color.New(color.Faint).Printf("Use 'doku start %s' to restart the service\n", instanceName)
	color.New(color.Faint).Printf("Use 'doku remove %s' to completely remove the service\n", instanceName)

	return nil
}
