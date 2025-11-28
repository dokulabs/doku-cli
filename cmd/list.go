package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

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
	listHealth  bool
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
	listCmd.Flags().BoolVar(&listHealth, "health", false, "Show health check status")
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

	// Update instance statuses from Docker in parallel
	ctx := context.Background()
	var wg sync.WaitGroup

	for _, instance := range filteredInstances {
		wg.Add(1)
		go func(inst *types.Instance) {
			defer wg.Done()
			updateInstanceStatus(ctx, dockerClient, inst)
		}(instance)
	}

	// Wait for all status updates to complete
	wg.Wait()

	// Check health status if requested
	if listHealth {
		updateHealthStatus(ctx, dockerClient, filteredInstances)
	}

	// Display instances
	displayInstances(filteredInstances, cfg.Preferences.Protocol, cfg.Preferences.Domain, listVerbose, listHealth)

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
	// Handle multi-container services
	if instance.IsMultiContainer {
		updateMultiContainerStatus(ctx, dockerClient, instance)
		return
	}

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

	// Note: Resource usage (CPU/Memory stats) is not currently displayed in list output
	// The updateResourceUsage function has been removed to improve performance
}

// updateMultiContainerStatus updates status for multi-container services in parallel
func updateMultiContainerStatus(ctx context.Context, dockerClient *docker.Client, instance *types.Instance) {
	runningCount := 0
	stoppedCount := 0
	failedCount := 0

	// Use mutex to safely update counters from goroutines
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range instance.Containers {
		wg.Add(1)
		go func(container *types.ContainerInfo) {
			defer wg.Done()

			containerInfo, err := dockerClient.ContainerInspect(container.ContainerID)
			if err != nil {
				container.Status = "unknown"
				return
			}

			mu.Lock()
			defer mu.Unlock()

			if containerInfo.State.Running {
				container.Status = "running"
				runningCount++
			} else if containerInfo.State.Dead || containerInfo.State.OOMKilled {
				container.Status = "failed"
				failedCount++
			} else {
				container.Status = "stopped"
				stoppedCount++
			}
		}(&instance.Containers[i])
	}

	// Wait for all container inspections to complete
	wg.Wait()

	// Determine overall status
	if failedCount > 0 {
		instance.Status = types.StatusFailed
	} else if runningCount == len(instance.Containers) {
		instance.Status = types.StatusRunning
	} else if stoppedCount == len(instance.Containers) {
		instance.Status = types.StatusStopped
	} else {
		// Partially running
		instance.Status = types.StatusRunning
	}
}

// updateHealthStatus checks health status for all instances
func updateHealthStatus(ctx context.Context, dockerClient *docker.Client, instances []*types.Instance) {
	var wg sync.WaitGroup

	for _, instance := range instances {
		if instance.Status != types.StatusRunning {
			continue
		}

		wg.Add(1)
		go func(inst *types.Instance) {
			defer wg.Done()
			checkInstanceHealth(ctx, dockerClient, inst)
		}(instance)
	}

	wg.Wait()
}

// checkInstanceHealth checks the health of a single instance
func checkInstanceHealth(ctx context.Context, dockerClient *docker.Client, instance *types.Instance) {
	containerInfo, err := dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		instance.HealthStatus = "unknown"
		return
	}

	// Check if container has health check
	if containerInfo.State.Health == nil {
		instance.HealthStatus = "none"
		return
	}

	// Get health status
	switch containerInfo.State.Health.Status {
	case "healthy":
		instance.HealthStatus = "healthy"
	case "unhealthy":
		instance.HealthStatus = "unhealthy"
	case "starting":
		instance.HealthStatus = "starting"
	default:
		instance.HealthStatus = "unknown"
	}
}

