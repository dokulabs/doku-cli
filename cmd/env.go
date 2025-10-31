package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	envRaw    bool
	envExport bool
	envJSON   bool
)

var envCmd = &cobra.Command{
	Use:   "env <service>",
	Short: "Show environment variables for a service",
	Long: `Display the environment variables configured for a service instance.

By default, sensitive values (passwords, tokens, etc.) are masked for security.
Use --raw to show actual values.

Output formats:
  Default: Key-value pairs (masked)
  --raw: Key-value pairs (unmasked)
  --export: Shell export format (for sourcing)
  --json: JSON format

Examples:
  doku env postgres-main              # Show env vars (masked)
  doku env postgres-main --raw        # Show actual values
  doku env postgres-main --export     # Shell export format
  doku env postgres-main --json       # JSON output`,
	Args: cobra.ExactArgs(1),
	RunE: runEnv,
}

func init() {
	rootCmd.AddCommand(envCmd)

	envCmd.Flags().BoolVar(&envRaw, "raw", false, "Show raw values without masking")
	envCmd.Flags().BoolVar(&envExport, "export", false, "Output in shell export format")
	envCmd.Flags().BoolVar(&envJSON, "json", false, "Output in JSON format")
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

	// Special handling for Traefik
	if instanceName == "traefik" || instanceName == "doku-traefik" {
		containerName := "doku-traefik"

		// Check if exists
		exists, err := dockerClient.ContainerExists(containerName)
		if err != nil || !exists {
			return fmt.Errorf("Traefik container not found. Run 'doku init' first")
		}

		color.Yellow("⚠️  Traefik is a system component and doesn't have configurable environment variables")
		fmt.Println()
		color.New(color.Faint).Println("Traefik is configured through:")
		color.New(color.Faint).Println("  • Traefik configuration files (static and dynamic)")
		color.New(color.Faint).Println("  • Docker labels on service containers")
		fmt.Println()
		color.New(color.Faint).Println("Use 'doku info traefik' to see Traefik configuration details")
		return nil
	}

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	// Check if service has environment variables
	if len(instance.Environment) == 0 {
		color.Yellow("No environment variables configured for '%s'", instanceName)
		return nil
	}

	// Output based on format
	if envJSON {
		return outputJSON(instance.Environment, envRaw)
	}

	if envExport {
		return outputExport(instance.Environment, envRaw)
	}

	return outputDefault(instanceName, instance.Environment, envRaw)
}

func outputDefault(instanceName string, env map[string]string, raw bool) error {
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Printf("Environment Variables: %s\n", instanceName)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Sort keys for consistent output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Display each variable
	for _, key := range keys {
		value := env[key]

		// Mask sensitive values unless --raw flag is set
		if !raw && isSensitiveKey(key) {
			value = maskValue(value)
			fmt.Printf("  %s=%s %s\n",
				color.YellowString(key),
				value,
				color.New(color.Faint).Sprint("(masked)"))
		} else {
			fmt.Printf("  %s=%s\n", color.YellowString(key), value)
		}
	}

	fmt.Println()

	// Show helpful tip if values are masked
	if !raw && hasSensitiveKeys(env) {
		color.New(color.Faint).Println("Use --raw to show actual values")
	}

	return nil
}

func outputExport(env map[string]string, raw bool) error {
	// Sort keys for consistent output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := env[key]

		// Mask sensitive values unless --raw flag is set
		if !raw && isSensitiveKey(key) {
			value = maskValue(value)
		}

		// Escape value for shell
		escapedValue := strings.ReplaceAll(value, `"`, `\"`)
		fmt.Printf("export %s=\"%s\"\n", key, escapedValue)
	}

	return nil
}

func outputJSON(env map[string]string, raw bool) error {
	output := make(map[string]string)

	for key, value := range env {
		// Mask sensitive values unless --raw flag is set
		if !raw && isSensitiveKey(key) {
			value = maskValue(value)
		}
		output[key] = value
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func hasSensitiveKeys(env map[string]string) bool {
	for key := range env {
		if isSensitiveKey(key) {
			return true
		}
	}
	return false
}
