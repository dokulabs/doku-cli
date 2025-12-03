package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/backup"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	serviceUpgradeVersion string
	serviceUpgradeYes     bool
	serviceUpgradeBackup  bool
)

var serviceUpgradeCmd = &cobra.Command{
	Use:   "upgrade <service>",
	Short: "Upgrade an installed service to a newer version",
	Long: `Upgrade an installed service to a newer version.

This will:
  1. Check for available versions
  2. Create a backup (optional)
  3. Stop the current service
  4. Pull the new image
  5. Recreate the container with the new version
  6. Preserve volumes and environment

Examples:
  doku service upgrade postgres                 # Upgrade to latest version
  doku service upgrade postgres --version 16    # Upgrade to specific version
  doku service upgrade postgres --backup        # Create backup before upgrade
  doku service upgrade postgres --yes           # Skip confirmation`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"up"},
	RunE:    runServiceUpgrade,
}

func init() {
	serviceCmd.AddCommand(serviceUpgradeCmd)

	serviceUpgradeCmd.Flags().StringVarP(&serviceUpgradeVersion, "version", "v", "", "Target version to upgrade to")
	serviceUpgradeCmd.Flags().BoolVarP(&serviceUpgradeYes, "yes", "y", false, "Skip confirmation prompt")
	serviceUpgradeCmd.Flags().BoolVarP(&serviceUpgradeBackup, "backup", "b", false, "Create backup before upgrade")
}

func runServiceUpgrade(cmd *cobra.Command, args []string) error {
	instanceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
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

	// Get current instance
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found", instanceName)
	}

	// Check if it's a catalog service (not custom project)
	if instance.ServiceType == "custom-project" {
		return fmt.Errorf("upgrade is not supported for custom projects. Use 'doku deploy' instead")
	}

	// Create catalog manager
	catalogMgr := catalog.NewManager(cfgMgr.GetDokuDir())

	// Get available versions
	svc, err := catalogMgr.GetService(instance.ServiceType)
	if err != nil {
		return fmt.Errorf("service '%s' not found in catalog", instance.ServiceType)
	}

	// Get version list
	var versions []string
	for version := range svc.Versions {
		versions = append(versions, version)
	}

	if len(versions) == 0 {
		return fmt.Errorf("no versions available for %s", instance.ServiceType)
	}

	// Determine target version
	targetVersion := serviceUpgradeVersion
	if targetVersion == "" {
		// Find the latest version (simple heuristic - use "latest" or first available)
		if _, ok := svc.Versions["latest"]; ok {
			targetVersion = "latest"
		} else {
			targetVersion = versions[0]
		}
	}

	// Validate target version exists
	if _, ok := svc.Versions[targetVersion]; !ok {
		return fmt.Errorf("version '%s' not found. Available versions: %s", targetVersion, strings.Join(versions, ", "))
	}

	// Check if already on this version
	if instance.Version == targetVersion {
		color.Yellow("Service is already running version '%s'", targetVersion)
		return nil
	}

	// Show upgrade plan
	fmt.Println()
	color.Cyan("Upgrade Plan for '%s'", instanceName)
	fmt.Println()
	fmt.Printf("  Current version: %s\n", color.YellowString(instance.Version))
	fmt.Printf("  Target version:  %s\n", color.GreenString(targetVersion))
	fmt.Println()

	if len(versions) > 1 {
		color.New(color.Faint).Printf("Available versions: %s\n", strings.Join(versions, ", "))
		fmt.Println()
	}

	// Confirmation
	if !serviceUpgradeYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with upgrade?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Upgrade cancelled")
			return nil
		}
		fmt.Println()
	}

	// Create backup if requested
	if serviceUpgradeBackup {
		color.Cyan("Creating backup before upgrade...")
		backupMgr := backup.NewManager(dockerClient, cfgMgr)
		backupOpts := backup.BackupOptions{
			InstanceName:   instanceName,
			IncludeVolumes: true,
			IncludeEnv:     true,
			Compress:       true,
		}
		if _, err := backupMgr.Backup(backupOpts); err != nil {
			color.Yellow("Warning: Backup failed: %v", err)

			// Ask if user wants to continue
			if !serviceUpgradeYes {
				continueAnyway := false
				prompt := &survey.Confirm{
					Message: "Backup failed. Continue with upgrade anyway?",
					Default: false,
				}
				if err := survey.AskOne(prompt, &continueAnyway); err != nil {
					return err
				}
				if !continueAnyway {
					return fmt.Errorf("upgrade cancelled")
				}
			}
		} else {
			color.Green("✓ Backup created")
		}
		fmt.Println()
	}

	// Get target spec
	targetSpec, err := catalogMgr.GetServiceVersion(instance.ServiceType, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to get target version spec: %w", err)
	}

	// Step 1: Pull new image
	color.Cyan("Pulling new image: %s", targetSpec.Image)
	if err := dockerClient.ImagePull(targetSpec.Image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	color.Green("✓ Image pulled")
	fmt.Println()

	// Step 2: Stop current container
	color.Cyan("Stopping current container...")
	if err := serviceMgr.Stop(instanceName); err != nil {
		color.Yellow("Warning: Failed to stop container: %v", err)
	} else {
		color.Green("✓ Container stopped")
	}
	fmt.Println()

	// Step 3: Recreate with new version
	color.Cyan("Recreating container with new version...")

	// Recreate the container with the new image
	if err := serviceMgr.RecreateWithImage(instanceName, targetSpec.Image); err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	// Update instance version in config
	instance.Version = targetVersion
	if err := cfgMgr.UpdateInstance(instanceName, instance); err != nil {
		color.Yellow("Warning: Failed to update config: %v", err)
	}

	color.Green("✓ Container recreated")
	fmt.Println()

	// Success message
	color.Green("Upgrade complete!")
	fmt.Println()
	fmt.Printf("  %s upgraded to version %s\n",
		color.CyanString(instanceName),
		color.GreenString(targetVersion))
	fmt.Println()

	color.New(color.Faint).Println("Verify the upgrade:")
	color.New(color.Faint).Printf("  doku health %s\n", instanceName)
	color.New(color.Faint).Printf("  doku logs %s\n", instanceName)
	fmt.Println()

	return nil
}
