package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	importFile      string
	importOverwrite bool
	importDryRun    bool
	importYes       bool
)

var configImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import Doku configuration from a file",
	Long: `Import Doku configuration from an exported file.

This will:
  - Read the exported configuration
  - Merge with existing configuration (or overwrite with --overwrite)
  - Update service and project settings

Note: This does not recreate containers. Use 'doku install' or 'doku deploy'
to create services based on the imported configuration.

Examples:
  doku config import config.yaml           # Import from YAML file
  doku config import config.json           # Import from JSON file
  doku config import config.yaml --dry-run # Preview changes without applying
  doku config import config.yaml --overwrite # Overwrite existing config`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigImport,
}

func init() {
	configCmd.AddCommand(configImportCmd)

	configImportCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "Overwrite existing configuration completely")
	configImportCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview changes without applying")
	configImportCmd.Flags().BoolVarP(&importYes, "yes", "y", false, "Skip confirmation prompt")
}

func runConfigImport(cmd *cobra.Command, args []string) error {
	importFile = args[0]

	// Read input file
	data, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the file
	var importCfg ExportConfig
	ext := strings.ToLower(filepath.Ext(importFile))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &importCfg); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &importCfg); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &importCfg); err != nil {
			if err := json.Unmarshal(data, &importCfg); err != nil {
				return fmt.Errorf("failed to parse file (tried YAML and JSON)")
			}
		}
	}

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

	// Show what will be imported
	fmt.Println()
	color.Cyan("Import Preview")
	fmt.Println()

	fmt.Printf("Source: %s\n", importFile)
	fmt.Printf("Exported at: %s\n", importCfg.ExportedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Count changes
	newInstances := 0
	updateInstances := 0
	newProjects := 0
	updateProjects := 0

	if importCfg.Instances != nil {
		fmt.Println("Service Instances:")
		for name, inst := range importCfg.Instances {
			_, exists := cfg.Instances[name]
			if exists {
				updateInstances++
				fmt.Printf("  • %s (%s) - %s\n", name, inst.ServiceType, color.YellowString("update"))
			} else {
				newInstances++
				fmt.Printf("  • %s (%s) - %s\n", name, inst.ServiceType, color.GreenString("new"))
			}
		}
		fmt.Println()
	}

	if importCfg.Projects != nil {
		fmt.Println("Projects:")
		for name, proj := range importCfg.Projects {
			_, exists := cfg.Projects[name]
			if exists {
				updateProjects++
				fmt.Printf("  • %s (%s) - %s\n", name, proj.Path, color.YellowString("update"))
			} else {
				newProjects++
				fmt.Printf("  • %s (%s) - %s\n", name, proj.Path, color.GreenString("new"))
			}
		}
		fmt.Println()
	}

	// Global settings
	globalChanges := false
	if importCfg.Preferences != nil || importCfg.Network != nil || importCfg.Monitoring != nil {
		globalChanges = true
		fmt.Println("Global Settings:")
		if importCfg.Preferences != nil {
			fmt.Printf("  • Preferences (domain: %s)\n", importCfg.Preferences.Domain)
		}
		if importCfg.Network != nil {
			fmt.Printf("  • Network (%s)\n", importCfg.Network.Name)
		}
		if importCfg.Monitoring != nil && importCfg.Monitoring.Tool != "" {
			fmt.Printf("  • Monitoring (%s)\n", importCfg.Monitoring.Tool)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  New instances: %d, Updates: %d\n", newInstances, updateInstances)
	fmt.Printf("  New projects: %d, Updates: %d\n", newProjects, updateProjects)
	if globalChanges {
		fmt.Printf("  Global settings will be updated\n")
	}
	fmt.Println()

	if importDryRun {
		color.Cyan("Dry run complete. No changes were made.")
		return nil
	}

	// Confirmation
	if !importYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Apply these changes?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Import cancelled")
			return nil
		}
		fmt.Println()
	}

	// Apply changes
	color.Cyan("Importing configuration...")
	fmt.Println()

	// Update global settings if provided
	if importOverwrite {
		if importCfg.Preferences != nil {
			cfg.Preferences = *importCfg.Preferences
		}
		if importCfg.Network != nil {
			cfg.Network = *importCfg.Network
		}
		if importCfg.Monitoring != nil {
			cfg.Monitoring = *importCfg.Monitoring
		}
	} else {
		// Merge only specific fields
		if importCfg.Preferences != nil && importCfg.Preferences.Domain != "" {
			cfg.Preferences.Domain = importCfg.Preferences.Domain
			cfg.Preferences.Protocol = importCfg.Preferences.Protocol
		}
	}

	// Import instances
	if importCfg.Instances != nil {
		if cfg.Instances == nil {
			cfg.Instances = make(map[string]*types.Instance)
		}

		for name, importInst := range importCfg.Instances {
			existing, exists := cfg.Instances[name]

			if exists && !importOverwrite {
				// Merge with existing
				if importInst.Environment != nil {
					if existing.Environment == nil {
						existing.Environment = make(map[string]string)
					}
					for k, v := range importInst.Environment {
						existing.Environment[k] = v
					}
				}
				if importInst.Resources.MemoryLimit != "" {
					existing.Resources.MemoryLimit = importInst.Resources.MemoryLimit
				}
				if importInst.Resources.CPULimit != "" {
					existing.Resources.CPULimit = importInst.Resources.CPULimit
				}
			} else {
				// Create new or overwrite
				instance := &types.Instance{
					Name:         name,
					ServiceType:  importInst.ServiceType,
					Version:      importInst.Version,
					Environment:  importInst.Environment,
					Volumes:      importInst.Volumes,
					Network:      importInst.Network,
					Resources:    importInst.Resources,
					Traefik:      importInst.Traefik,
					Dependencies: importInst.Dependencies,
				}
				cfg.Instances[name] = instance
			}
		}
	}

	// Import projects
	if importCfg.Projects != nil {
		if cfg.Projects == nil {
			cfg.Projects = make(map[string]*types.Project)
		}

		for name, importProj := range importCfg.Projects {
			existing, exists := cfg.Projects[name]

			if exists && !importOverwrite {
				// Merge with existing
				if importProj.Environment != nil {
					if existing.Environment == nil {
						existing.Environment = make(map[string]string)
					}
					for k, v := range importProj.Environment {
						existing.Environment[k] = v
					}
				}
				if importProj.Port != 0 {
					existing.Port = importProj.Port
				}
			} else {
				// Create new or overwrite
				project := &types.Project{
					Name:         name,
					Path:         importProj.Path,
					Dockerfile:   importProj.Dockerfile,
					Port:         importProj.Port,
					Environment:  importProj.Environment,
					Dependencies: importProj.Dependencies,
				}
				cfg.Projects[name] = project
			}
		}
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	color.Green("Configuration imported successfully!")
	fmt.Println()

	// Show next steps
	if newInstances > 0 || updateInstances > 0 {
		color.Cyan("Next steps:")
		fmt.Println("  To install new services:")
		fmt.Println("    doku install <service-type> --name <instance-name>")
		fmt.Println()
		fmt.Println("  To restart updated services:")
		fmt.Println("    doku restart <instance-name>")
		fmt.Println()
	}

	return nil
}
