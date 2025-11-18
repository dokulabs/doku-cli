package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	restartPort int
)

var restartCmd = &cobra.Command{
	Use:   "restart <service>",
	Short: "Restart a service",
	Long: `Restart a service instance.

The service will be stopped and then started again.
This is useful when you need to apply configuration changes or recover from errors.

You can also change the port mapping when restarting:
  doku restart postgres --port 5432   # Add or change port mapping
  doku restart postgres --port 0      # Remove port mapping`,
	Args: cobra.ExactArgs(1),
	RunE: runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)

	restartCmd.Flags().IntVarP(&restartPort, "port", "p", -1, "Change host port mapping (0 to remove, -1 to keep current)")
}

func runRestart(cmd *cobra.Command, args []string) error {
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
	handled, err := handleTraefikCommand(instanceName, TraefikActionRestart, dockerClient, cfgMgr)
	if handled {
		return err
	}

	// Create service manager
	serviceMgr := getServiceManager(dockerClient, cfgMgr)

	// Get instance to check if it exists
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	fmt.Printf("Restarting %s...\n", color.CyanString(instanceName))

	// Check if port flag was provided
	if restartPort != -1 {
		// Port change requested - need to recreate container
		if restartPort != instance.Network.HostPort {
			fmt.Printf("Changing port mapping: %d → %d\n", instance.Network.HostPort, restartPort)
			if err := serviceMgr.RestartWithPort(instanceName, restartPort); err != nil {
				return fmt.Errorf("failed to restart service with new port: %w", err)
			}
			// Update instance reference
			instance, err = serviceMgr.Get(instanceName)
			if err != nil {
				return fmt.Errorf("failed to get updated instance: %w", err)
			}
		} else {
			// Same port, just do normal restart
			if err := serviceMgr.Restart(instanceName); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}
		}
	} else {
		// No port change, just restart
		if err := serviceMgr.Restart(instanceName); err != nil {
			return fmt.Errorf("failed to restart service: %w", err)
		}
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

	// Show host port mappings if configured
	if len(instance.Network.PortMappings) > 0 {
		fmt.Println("Port mappings:")
		for containerPort, hostPort := range instance.Network.PortMappings {
			fmt.Printf("  localhost:%s → container:%s\n", hostPort, containerPort)
		}
	} else if instance.Network.HostPort > 0 {
		// Backward compatibility with old single port format
		fmt.Printf("Host port: localhost:%d → container:%d\n", instance.Network.HostPort, instance.Network.InternalPort)
	}

	// Show helpful commands
	fmt.Println()
	color.New(color.Faint).Printf("Use 'doku info %s' to see full details\n", instanceName)
	color.New(color.Faint).Printf("Use 'doku logs %s -f' to view logs\n", instanceName)

	return nil
}
