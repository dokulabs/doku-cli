package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ExportConfig represents the exported configuration
type ExportConfig struct {
	Version     string                    `json:"version" yaml:"version"`
	ExportedAt  time.Time                 `json:"exported_at" yaml:"exported_at"`
	Preferences *types.PreferencesConfig  `json:"preferences,omitempty" yaml:"preferences,omitempty"`
	Network     *types.NetworkGlobalConfig `json:"network,omitempty" yaml:"network,omitempty"`
	Traefik     *types.TraefikGlobalConfig `json:"traefik,omitempty" yaml:"traefik,omitempty"`
	Monitoring  *types.MonitoringConfig   `json:"monitoring,omitempty" yaml:"monitoring,omitempty"`
	Instances   map[string]*ExportInstance `json:"instances,omitempty" yaml:"instances,omitempty"`
	Projects    map[string]*ExportProject `json:"projects,omitempty" yaml:"projects,omitempty"`
}

// ExportInstance represents an exported service instance (without sensitive data)
type ExportInstance struct {
	ServiceType  string            `json:"service_type" yaml:"service_type"`
	Version      string            `json:"version" yaml:"version"`
	Environment  map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes      map[string]string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Network      types.NetworkConfig `json:"network" yaml:"network"`
	Resources    types.ResourceConfig `json:"resources,omitempty" yaml:"resources,omitempty"`
	Traefik      types.TraefikInstanceConfig `json:"traefik" yaml:"traefik"`
	Dependencies []string          `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// ExportProject represents an exported project configuration
type ExportProject struct {
	Path         string            `json:"path" yaml:"path"`
	Dockerfile   string            `json:"dockerfile" yaml:"dockerfile"`
	Port         int               `json:"port" yaml:"port"`
	Environment  map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

var (
	exportOutput      string
	exportFormat      string
	exportIncludeEnv  bool
	exportServicesOnly bool
)

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export Doku configuration to a file",
	Long: `Export the current Doku configuration to a file.

The exported file can be used to:
  - Backup configuration settings
  - Share configuration with team members
  - Migrate configuration to another machine
  - Version control your infrastructure setup

Supported formats: json, yaml

Examples:
  doku config export                      # Export to stdout (YAML)
  doku config export -o config.yaml       # Export to file
  doku config export --format json        # Export as JSON
  doku config export --include-env        # Include environment variables`,
	RunE: runConfigExport,
}

func init() {
	configCmd.AddCommand(configExportCmd)

	configExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
	configExportCmd.Flags().StringVarP(&exportFormat, "format", "f", "yaml", "Output format (json, yaml)")
	configExportCmd.Flags().BoolVar(&exportIncludeEnv, "include-env", false, "Include environment variables (may contain secrets)")
	configExportCmd.Flags().BoolVar(&exportServicesOnly, "services-only", false, "Export only service instances")
}

func runConfigExport(cmd *cobra.Command, args []string) error {
	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Get current config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Build export config
	exportCfg := &ExportConfig{
		Version:    "1.0",
		ExportedAt: time.Now(),
	}

	if !exportServicesOnly {
		exportCfg.Preferences = &cfg.Preferences
		exportCfg.Network = &cfg.Network
		exportCfg.Traefik = &cfg.Traefik
		exportCfg.Monitoring = &cfg.Monitoring
	}

	// Export instances
	if len(cfg.Instances) > 0 {
		exportCfg.Instances = make(map[string]*ExportInstance)
		for name, instance := range cfg.Instances {
			exportInst := &ExportInstance{
				ServiceType:  instance.ServiceType,
				Version:      instance.Version,
				Network:      instance.Network,
				Resources:    instance.Resources,
				Traefik:      instance.Traefik,
				Volumes:      instance.Volumes,
				Dependencies: instance.Dependencies,
			}

			if exportIncludeEnv {
				exportInst.Environment = instance.Environment
			}

			exportCfg.Instances[name] = exportInst
		}
	}

	// Export projects
	if !exportServicesOnly && len(cfg.Projects) > 0 {
		exportCfg.Projects = make(map[string]*ExportProject)
		for name, project := range cfg.Projects {
			exportProj := &ExportProject{
				Path:         project.Path,
				Dockerfile:   project.Dockerfile,
				Port:         project.Port,
				Dependencies: project.Dependencies,
			}

			if exportIncludeEnv {
				exportProj.Environment = project.Environment
			}

			exportCfg.Projects[name] = exportProj
		}
	}

	// Marshal to output format
	var output []byte
	switch exportFormat {
	case "json":
		output, err = json.MarshalIndent(exportCfg, "", "  ")
	case "yaml", "yml":
		output, err = yaml.Marshal(exportCfg)
	default:
		return fmt.Errorf("unsupported format: %s (use json or yaml)", exportFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write output
	if exportOutput == "" {
		// Write to stdout
		fmt.Println(string(output))
	} else {
		// Ensure directory exists
		dir := filepath.Dir(exportOutput)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		if err := os.WriteFile(exportOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		color.Green("Configuration exported to %s", exportOutput)

		// Show summary
		fmt.Println()
		fmt.Printf("Exported:\n")
		if exportCfg.Instances != nil {
			fmt.Printf("  • %d service instances\n", len(exportCfg.Instances))
		}
		if exportCfg.Projects != nil {
			fmt.Printf("  • %d projects\n", len(exportCfg.Projects))
		}
		if !exportServicesOnly {
			fmt.Printf("  • Global preferences and settings\n")
		}

		if !exportIncludeEnv {
			fmt.Println()
			color.New(color.Faint).Println("Note: Environment variables were not included.")
			color.New(color.Faint).Println("Use --include-env to include them (may contain secrets).")
		}
		fmt.Println()
	}

	return nil
}
