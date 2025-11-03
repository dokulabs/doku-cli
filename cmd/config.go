package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Doku configuration",
	Long: `View and modify Doku configuration settings.

Configuration is stored in ~/.doku/config.yaml

Examples:
  doku config list                          # List all configuration
  doku config get monitoring.tool           # Get specific value
  doku config set monitoring.dsn <value>    # Set specific value`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value by key.

Use dot notation to access nested values.

Examples:
  doku config get monitoring.tool
  doku config get monitoring.url
  doku config get preferences.domain`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value by key.

Use dot notation to access nested values.

Examples:
  doku config set monitoring.dsn https://...
  doku config set monitoring.enabled true
  doku config set preferences.domain mydomain.local`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration",
	Long:  `Display all configuration settings in YAML format.`,
	RunE:  runConfigList,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

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

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Get value by key
	value, err := getConfigValue(cfg, key)
	if err != nil {
		return err
	}

	// Print value
	fmt.Println(value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

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

	// Set value by key
	if err := setConfigValue(cfgMgr, key, value); err != nil {
		return err
	}

	color.Green("✓ Configuration updated: %s = %s", key, value)
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
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

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Convert to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Print YAML
	fmt.Println(string(yamlData))
	return nil
}

// getConfigValue retrieves a value from config using dot notation
func getConfigValue(cfg interface{}, key string) (string, error) {
	parts := strings.Split(key, ".")

	// Navigate through nested structure
	current := cfg
	for i, part := range parts {
		// Convert to map
		m, ok := current.(map[string]interface{})
		if !ok {
			// Try to convert from struct
			data, err := yaml.Marshal(current)
			if err != nil {
				return "", fmt.Errorf("invalid key path at '%s'", strings.Join(parts[:i], "."))
			}
			if err := yaml.Unmarshal(data, &m); err != nil {
				return "", fmt.Errorf("invalid key path at '%s'", strings.Join(parts[:i], "."))
			}
		}

		// Get next value
		val, exists := m[part]
		if !exists {
			return "", fmt.Errorf("key not found: %s", key)
		}

		// If last part, return value
		if i == len(parts)-1 {
			return fmt.Sprintf("%v", val), nil
		}

		current = val
	}

	return "", fmt.Errorf("key not found: %s", key)
}

// setConfigValue sets a value in config using dot notation
func setConfigValue(cfgMgr *config.Manager, key, value string) error {
	parts := strings.Split(key, ".")

	// Special handling for known keys
	switch key {
	case "monitoring.tool":
		return cfgMgr.SetMonitoringTool(value)
	case "monitoring.url":
		return cfgMgr.SetMonitoringURL(value)
	case "monitoring.dsn":
		return cfgMgr.SetMonitoringDSN(value)
	case "monitoring.enabled":
		enabled := value == "true" || value == "1" || value == "yes"
		return cfgMgr.Update(func(c *types.Config) error {
			c.Monitoring.Enabled = enabled
			return nil
		})
	case "preferences.domain":
		return cfgMgr.Update(func(c *types.Config) error {
			c.Preferences.Domain = value
			return nil
		})
	case "preferences.protocol":
		if value != "http" && value != "https" {
			return fmt.Errorf("protocol must be 'http' or 'https'")
		}
		return cfgMgr.Update(func(c *types.Config) error {
			c.Preferences.Protocol = value
			return nil
		})
	default:
		// Generic nested key setting
		return cfgMgr.Update(func(c *types.Config) error {
			return setNestedValue(c, parts, value)
		})
	}
}

// setNestedValue sets a nested value in a struct
func setNestedValue(obj interface{}, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty key path")
	}

	// Convert to map for manipulation
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	// Navigate to parent
	current := m
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, exists := current[part]
		if !exists {
			// Create nested map
			current[part] = make(map[string]interface{})
			next = current[part]
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("cannot set nested value at '%s': not a map", strings.Join(parts[:i+1], "."))
		}
		current = nextMap
	}

	// Set final value
	lastPart := parts[len(parts)-1]
	current[lastPart] = value

	// Convert back to struct
	data, err = yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}

	return yaml.Unmarshal(data, obj)
}
