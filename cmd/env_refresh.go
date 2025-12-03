package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envRefreshYes bool
)

var envRefreshCmd = &cobra.Command{
	Use:   "refresh <service>",
	Short: "Reload and apply environment variables from env file",
	Long: `Reload environment variables from the service's env file and recreate the container.

This command:
  • Reads the environment file (~/.doku/services/<service>.env)
  • Recreates the container with the current environment variables

This is useful when you've manually edited the env file and want to apply changes.

Examples:
  doku env refresh postgres     # Reload env from file and recreate container
  doku env refresh myapp --yes  # Skip confirmation prompt`,
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

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance
	instance, err := serviceMgr.Get(serviceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", serviceName)
	}

	isCustomProject := instance.ServiceType == "custom-project"

	// Get env file path
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())
	var envPath string
	if isCustomProject {
		envPath = envMgr.GetProjectEnvPath(serviceName)
	} else {
		envPath = envMgr.GetServiceEnvPath(serviceName, "")
	}

	// Check if env file exists
	if !envMgr.Exists(envPath) {
		return fmt.Errorf("environment file not found: %s\nUse 'doku env edit %s' to create one", envPath, serviceName)
	}

	// Load environment variables from file
	fmt.Println()
	color.Cyan("Loading environment variables from %s...", envPath)
	fileEnv, err := envMgr.Load(envPath)
	if err != nil {
		return fmt.Errorf("failed to load env file: %w", err)
	}

	if len(fileEnv) == 0 {
		color.Yellow("⚠️  No environment variables found in env file")
		return nil
	}

	fmt.Printf("Found %d environment variables\n", len(fileEnv))
	fmt.Println()

	// Show what will be loaded
	color.New(color.Bold).Println("Environment variables to be applied:")
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
			Message: "Recreate container with these environment variables?",
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

	// Recreate container with new environment
	color.Cyan("Recreating container to apply environment changes...")
	fmt.Println()

	if isCustomProject {
		projectMgr, err := project.NewManager(dockerClient, cfgMgr)
		if err != nil {
			return fmt.Errorf("failed to create project manager: %w", err)
		}

		runOpts := project.RunOptions{
			Name:   serviceName,
			Build:  false, // Don't rebuild image
			Detach: true,
		}
		if err := projectMgr.Run(runOpts); err != nil {
			return fmt.Errorf("failed to recreate container: %w", err)
		}
	} else {
		if err := serviceMgr.Recreate(serviceName); err != nil {
			return fmt.Errorf("failed to recreate container: %w", err)
		}
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
