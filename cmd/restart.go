package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <service>",
	Short: "Restart a service",
	Long: `Restart a service instance.

The service will be stopped and then started again.
This is useful when you need to apply configuration changes or recover from errors.`,
	Args: cobra.ExactArgs(1),
	RunE: runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
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
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	fmt.Printf("Restarting %s...\n", color.CyanString(instanceName))

	// Restart the service
	if err := serviceMgr.Restart(instanceName); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	// Success message
	color.Green("✓ Service restarted successfully")

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
