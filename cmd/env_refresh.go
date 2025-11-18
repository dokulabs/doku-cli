package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envRefreshYes bool
)

var envRefreshCmd = &cobra.Command{
	Use:   "refresh <service>",
	Short: "Refresh environment variables from .env.doku file",
	Long: `Reload environment variables from the .env.doku file in the project directory.

This command:
  • Reads the .env.doku file from the project's path
  • Updates the service configuration with the new variables
  • Recreates the container to apply changes

This is useful when you've edited the .env.doku file and want to apply changes
without manually running install again.

Examples:
  doku env refresh myapp       # Refresh env from .env.doku and recreate container
  doku env refresh gw --yes    # Skip confirmation prompt`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvRefresh,
}

func init() {
	envCmd.AddCommand(envRefreshCmd)
	envRefreshCmd.Flags().BoolVarP(&envRefreshYes, "yes", "y", false, "Skip confirmation prompt")
}

func runEnvRefresh(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

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

	// Create service manager and get service
	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to create project manager: %w", err)
	}

	// Check if it's a custom project
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	projectConfig, isCustomProject := cfg.Projects[serviceName]
	if !isCustomProject {
		return fmt.Errorf("environment refresh is only supported for custom projects")
	}

	// Check for .env.doku file
	envDokuPath := filepath.Join(projectConfig.Path, ".env.doku")
	if !project.FileExists(envDokuPath) {
		return fmt.Errorf(".env.doku file not found at: %s", envDokuPath)
	}

	// Load environment variables from file
	fmt.Println()
	color.Cyan("Loading environment variables from .env.doku...")
	fileEnv, err := project.LoadEnvFile(envDokuPath)
	if err != nil {
		return fmt.Errorf("failed to load .env.doku: %w", err)
	}

	if len(fileEnv) == 0 {
		color.Yellow("⚠️  No environment variables found in .env.doku")
		return nil
	}

	fmt.Printf("Found %d environment variables\n", len(fileEnv))
	fmt.Println()

	// Show what will be loaded
	color.New(color.Bold).Println("Environment variables to be loaded:")
	for key, value := range fileEnv {
		if isSensitiveKey(key) {
			fmt.Printf("  %s = %s %s\n",
				color.YellowString(key),
				maskValue(value),
				color.New(color.Faint).Sprint("🔐"))
		} else {
			fmt.Printf("  %s = %s\n", color.CyanString(key), value)
		}
	}
	fmt.Println()

	// Confirm unless --yes flag
	if !envRefreshYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Update and recreate container with these environment variables?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Refresh cancelled")
			return nil
		}
		fmt.Println()
	}

	// Update project environment in config
	if err := cfgMgr.Update(func(c *types.Config) error {
		if p, exists := c.Projects[serviceName]; exists {
			p.Environment = fileEnv
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	color.Green("✓ Configuration updated")
	fmt.Println()

	// Recreate container with new environment
	color.Cyan("Recreating container to apply environment changes...")
	fmt.Println()

	runOpts := project.RunOptions{
		Name:   serviceName,
		Build:  false, // Don't rebuild image
		Detach: true,
	}
	if err := projectMgr.Run(runOpts); err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	fmt.Println()
	color.Green("✓ Environment variables refreshed successfully")
	fmt.Println()

	// Show success message with helpful info
	color.Cyan("Service '%s' is now running with updated environment", serviceName)
	fmt.Println()
	color.New(color.Faint).Println("Verify with:")
	color.New(color.Faint).Printf("  doku env %s --show-values\n", serviceName)
	color.New(color.Faint).Printf("  doku logs %s\n", serviceName)
	fmt.Println()

	return nil
}
