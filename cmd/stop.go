package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
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

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if initialized
	if !cfgMgr.IsInitialized() {
		color.Yellow("⚠️  Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance to check if it exists
	_, err = serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	fmt.Printf("Stopping %s...\n", color.CyanString(instanceName))

	// Stop the service
	if err := serviceMgr.Stop(instanceName); err != nil {
		// Check if already stopped
		if err.Error() == fmt.Sprintf("instance '%s' is already stopped", instanceName) {
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
