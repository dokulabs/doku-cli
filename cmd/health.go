package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health [service]",
	Short: "Show health status of services",
	Long: `Display detailed health information for one or all services.

Shows:
  - Container status and health check results
  - Uptime and restart count
  - Resource usage summary
  - Network connectivity

Examples:
  doku health              # Show health for all services
  doku health postgres     # Show detailed health for postgres`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

// HealthInfo contains health information for a service
type HealthInfo struct {
	Name          string
	Status        string
	Health        string
	Uptime        string
	RestartCount  int
	CPUPercent    string
	MemoryUsage   string
	NetworkStatus string
	Containers    []ContainerHealth
}

// ContainerHealth contains health info for a single container
type ContainerHealth struct {
	Name         string
	Status       string
	Health       string
	Uptime       string
	RestartCount int
}

func runHealth(cmd *cobra.Command, args []string) error {
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

	fmt.Println()

	if len(args) > 0 {
		// Show detailed health for specific service
		return showDetailedHealth(serviceMgr, dockerClient, args[0])
	}

	// Show health summary for all services
	return showHealthSummary(serviceMgr, dockerClient, cfgMgr)
}

func showHealthSummary(serviceMgr *service.Manager, dockerClient *docker.Client, cfgMgr *config.Manager) error {
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if len(cfg.Instances) == 0 && len(cfg.Projects) == 0 {
		color.Yellow("No services installed")
		fmt.Println()
		return nil
	}

	color.Cyan("Service Health Status")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tSTATUS\tHEALTH\tUPTIME\tRESTARTS\n")
	fmt.Fprintf(w, "-------\t------\t------\t------\t--------\n")

	// Check each instance
	for name := range cfg.Instances {
		health := getServiceHealth(serviceMgr, dockerClient, name)
		statusColor := getHealthStatusColor(health.Status)
		healthColor := getHealthColor(health.Health)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			name,
			statusColor(health.Status),
			healthColor(health.Health),
			health.Uptime,
			health.RestartCount,
		)
	}

	// Check each project
	for name := range cfg.Projects {
		health := getServiceHealth(serviceMgr, dockerClient, name)
		statusColor := getHealthStatusColor(health.Status)
		healthColor := getHealthColor(health.Health)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			name,
			statusColor(health.Status),
			healthColor(health.Health),
			health.Uptime,
			health.RestartCount,
		)
	}

	w.Flush()
	fmt.Println()

	return nil
}

func showDetailedHealth(serviceMgr *service.Manager, dockerClient *docker.Client, instanceName string) error {
	health := getServiceHealth(serviceMgr, dockerClient, instanceName)

	color.Cyan("Health Status: %s", instanceName)
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()

	// Status section
	statusColor := getHealthStatusColor(health.Status)
	healthColor := getHealthColor(health.Health)

	fmt.Printf("Status:       %s\n", statusColor(health.Status))
	fmt.Printf("Health:       %s\n", healthColor(health.Health))
	fmt.Printf("Uptime:       %s\n", health.Uptime)
	fmt.Printf("Restarts:     %d\n", health.RestartCount)
	fmt.Println()

	// Resource usage
	if health.CPUPercent != "" || health.MemoryUsage != "" {
		color.Cyan("Resource Usage")
		fmt.Println(strings.Repeat("-", 20))
		if health.CPUPercent != "" {
			fmt.Printf("CPU:          %s\n", health.CPUPercent)
		}
		if health.MemoryUsage != "" {
			fmt.Printf("Memory:       %s\n", health.MemoryUsage)
		}
		fmt.Println()
	}

	// Multi-container details
	if len(health.Containers) > 1 {
		color.Cyan("Containers")
		fmt.Println(strings.Repeat("-", 20))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "NAME\tSTATUS\tHEALTH\tRESTARTS\n")

		for _, c := range health.Containers {
			statusColor := getHealthStatusColor(c.Status)
			healthColor := getHealthColor(c.Health)
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				c.Name,
				statusColor(c.Status),
				healthColor(c.Health),
				c.RestartCount,
			)
		}
		w.Flush()
		fmt.Println()
	}

	// Network status
	fmt.Printf("Network:      %s\n", health.NetworkStatus)
	fmt.Println()

	return nil
}

