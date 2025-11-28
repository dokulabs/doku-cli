package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	updateYes     bool
	updateVersion string
	updateAll     bool
)

var updateCmd = &cobra.Command{
	Use:   "update [service-name]",
	Short: "Update a service to a newer version",
	Long: `Update a service to a newer version while preserving data volumes.

Examples:
  doku update postgres              # Update postgres to latest version
  doku update postgres --version 16 # Update postgres to version 16
  doku update postgres -y           # Update without confirmation
  doku update --all                 # Update all services to latest versions`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Skip confirmation prompts")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Target version to update to")
	updateCmd.Flags().BoolVar(&updateAll, "all", false, "Update all services")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	// Validate arguments
	if !updateAll && len(args) == 0 {
		return fmt.Errorf("specify a service name or use --all to update all services")
	}

	if updateAll && len(args) > 0 {
		return fmt.Errorf("cannot specify both --all and a service name")
	}

	// Create managers
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if initialized
	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	if updateAll {
		return updateAllServices(cfgMgr, catalogMgr, serviceMgr, dockerClient)
	}

	return updateSingleService(args[0], cfgMgr, catalogMgr, serviceMgr, dockerClient)
}

func updateSingleService(instanceName string, cfgMgr *config.Manager, catalogMgr *catalog.Manager, serviceMgr *service.Manager, dockerClient *docker.Client) error {
	// Get existing instance
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found: %w", instanceName, err)
	}

	// Multi-container services are not supported yet
	if instance.IsMultiContainer {
		return fmt.Errorf("updating multi-container services is not yet supported")
	}

	// Get service from catalog
	catalogService, err := catalogMgr.GetService(instance.ServiceType)
	if err != nil {
		return fmt.Errorf("service '%s' not found in catalog", instance.ServiceType)
	}

	// Determine target version
	targetVersion := updateVersion
	if targetVersion == "" {
		// Get latest version from catalog
		for v := range catalogService.Versions {
			if targetVersion == "" || v > targetVersion {
				targetVersion = v
			}
		}
	}

	// Get service spec for target version
	targetSpec, err := catalogMgr.GetServiceVersion(instance.ServiceType, targetVersion)
	if err != nil {
		return fmt.Errorf("version '%s' not found for service '%s'", targetVersion, instance.ServiceType)
	}

	// Check if already on target version
	if instance.Version == targetVersion {
		color.Green("Service '%s' is already on version %s", instanceName, targetVersion)
		return nil
	}

	// Display update information
	fmt.Println()
	color.Cyan("Update: %s %s", catalogService.Icon, catalogService.Name)
	fmt.Println()
	fmt.Printf("Instance:        %s\n", instanceName)
	fmt.Printf("Current version: %s\n", instance.Version)
	fmt.Printf("Target version:  %s\n", color.GreenString(targetVersion))
	fmt.Printf("New image:       %s\n", targetSpec.Image)
	fmt.Println()

	// Confirm update
	if !updateYes {
		fmt.Println("This will:")
		fmt.Println("  1. Stop the current container")
		fmt.Println("  2. Pull the new image")
		fmt.Println("  3. Create a new container with the same configuration")
		fmt.Println("  4. Start the new container")
		color.New(color.FgGreen).Println("  * Data volumes will be preserved")
		fmt.Println()

		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with update?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Update cancelled")
			return nil
		}
		fmt.Println()
	}

	// Step 1: Stop current container
	color.Cyan("Step 1/4: Stopping current container...")
	if err := serviceMgr.Stop(instanceName); err != nil {
		// Ignore error if container is already stopped
		if !strings.Contains(err.Error(), "not running") {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}
	color.Green("Container stopped")

	// Step 2: Pull new image
	fmt.Println()
	color.Cyan("Step 2/4: Pulling new image...")
	fmt.Printf("Pulling %s...\n", targetSpec.Image)
	if err := dockerClient.ImagePull(targetSpec.Image); err != nil {
		// Try to restart the old container if pull fails
		_ = serviceMgr.Start(instanceName)
		return fmt.Errorf("failed to pull image: %w", err)
	}
	color.Green("Image pulled")

	// Step 3: Recreate container
	fmt.Println()
	color.Cyan("Step 3/4: Recreating container...")

	// Create installer for updating
	installer, err := service.NewInstaller(dockerClient, cfgMgr, catalogMgr)
	if err != nil {
		// Try to restart the old container if installer creation fails
		_ = serviceMgr.Start(instanceName)
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Remove old container (preserve volumes)
	if err := serviceMgr.Remove(instanceName, true, false); err != nil {
		return fmt.Errorf("failed to remove old container: %w", err)
	}

	// Reinstall with new version
	opts := service.InstallOptions{
		ServiceName:  instance.ServiceType,
		Version:      targetVersion,
		InstanceName: instanceName,
		Environment:  instance.Environment,
		MemoryLimit:  instance.Resources.MemoryLimit,
		CPULimit:     instance.Resources.CPULimit,
		Volumes:      instance.Volumes,
		PortMappings: instance.Network.PortMappings,
		Internal:     !instance.Traefik.Enabled,
		Replace:      true,
	}

	newInstance, err := installer.Install(opts)
	if err != nil {
		return fmt.Errorf("failed to install new version: %w", err)
	}
	color.Green("Container recreated")

	// Step 4: Start new container (already done by Install)
	fmt.Println()
	color.Cyan("Step 4/4: Starting new container...")
	color.Green("Container started")

	// Success message
	fmt.Println()
	color.Green("Successfully updated %s from %s to %s", instanceName, instance.Version, targetVersion)
	fmt.Println()

	// Show connection information
	if newInstance.URL != "" {
		color.Cyan("Access your service:")
		fmt.Printf("  URL: %s\n", newInstance.URL)
	}
	fmt.Println()

	return nil
}

func updateAllServices(cfgMgr *config.Manager, catalogMgr *catalog.Manager, serviceMgr *service.Manager, dockerClient *docker.Client) error {
	// Get all instances
	instances, err := serviceMgr.List()
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if len(instances) == 0 {
		color.Yellow("No services installed")
		return nil
	}

	// Find services with available updates
	type updateInfo struct {
		instance       string
		serviceType    string
		currentVer     string
		latestVer      string
		multiContainer bool
	}

	var updates []updateInfo

	for _, instance := range instances {
		// Skip multi-container services
		if instance.IsMultiContainer {
			continue
		}

		catalogService, err := catalogMgr.GetService(instance.ServiceType)
		if err != nil {
			continue // Skip if service not in catalog
		}

		// Find latest version
		var latestVersion string
		for v := range catalogService.Versions {
			if latestVersion == "" || v > latestVersion {
				latestVersion = v
			}
		}

		if latestVersion != "" && instance.Version != latestVersion {
			updates = append(updates, updateInfo{
				instance:    instance.Name,
				serviceType: instance.ServiceType,
				currentVer:  instance.Version,
				latestVer:   latestVersion,
			})
		}
	}

	if len(updates) == 0 {
		color.Green("All services are up to date!")
		return nil
	}

	// Display available updates
	fmt.Println()
	color.Cyan("Available updates:")
	fmt.Println()
	for _, u := range updates {
		fmt.Printf("  %s: %s -> %s\n", u.instance, u.currentVer, color.GreenString(u.latestVer))
	}
	fmt.Println()

	// Confirm updates
	if !updateYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Update %d service(s)?", len(updates)),
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Update cancelled")
			return nil
		}
		fmt.Println()
	}

	// Update each service
	successCount := 0
	failCount := 0

	for _, u := range updates {
		fmt.Println()
		color.Cyan("Updating %s...", u.instance)

		updateVersion = u.latestVer
		if err := updateSingleService(u.instance, cfgMgr, catalogMgr, serviceMgr, dockerClient); err != nil {
			color.Red("Failed to update %s: %v", u.instance, err)
			failCount++
		} else {
			successCount++
		}
	}

	// Summary
	fmt.Println()
	if failCount == 0 {
		color.Green("Successfully updated %d service(s)", successCount)
	} else {
		color.Yellow("Updated %d service(s), %d failed", successCount, failCount)
	}

	return nil
}
