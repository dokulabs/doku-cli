package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envSetRestart bool
)

var envSetCmd = &cobra.Command{
	Use:   "set <service> <KEY=VALUE> [KEY2=VALUE2...]",
	Short: "Set environment variables for a service",
	Long: `Set or update environment variables for an installed service.

Environment variables are saved to the service's env file:
  ~/.doku/services/<service>.env

The service will need to be restarted for changes to take effect.
Use --restart to automatically restart the service.

Examples:
  # Set a single environment variable
  doku env set postgres POSTGRES_PASSWORD=newpassword

  # Set multiple environment variables
  doku env set frontend API_URL=https://api.example.com NODE_ENV=production

  # Set and automatically restart
  doku env set redis REDIS_PASSWORD=secret --restart`,
	Args: cobra.MinimumNArgs(2),
	RunE: runEnvSet,
}

func init() {
	envCmd.AddCommand(envSetCmd)
	envSetCmd.Flags().BoolVarP(&envSetRestart, "restart", "r", false, "Restart service after setting variables")
}

func runEnvSet(cmd *cobra.Command, args []string) error {
	instanceName := args[0]
	envVars := args[1:]

	// Parse environment variables
	envMap := make(map[string]string)
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid environment variable format: %s (use KEY=VALUE)", envVar)
		}
		envMap[parts[0]] = parts[1]
	}

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
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	isCustomProject := instance.ServiceType == "custom-project"

	// Get env file path
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())
	var envPath string
	if isCustomProject {
		envPath = envMgr.GetProjectEnvPath(instanceName)
	} else {
		envPath = envMgr.GetServiceEnvPath(instanceName, "")
	}

	// Load existing env (for display purposes)
	existingEnv, _ := envMgr.Load(envPath)
	if existingEnv == nil {
		existingEnv = make(map[string]string)
	}

	fmt.Println()
	color.Cyan("Updating environment variables for %s", instance.Name)
	fmt.Println()

	for key, value := range envMap {
		oldValue := existingEnv[key]
		if oldValue != "" {
			fmt.Printf("  %s: %s → %s\n", color.YellowString(key), color.RedString(oldValue), color.GreenString(value))
		} else {
			fmt.Printf("  %s: %s\n", color.GreenString(key), color.CyanString(value))
		}
	}

	// Update env file
	if err := envfile.UpdateEnvFile(envPath, envMap); err != nil {
		return fmt.Errorf("failed to update environment file: %w", err)
	}

	fmt.Println()
	color.Green("✓ Environment variables saved to %s", envPath)
	fmt.Println()

	// Restart if requested
	if envSetRestart {
		color.Cyan("Recreating container to apply changes...")
		if err := serviceMgr.Recreate(instanceName); err != nil {
			return fmt.Errorf("failed to recreate service: %w", err)
		}
		color.Green("✓ Service recreated with new environment")
		fmt.Println()
	} else {
		color.Yellow("⚠️  Note: Service needs to be restarted for changes to take effect")
		fmt.Printf("   Run: doku restart %s\n", instanceName)
		fmt.Println()
	}

	return nil
}
