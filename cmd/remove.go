package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	removeForce bool
	removeYes   bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <service>",
	Short: "Remove a service instance",
	Long: `Remove a service instance while preserving data for safety.

This will:
  • Stop the service (if running)
  • Remove the container
  • Remove from Doku configuration

Data is preserved for safety:
  • Docker volumes (your data) are NOT removed
  • Environment files (~/.doku/services/<service>.env) are NOT removed

After removal, manual cleanup instructions will be shown if you want to
permanently delete the data.

Use --yes to skip confirmation prompt.
Use --force to force removal even if container is running.`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"rm", "delete"},
	RunE:    runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal (even if running)")
	removeCmd.Flags().BoolVarP(&removeYes, "yes", "y", false, "Skip confirmation prompt")
}

func runRemove(cmd *cobra.Command, args []string) error {
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

	// Prevent removal of Traefik (system component)
	if instanceName == "traefik" || instanceName == "doku-traefik" {
		fmt.Println()
		color.Red("✗ Cannot remove Traefik")
		fmt.Println()
		color.Yellow("Traefik is a core system component required for all services.")
		fmt.Println()
		color.New(color.Bold).Println("To remove Traefik along with all Doku components:")
		fmt.Printf("  %s\n", color.CyanString("doku uninstall"))
		fmt.Println()
		color.New(color.Faint).Println("This will remove:")
		color.New(color.Faint).Println("  • All services")
		color.New(color.Faint).Println("  • Traefik reverse proxy")
		color.New(color.Faint).Println("  • Docker network")
		color.New(color.Faint).Println("  • SSL certificates")
		color.New(color.Faint).Println("  • All configuration")
		fmt.Println()
		return nil
	}

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance to check if it exists
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list --all' to see all services", instanceName)
	}

	// Collect volume and env file information for cleanup instructions
	var volumeNames []string
	var envFilePaths []string

	// Get env file path
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())

	if instance.IsMultiContainer {
		// Multi-container: collect per-container env files
		for _, container := range instance.Containers {
			envPath := envMgr.GetServiceEnvPath(instanceName, container.Name)
			if envMgr.Exists(envPath) {
				envFilePaths = append(envFilePaths, envPath)
			}
		}
		// Also check main service env file
		mainEnvPath := envMgr.GetServiceEnvPath(instanceName, "")
		if envMgr.Exists(mainEnvPath) {
			envFilePaths = append(envFilePaths, mainEnvPath)
		}
	} else {
		envPath := envMgr.GetServiceEnvPath(instanceName, "")
		if envMgr.Exists(envPath) {
			envFilePaths = append(envFilePaths, envPath)
		}
	}

	// Get volume names from Docker
	if instance.IsMultiContainer {
		for _, container := range instance.Containers {
			containerInfo, err := dockerClient.ContainerInspect(container.FullName)
			if err == nil {
				for _, mount := range containerInfo.Mounts {
					if mount.Type == "volume" && strings.HasPrefix(mount.Name, "doku-") {
						volumeNames = append(volumeNames, mount.Name)
					}
				}
			}
		}
	} else {
		containerInfo, err := dockerClient.ContainerInspect(instance.ContainerName)
		if err == nil {
			for _, mount := range containerInfo.Mounts {
				if mount.Type == "volume" && strings.HasPrefix(mount.Name, "doku-") {
					volumeNames = append(volumeNames, mount.Name)
				}
			}
		}
	}

	// Show what will be removed
	fmt.Println()
	color.New(color.Bold, color.FgYellow).Printf("Remove Service: %s\n", instanceName)
	fmt.Println()
	fmt.Println("This will remove:")

	// Show container information
	if instance.IsMultiContainer {
		fmt.Printf("  • Multi-container service (%d containers):\n", len(instance.Containers))
		for _, container := range instance.Containers {
			fmt.Printf("    - %s\n", container.Name)
		}
	} else {
		fmt.Printf("  • Container: %s\n", instance.ContainerName)
	}

	fmt.Printf("  • Configuration entry\n")
	fmt.Println()

	// Show what will be preserved
	if len(volumeNames) > 0 || len(envFilePaths) > 0 {
		color.Green("Data preserved (for safety):")
		if len(volumeNames) > 0 {
			fmt.Printf("  • %d Docker volume(s)\n", len(volumeNames))
		}
		if len(envFilePaths) > 0 {
			fmt.Printf("  • %d environment file(s)\n", len(envFilePaths))
		}
		fmt.Println()
	}

	// Show dependencies
	if len(instance.Dependencies) > 0 {
		color.New(color.Faint).Printf("Dependencies (%s) will NOT be removed\n", strings.Join(instance.Dependencies, ", "))
		fmt.Println()
	}

	// Confirmation (unless --yes flag)
	if !removeYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Remove '%s'? (data will be preserved)", instanceName),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}

		if !confirm {
			color.Yellow("Removal cancelled")
			return nil
		}
	}

	// Show progress
	fmt.Println()
	fmt.Printf("Removing %s...\n", color.CyanString(instanceName))

	// Remove the service (always preserve volumes)
	if err := serviceMgr.Remove(instanceName, removeForce, false); err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	// Success message
	fmt.Println()
	color.Green("✓ Service '%s' removed successfully", instanceName)
	fmt.Println()

	// Show cleanup instructions if there's data to clean up
	if len(volumeNames) > 0 || len(envFilePaths) > 0 {
		color.Yellow("Data preserved for safety. To permanently delete:")
		fmt.Println()

		if len(volumeNames) > 0 {
			color.New(color.Bold).Println("Docker volumes:")
			for _, vol := range volumeNames {
				fmt.Printf("  docker volume rm %s\n", vol)
			}
			fmt.Println()
		}

		if len(envFilePaths) > 0 {
			color.New(color.Bold).Println("Environment files:")
			for _, envPath := range envFilePaths {
				fmt.Printf("  rm %s\n", envPath)
			}
			fmt.Println()
		}

		color.New(color.Faint).Println("Or reinstall to reuse the existing data:")
		color.New(color.Faint).Printf("  doku install %s\n", instance.ServiceType)
		fmt.Println()
	}

	return nil
}
