package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
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
	Long: `Remove a service instance and clean up all associated resources.

This will:
  • Stop the service (if running)
  • Remove the container
  • Remove associated volumes (data will be lost!)
  • Remove from Doku configuration

Use --yes to skip confirmation prompt.`,
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

	// Show what will be removed
	fmt.Println()
	color.New(color.Bold, color.FgRed).Printf("⚠️  Remove Service: %s\n", instanceName)
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

	// Show volume information
	hasVolumes := len(instance.Volumes) > 0
	if hasVolumes {
		fmt.Printf("  • Volumes: %d volume(s)\n", len(instance.Volumes))
		for volumeName := range instance.Volumes {
			fmt.Printf("    - %s\n", volumeName)
		}
	}

	// Show dependencies
	if len(instance.Dependencies) > 0 {
		fmt.Printf("  • Dependencies: %s\n", strings.Join(instance.Dependencies, ", "))
		color.New(color.Faint).Println("    (Dependencies will NOT be removed)")
	}

	fmt.Printf("  • Configuration for: %s\n", instanceName)
	fmt.Println()

	// Confirmation (unless --yes flag)
	if !removeYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to remove '%s'?", instanceName),
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

	// Ask about volume removal if service has volumes
	removeVolumes := false
	if hasVolumes && !removeYes {
		fmt.Println()
		color.Yellow("⚠️  This service has Docker volumes containing data")
		volumePrompt := &survey.Confirm{
			Message: "Do you want to remove the volumes? (This will delete all data)",
			Default: false,
		}
		if err := survey.AskOne(volumePrompt, &removeVolumes); err != nil {
			return fmt.Errorf("volume prompt failed: %w", err)
		}
	} else if hasVolumes && removeYes {
		// With --yes flag, don't remove volumes by default for safety
		removeVolumes = false
		color.Yellow("⚠️  Volumes will be preserved (use 'doku remove' interactively to delete volumes)")
	}

	// Show progress
	fmt.Println()
	fmt.Printf("Removing %s...\n", color.CyanString(instanceName))

	// Remove the service
	if err := serviceMgr.Remove(instanceName, removeForce, removeVolumes); err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	// Show volume preservation message if applicable
	if hasVolumes && !removeVolumes {
		fmt.Println()
		color.Green("✓ Service removed (volumes preserved)")
	}

	// Success message
	fmt.Println()
	color.Green("✓ Service removed successfully")
	fmt.Println()

	// Show helpful next steps
	color.New(color.Faint).Println("To install a new service:")
	color.New(color.Faint).Printf("  doku install <service>\n")
	fmt.Println()
	color.New(color.Faint).Println("To see available services:")
	color.New(color.Faint).Printf("  doku catalog\n")
	fmt.Println()

	return nil
}
