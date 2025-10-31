package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	removeForce  bool
	removeYes    bool
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
	fmt.Printf("  • Container: %s\n", instance.ContainerName)
	if len(instance.Volumes) > 0 {
		fmt.Printf("  • Volumes: %d volume(s) ", len(instance.Volumes))
		color.Red("(data will be lost!)")
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

	// Show progress
	fmt.Println()
	fmt.Printf("Removing %s...\n", color.CyanString(instanceName))

	// Remove the service
	if err := serviceMgr.Remove(instanceName, removeForce); err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
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
