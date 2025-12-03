package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envShowValues bool
	envExport     bool
	envShowFile   bool
)

var envCmd = &cobra.Command{
	Use:   "env <service>",
	Short: "Display environment variables for a service",
	Long: `Display environment variables configured for an installed service.

Environment variables are read from the service's env file:
  ~/.doku/services/<service>.env

By default, sensitive values (passwords, tokens, etc.) are masked for security.
Use --show-values to display actual values.

Examples:
  doku env postgres                 # Show environment variables (masked)
  doku env postgres --show-values   # Show actual values
  doku env rabbitmq --export        # Show in export format for shell
  doku env postgres --file          # Show env file location`,
	Args: cobra.ExactArgs(1),
	RunE: runEnv,
}

func init() {
	rootCmd.AddCommand(envCmd)

	envCmd.Flags().BoolVarP(&envShowValues, "show-values", "s", false, "Show actual values (unmask sensitive data)")
	envCmd.Flags().BoolVarP(&envExport, "export", "e", false, "Output in shell export format")
	envCmd.Flags().BoolVar(&envShowFile, "file", false, "Show env file location")
}

func runEnv(cmd *cobra.Command, args []string) error {
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

	// If --file flag, just show the file location
	if envShowFile {
		fmt.Println()
		fmt.Printf("Environment file: %s\n", envPath)
		if envMgr.Exists(envPath) {
			color.Green("  (exists)")
		} else {
			color.Yellow("  (not created yet - will be created on edit)")
		}
		fmt.Println()
		return nil
	}

	// Load environment from env file
	env, err := envMgr.Load(envPath)
	if err != nil {
		// Fall back to instance.Environment for backward compatibility
		env = instance.Environment
	}

	// If still no env, try to migrate from config
	if len(env) == 0 && len(instance.Environment) > 0 {
		env = instance.Environment
		// Save to env file for future use
		if err := envMgr.Save(envPath, env); err == nil {
			color.Green("✓ Migrated environment to %s", envPath)
		}
	}

	// Check if there are any environment variables
	if len(env) == 0 {
		fmt.Println()
		color.Yellow("No environment variables configured for %s", instance.Name)
		fmt.Printf("File: %s\n", envPath)
		fmt.Println()
		return nil
	}

	// Display environment variables
	displayEnvironmentVariables(instance.Name, env, envShowValues, envExport, envPath)

	return nil
}

func displayEnvironmentVariables(serviceName string, env map[string]string, showValues bool, exportFormat bool, envPath string) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if exportFormat {
		// Export format for shell
		fmt.Println()
		for _, key := range keys {
			value := env[key]
			if !showValues && isSensitiveKey(key) {
				value = maskValue(value)
			}
			// Escape quotes and special characters for shell
			value = strings.ReplaceAll(value, "\"", "\\\"")
			fmt.Printf("export %s=\"%s\"\n", key, value)
		}
		fmt.Println()
	} else {
		// Pretty format
		fmt.Println()
		color.New(color.Bold, color.FgCyan).Printf("Environment Variables for %s\n", serviceName)
		fmt.Println(strings.Repeat("=", len(serviceName)+25))
		fmt.Printf("File: %s\n", envPath)
		fmt.Println()

		if !showValues {
			color.New(color.Faint).Println("🔒 Sensitive values are masked. Use --show-values to display actual values.")
			fmt.Println()
		}

		for _, key := range keys {
			value := env[key]
			sensitive := isSensitiveKey(key)

			// Display the key
			if sensitive {
				fmt.Printf("  %s ", color.YellowString(key))
				fmt.Print(color.New(color.Faint).Sprint("🔐"))
			} else {
				fmt.Printf("  %s", color.CyanString(key))
			}

			// Display the value
			displayValue := value
			if !showValues && sensitive {
				displayValue = maskValue(value)
			}

			fmt.Printf(" = %s\n", displayValue)
		}

		fmt.Println()

		// Show helpful hints
		if !showValues {
			color.New(color.Faint).Printf("Tip: Use 'doku env %s --show-values' to see actual values\n", serviceName)
		}
		color.New(color.Faint).Printf("Tip: Use 'doku env edit %s' to edit environment variables\n", serviceName)
		fmt.Println()
	}
}
