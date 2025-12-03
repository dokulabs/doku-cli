package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	restartPort    int
	restartRunInit bool
	restartEnv     []string
)

var restartCmd = &cobra.Command{
	Use:   "restart <service>",
	Short: "Restart a service",
	Long: `Restart a service instance.

The service will be stopped and then started again. Environment variables are
always loaded from the service's env file (~/.doku/services/<service>.env).

You can update environment variables while restarting:
  doku restart postgres --env POSTGRES_PASSWORD=newpass

You can also change the port mapping when restarting:
  doku restart postgres --port 5432   # Add or change port mapping
  doku restart postgres --port 0      # Remove port mapping

For multi-container services with init containers (e.g., database migrations),
use the --run-init flag to run init containers before restarting:
  doku restart signoz --run-init      # Run migrations before restart`,
	Args: cobra.ExactArgs(1),
	RunE: runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)

	restartCmd.Flags().IntVarP(&restartPort, "port", "p", -1, "Change host port mapping (0 to remove, -1 to keep current)")
	restartCmd.Flags().BoolVar(&restartRunInit, "run-init", false, "Run init containers before restarting (for multi-container services)")
	restartCmd.Flags().StringSliceVarP(&restartEnv, "env", "e", []string{}, "Update environment variables (KEY=VALUE), saved to env file")
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

	// Try service manager first
	serviceMgr := getServiceManager(dockerClient, cfgMgr)
	instance, err := serviceMgr.Get(instanceName)

	if err != nil {
		// Not found at all
		return fmt.Errorf("'%s' not found. Use 'doku list' or 'doku project list' to see installed services", instanceName)
	}

	// Check if it's a custom project
	if instance.ServiceType == "custom-project" {
		if restartRunInit {
			return fmt.Errorf("--run-init is not supported for custom projects")
		}
		return restartProject(instanceName, dockerClient, cfgMgr, restartEnv)
	}

	// Initialize catalog manager if --run-init is requested
	var catalogMgr *catalog.Manager
	if restartRunInit {
		catalogMgr = catalog.NewManager(cfgMgr.GetCatalogDir())
		if !catalogMgr.CatalogExists() {
			return fmt.Errorf("catalog not found. Run 'doku catalog update' first")
		}
	}

	// Update env file if --env flags were provided
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())
	if len(restartEnv) > 0 {
		// Parse env flags
		envUpdates := make(map[string]string)
		for _, e := range restartEnv {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", e)
			}
			envUpdates[parts[0]] = parts[1]
		}

		// Update env file
		envPath := envMgr.GetServiceEnvPath(instanceName, "")
		if err := envfile.UpdateEnvFile(envPath, envUpdates); err != nil {
			return fmt.Errorf("failed to update environment file: %w", err)
		}
		color.Green("✓ Updated environment file")
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
			if err := serviceMgr.RestartWithInit(instanceName, restartRunInit, catalogMgr); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}
		}
	} else {
		// No port change, just restart
		if err := serviceMgr.RestartWithInit(instanceName, restartRunInit, catalogMgr); err != nil {
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

func restartProject(projectName string, dockerClient *docker.Client, cfgMgr *config.Manager, envFlags []string) error {
	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to initialize project manager: %w", err)
	}

	// Check if project exists
	proj, err := projectMgr.Get(projectName)
	if err != nil {
		return fmt.Errorf("'%s' not found. Use 'doku list' or 'doku project list' to see installed services", projectName)
	}

	// Update env file if --env flags were provided
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())
	if len(envFlags) > 0 {
		// Parse env flags
		envUpdates := make(map[string]string)
		for _, e := range envFlags {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", e)
			}
			envUpdates[parts[0]] = parts[1]
		}

		// Update env file
		envPath := envMgr.GetProjectEnvPath(projectName)
		if err := envfile.UpdateEnvFile(envPath, envUpdates); err != nil {
			return fmt.Errorf("failed to update environment file: %w", err)
		}
		color.Green("✓ Updated environment file")
	}

	fmt.Printf("Restarting %s...\n", color.CyanString(projectName))

	// Stop the project
	if err := projectMgr.Stop(projectName); err != nil {
		return fmt.Errorf("failed to stop project: %w", err)
	}

	// Start the project
	if err := projectMgr.Start(projectName); err != nil {
		return fmt.Errorf("failed to start project: %w", err)
	}

	// Success message
	color.Green("✓ Project restarted successfully")

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
