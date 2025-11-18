package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envUnsetRestart bool
)

var envUnsetCmd = &cobra.Command{
	Use:   "unset <service> <KEY> [KEY2...]",
	Short: "Unset environment variables for a service",
	Long: `Remove environment variables from an installed service.

The service will need to be restarted for changes to take effect.

Examples:
  # Unset a single environment variable
  doku env unset postgres POSTGRES_PASSWORD

  # Unset multiple environment variables
  doku env unset frontend API_URL NODE_ENV

  # Unset and automatically restart
  doku env unset redis REDIS_PASSWORD --restart`,
	Args: cobra.MinimumNArgs(2),
	RunE: runEnvUnset,
}

func init() {
	envCmd.AddCommand(envUnsetCmd)
	envUnsetCmd.Flags().BoolVarP(&envUnsetRestart, "restart", "r", false, "Restart service after unsetting variables")
}

func runEnvUnset(cmd *cobra.Command, args []string) error {
	instanceName := args[0]
	keys := args[1:]

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

	// Get instance
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found", instanceName)
	}

	fmt.Println()
	color.Cyan("Removing environment variables from %s", instance.Name)
	fmt.Println()

	// Remove keys
	removedCount := 0
	for _, key := range keys {
		if _, exists := instance.Environment[key]; exists {
			fmt.Printf("  %s %s\n", color.RedString("✗"), key)
			delete(instance.Environment, key)
			removedCount++
		} else {
			fmt.Printf("  %s %s (not found)\n", color.YellowString("⚠"), key)
		}
	}

	if removedCount == 0 {
		fmt.Println()
		color.Yellow("No environment variables were removed")
		return nil
	}

	// Save changes
	err = cfgMgr.Update(func(c *types.Config) error {
		if inst, exists := c.Instances[instanceName]; exists {
			for _, key := range keys {
				delete(inst.Environment, key)
			}
		}
		// Also check in Projects
		if proj, exists := c.Projects[instanceName]; exists {
			for _, key := range keys {
				delete(proj.Environment, key)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to save changes: %w", err)
	}

	fmt.Println()
	color.Green("✓ Removed %d environment variable(s)", removedCount)
	fmt.Println()

	// Restart if requested
	if envUnsetRestart {
		color.Cyan("Restarting service...")
		if err := serviceMgr.Restart(instanceName); err != nil {
			return fmt.Errorf("failed to restart service: %w", err)
		}
		color.Green("✓ Service restarted")
		fmt.Println()
	} else {
		color.Yellow("⚠️  Note: Service needs to be restarted for changes to take effect")
		fmt.Printf("   Run: doku restart %s\n", instanceName)
		fmt.Println()
	}

	return nil
}