func getServiceHealth(serviceMgr *service.Manager, dockerClient *docker.Client, instanceName string) HealthInfo {
	health := HealthInfo{
		Name:          instanceName,
		Status:        "unknown",
		Health:        "unknown",
		Uptime:        "-",
		NetworkStatus: "unknown",
	}

	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		health.Status = "not found"
		return health
	}

	ctx := context.Background()

	if instance.IsMultiContainer {
		// Multi-container service
		allRunning := true
		anyUnhealthy := false
		totalRestarts := 0

		for _, container := range instance.Containers {
			containerHealth := ContainerHealth{
				Name:   container.Name,
				Status: "unknown",
				Health: "unknown",
			}

			info, err := dockerClient.ContainerInspect(container.ContainerID)
			if err != nil {
				containerHealth.Status = "error"
				allRunning = false
			} else {
				if info.State.Running {
					containerHealth.Status = "running"
					containerHealth.Uptime = formatHealthUptime(info.State.StartedAt)
				} else {
					containerHealth.Status = "stopped"
					allRunning = false
				}

				containerHealth.RestartCount = info.RestartCount
				totalRestarts += info.RestartCount

				// Check health status
				if info.State.Health != nil {
					containerHealth.Health = info.State.Health.Status
					if containerHealth.Health != "healthy" {
						anyUnhealthy = true
					}
				} else {
					containerHealth.Health = "no healthcheck"
				}
			}

			health.Containers = append(health.Containers, containerHealth)
		}

		if allRunning {
			health.Status = "running"
		} else {
			health.Status = "degraded"
		}

		if anyUnhealthy {
			health.Health = "unhealthy"
		} else if allRunning {
			health.Health = "healthy"
		} else {
			health.Health = "degraded"
		}

		health.RestartCount = totalRestarts
	} else {
		// Single container service
		info, err := dockerClient.ContainerInspect(instance.ContainerName)
		if err != nil {
			health.Status = "error"
			return health
		}

		if info.State.Running {
			health.Status = "running"
			health.Uptime = formatHealthUptime(info.State.StartedAt)
		} else {
			health.Status = "stopped"
		}

		health.RestartCount = info.RestartCount

		// Check health status
		if info.State.Health != nil {
			health.Health = info.State.Health.Status
		} else {
			health.Health = "no healthcheck"
		}

		// Get resource stats
		stats, err := dockerClient.ContainerStats(ctx, instance.ContainerName)
		if err == nil && stats != nil {
			health.CPUPercent = fmt.Sprintf("%.2f%%", stats.CPUPercent)
			health.MemoryUsage = fmt.Sprintf("%s / %s",
				formatBytes(int64(stats.MemoryUsage)),
				formatBytes(int64(stats.MemoryLimit)))
		}
	}

	// Check network connectivity
	networkMgr := docker.NewNetworkManager(dockerClient)
	connected, _ := networkMgr.IsContainerConnected("doku-network", instance.ContainerName)
	if connected {
		health.NetworkStatus = color.GreenString("connected to doku-network")
	} else {
		health.NetworkStatus = color.YellowString("not connected")
	}

	return health
}

func getHealthStatusColor(status string) func(a ...interface{}) string {
	switch strings.ToLower(status) {
	case "running":
		return color.New(color.FgGreen).SprintFunc()
	case "stopped", "exited":
		return color.New(color.FgRed).SprintFunc()
	case "degraded":
		return color.New(color.FgYellow).SprintFunc()
	default:
		return color.New(color.FgWhite).SprintFunc()
	}
}

func getHealthColor(health string) func(a ...interface{}) string {
	switch strings.ToLower(health) {
	case "healthy":
		return color.New(color.FgGreen).SprintFunc()
	case "unhealthy":
		return color.New(color.FgRed).SprintFunc()
	case "starting":
		return color.New(color.FgYellow).SprintFunc()
	case "no healthcheck":
		return color.New(color.Faint).SprintFunc()
	default:
		return color.New(color.FgWhite).SprintFunc()
	}
}

func formatHealthUptime(startedAt string) string {
	startTime, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return "-"
	}

	duration := time.Since(startTime)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
	} else {
		days := int(duration.Hours() / 24)
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}