func displayInstances(instances []*types.Instance, protocol, domain string, verbose, showHealth bool) {
	if verbose {
		displayInstancesVerbose(instances, protocol, domain, showHealth)
		return
	}

	fmt.Println()

	// Create a new tabwriter
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header - plain text without colors for proper alignment
	if showHealth {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			"NAME",
			"SERVICE",
			"VERSION",
			"STATUS",
			"HEALTH",
			"PORTS",
			"URL",
		)
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			"NAME",
			"SERVICE",
			"VERSION",
			"STATUS",
			"PORTS",
			"URL",
		)
	}

	// Print each instance
	for _, instance := range instances {
		// Format name
		name := instance.Name

		// Format service type
		serviceType := instance.ServiceType
		if instance.IsMultiContainer {
			serviceType = fmt.Sprintf("%s (%d)", serviceType, len(instance.Containers))
		}

		// Format version
		version := instance.Version
		if version == "" {
			version = "-"
		} else {
			version = "v" + version
		}

		// Format status (plain text to fix alignment)
		status := formatStatusTextForTable(instance.Status)

		// Format health
		health := formatHealthForTable(instance.HealthStatus)

		// Format ports
		ports := formatPortsForTable(instance)

		// Format URL
		url := instance.URL
		if url == "" {
			url = "-"
		}

		if showHealth {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				name,
				serviceType,
				version,
				status,
				health,
				ports,
				url,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				name,
				serviceType,
				version,
				status,
				ports,
				url,
			)
		}
	}

	w.Flush()
	fmt.Println()
	color.Cyan("Total: %d service(s)", len(instances))
	fmt.Println()
}

func formatHealthForTable(health string) string {
	switch health {
	case "healthy":
		return "Healthy"
	case "unhealthy":
		return "Unhealthy"
	case "starting":
		return "Starting"
	case "none":
		return "-"
	default:
		return "-"
	}
}

func displayInstancesVerbose(instances []*types.Instance, protocol, domain string, showHealth bool) {
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Println("📋 Installed Services")
	fmt.Println()

	for i, instance := range instances {
		if i > 0 {
			fmt.Println()
		}

		displayInstance(instance, protocol, domain, true)
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

	// Multi-container info
	if instance.IsMultiContainer {
		fmt.Printf("  Type: Multi-container (%d containers)\n", len(instance.Containers))
		if verbose {
			for _, container := range instance.Containers {
				statusSymbol := getContainerStatusSymbol(container.Status)
				fmt.Printf("    %s %s\n", statusSymbol, container.Name)
			}
		}
	}

	// URL (if Traefik enabled)
	if instance.Traefik.Enabled && instance.URL != "" {
		fmt.Printf("  URL: %s\n", color.GreenString(instance.URL))
	}

	// Show dependencies if any
	if verbose && len(instance.Dependencies) > 0 {
		fmt.Printf("  Dependencies: %s\n", strings.Join(instance.Dependencies, ", "))
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

	// Show host port mappings
	if len(instance.Network.PortMappings) > 0 {
		for containerPort, hostPort := range instance.Network.PortMappings {
			fmt.Printf("  Port: localhost:%s → container:%s\n", hostPort, containerPort)
		}
	} else if instance.Network.HostPort > 0 {
		// Backward compatibility with old single port format
		fmt.Printf("  Port: localhost:%d → container:%d\n", instance.Network.HostPort, instance.Network.InternalPort)
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

func getContainerStatusSymbol(status string) string {
	switch status {
	case "running":
		return color.GreenString("●")
	case "stopped":
		return color.YellowString("○")
	case "failed":
		return color.RedString("✗")
	default:
		return color.New(color.Faint).Sprint("?")
	}
}

func formatStatusForTable(status types.ServiceStatus) string {
	switch status {
	case types.StatusRunning:
		return color.GreenString("Up")
	case types.StatusStopped:
		return color.YellowString("Exited")
	case types.StatusFailed:
		return color.RedString("Failed")
	default:
		return color.New(color.Faint).Sprint("Unknown")
	}
}

func formatStatusTextForTable(status types.ServiceStatus) string {
	switch status {
	case types.StatusRunning:
		return "Up"
	case types.StatusStopped:
		return "Exited"
	case types.StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func formatPortsForTable(instance *types.Instance) string {
	ports := []string{}

	// Collect port mappings
	if len(instance.Network.PortMappings) > 0 {
		for containerPort, hostPort := range instance.Network.PortMappings {
			ports = append(ports, fmt.Sprintf("%s->%s", hostPort, containerPort))
		}
	} else if instance.Network.HostPort > 0 {
		// Backward compatibility with old single port format
		ports = append(ports, fmt.Sprintf("%d->%d", instance.Network.HostPort, instance.Network.InternalPort))
	}

	// If no port mappings but has internal port
	if len(ports) == 0 && instance.Network.InternalPort > 0 {
		ports = append(ports, fmt.Sprintf("%d/tcp", instance.Network.InternalPort))
	}

	if len(ports) == 0 {
		return "-"
	}

	// Show first 2 ports, add ellipsis if more
	if len(ports) > 2 {
		return strings.Join(ports[:2], ", ") + "..."
	}

	return strings.Join(ports, ", ")
}
