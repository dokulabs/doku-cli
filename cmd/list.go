package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listAll     bool
	listService string
	listVerbose bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all installed services",
	Long:    "List all installed services with their status, versions, and access URLs",
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "Show all instances including stopped")
	listCmd.Flags().StringVarP(&listService, "service", "s", "", "Filter by service type")
	listCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed information")
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Get all instances
	instances, err := serviceMgr.List()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// Filter instances
	filteredInstances := filterInstances(instances, listService, listAll)

	if len(filteredInstances) == 0 {
		fmt.Println()
		if listService != "" {
			color.Yellow("No services found matching '%s'", listService)
		} else if !listAll {
			color.Yellow("No running services found")
			fmt.Println("\nUse 'doku list --all' to see stopped services")
		} else {
			color.Yellow("No services installed")
			fmt.Println("\nInstall services with: doku install <service>")
		}
		fmt.Println()
		return nil
	}

	// Get config for domain
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Update instance statuses from Docker
	ctx := context.Background()
	for _, instance := range filteredInstances {
		updateInstanceStatus(ctx, dockerClient, instance)
	}

	// Display instances
	displayInstances(filteredInstances, cfg.Preferences.Protocol, cfg.Preferences.Domain, listVerbose)

	return nil
}

func filterInstances(instances []*types.Instance, serviceFilter string, showAll bool) []*types.Instance {
	filtered := make([]*types.Instance, 0)

	for _, instance := range instances {
		// Filter by service type
		if serviceFilter != "" && !strings.EqualFold(instance.ServiceType, serviceFilter) {
			continue
		}

		// Filter by status (if not showing all)
		if !showAll && instance.Status != types.StatusRunning {
			continue
		}

		filtered = append(filtered, instance)
	}

	return filtered
}

func updateInstanceStatus(ctx context.Context, dockerClient *docker.Client, instance *types.Instance) {
	// Try to inspect the container
	containerInfo, err := dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		instance.Status = types.StatusUnknown
		return
	}

	// Update status based on container state
	if containerInfo.State.Running {
		instance.Status = types.StatusRunning
	} else if containerInfo.State.Dead || containerInfo.State.Status == "exited" {
		if containerInfo.State.ExitCode != 0 {
			instance.Status = types.StatusFailed
		} else {
			instance.Status = types.StatusStopped
		}
	} else {
		instance.Status = types.StatusStopped
	}

	// Get resource usage if running
	if instance.Status == types.StatusRunning {
		updateResourceUsage(ctx, dockerClient, instance, &containerInfo)
	}
}

func updateResourceUsage(ctx context.Context, dockerClient *docker.Client, instance *types.Instance, containerInfo *dockerTypes.ContainerJSON) {
	// Get container stats (non-streaming)
	_, err := dockerClient.ContainerStats(instance.ContainerName)
	if err != nil {
		return
	}
	// Note: Stats response should be read and closed properly in production

	// Parse stats for memory usage
	// Note: This is a simplified version, real implementation would need proper JSON parsing
	// For now, we'll use the container inspect data
	if containerInfo.State.Running {
		instance.Resources.MemoryUsage = "N/A" // Would need stats parsing
		instance.Resources.CPUUsage = "N/A"    // Would need stats parsing
	}
}

func displayInstances(instances []*types.Instance, protocol, domain string, verbose bool) {
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Println("📋 Installed Services")
	fmt.Println()

	for i, instance := range instances {
		if i > 0 {
			fmt.Println()
		}

		displayInstance(instance, protocol, domain, verbose)
	}

	fmt.Println()
	color.Cyan("Total: %d service(s)", len(instances))
	fmt.Println()
}

func displayInstance(instance *types.Instance, protocol, domain string, verbose bool) {
	// Status indicator
	statusColor := getStatusColor(instance.Status)
	statusIcon := getStatusIcon(instance.Status)

	// Header line with name and status
	fmt.Printf("%s ", statusIcon)
	color.New(color.Bold, color.FgWhite).Printf("%s", instance.Name)
	fmt.Printf(" ")
	statusColor(" [%s]", string(instance.Status))
	fmt.Println()

	// Service type and version
	fmt.Printf("  Service: %s", color.CyanString(instance.ServiceType))
	if instance.Version != "" {
		fmt.Printf(" (v%s)", instance.Version)
	}
	fmt.Println()

	// URL (if Traefik enabled)
	if instance.Traefik.Enabled && instance.URL != "" {
		fmt.Printf("  URL: %s\n", color.GreenString(instance.URL))
	}

	// Connection string (if available)
	if instance.ConnectionString != "" && verbose {
		fmt.Printf("  Connection: %s\n", instance.ConnectionString)
	}

	// Resources
	if verbose {
		if instance.Resources.MemoryLimit != "" {
			fmt.Printf("  Memory: %s", instance.Resources.MemoryLimit)
			if instance.Resources.MemoryUsage != "" && instance.Resources.MemoryUsage != "N/A" {
				fmt.Printf(" (using %s)", instance.Resources.MemoryUsage)
			}
			fmt.Println()
		}

		if instance.Resources.CPULimit != "" {
			fmt.Printf("  CPU: %s", instance.Resources.CPULimit)
			if instance.Resources.CPUUsage != "" && instance.Resources.CPUUsage != "N/A" {
				fmt.Printf(" (using %s)", instance.Resources.CPUUsage)
			}
			fmt.Println()
		}
	}

	// Container name
	if verbose {
		fmt.Printf("  Container: %s\n", color.New(color.Faint).Sprint(instance.ContainerName))
	}

	// Created time
	if verbose {
		fmt.Printf("  Created: %s\n", formatTime(instance.CreatedAt))
	}

	// Access instructions (if running)
	if instance.Status == types.StatusRunning && !verbose {
		if instance.Traefik.Enabled {
			fmt.Printf("  Access: %s\n", color.New(color.Faint).Sprintf("Open %s in your browser", instance.URL))
		} else if instance.Network.InternalPort > 0 {
			fmt.Printf("  Access: %s\n", color.New(color.Faint).Sprintf("Internal only (port %d)", instance.Network.InternalPort))
		}
	}
}

func getStatusColor(status types.ServiceStatus) func(format string, a ...interface{}) {
	switch status {
	case types.StatusRunning:
		return color.Green
	case types.StatusStopped:
		return color.Yellow
	case types.StatusFailed:
		return color.Red
	default:
		return func(format string, a ...interface{}) {
			color.New(color.Faint).Printf(format, a...)
		}
	}
}

func getStatusIcon(status types.ServiceStatus) string {
	switch status {
	case types.StatusRunning:
		return color.GreenString("●")
	case types.StatusStopped:
		return color.YellowString("○")
	case types.StatusFailed:
		return color.RedString("✗")
	default:
		return color.New(color.Faint).Sprint("?")
	}
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d minute(s) ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hour(s) ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day(s) ago", days)
	}
}
