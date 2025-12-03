package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	statsWatch    bool
	statsInterval int
)

var statsCmd = &cobra.Command{
	Use:   "stats [service]",
	Short: "Display resource usage statistics for services",
	Long: `Display CPU, memory, and network usage for running services.

Without arguments, shows stats for all services.
With a service name, shows detailed stats for that service.

Examples:
  doku stats                # Show stats for all services
  doku stats postgres       # Show stats for postgres only
  doku stats --watch        # Continuously update stats
  doku stats --interval 5   # Update every 5 seconds (with --watch)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)

	statsCmd.Flags().BoolVarP(&statsWatch, "watch", "w", false, "Continuously update stats")
	statsCmd.Flags().IntVar(&statsInterval, "interval", 2, "Update interval in seconds (with --watch)")
}

func runStats(cmd *cobra.Command, args []string) error {
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

	ctx := context.Background()

	if statsWatch {
		// Continuous mode
		return watchStats(ctx, serviceMgr, dockerClient, cfgMgr, args)
	}

	// One-shot mode
	if len(args) > 0 {
		return showServiceStats(ctx, serviceMgr, dockerClient, args[0])
	}

	return showAllStats(ctx, serviceMgr, dockerClient, cfgMgr)
}

func showAllStats(ctx context.Context, serviceMgr *service.Manager, dockerClient *docker.Client, cfgMgr *config.Manager) error {
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if len(cfg.Instances) == 0 && len(cfg.Projects) == 0 {
		color.Yellow("No services installed")
		return nil
	}

	fmt.Println()
	color.Cyan("Resource Usage Statistics")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tSTATUS\tCPU %%\tMEM USAGE\tMEM %%\tNET I/O\n")
	fmt.Fprintf(w, "-------\t------\t-----\t---------\t-----\t-------\n")

	// Check instances
	for name, instance := range cfg.Instances {
		stats := getContainerStats(ctx, dockerClient, instance.ContainerName)
		status := "stopped"
		if stats != nil {
			status = "running"
		}

		cpuStr := "-"
		memStr := "-"
		memPercent := "-"
		netIO := "-"

		if stats != nil {
			cpuStr = fmt.Sprintf("%.1f%%", stats.CPUPercent)
			memStr = formatStatsBytes(stats.MemoryUsage)
			if stats.MemoryLimit > 0 {
				memPercent = fmt.Sprintf("%.1f%%", float64(stats.MemoryUsage)/float64(stats.MemoryLimit)*100)
			}
			netIO = fmt.Sprintf("%s / %s", formatStatsBytes(stats.NetworkRx), formatStatsBytes(stats.NetworkTx))
		}

		statusColor := getStatsStatusColor(status)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name,
			statusColor(status),
			cpuStr,
			memStr,
			memPercent,
			netIO,
		)
	}

	// Check projects
	for name, project := range cfg.Projects {
		stats := getContainerStats(ctx, dockerClient, project.ContainerName)
		status := "stopped"
		if stats != nil {
			status = "running"
		}

		cpuStr := "-"
		memStr := "-"
		memPercent := "-"
		netIO := "-"

		if stats != nil {
			cpuStr = fmt.Sprintf("%.1f%%", stats.CPUPercent)
			memStr = formatStatsBytes(stats.MemoryUsage)
			if stats.MemoryLimit > 0 {
				memPercent = fmt.Sprintf("%.1f%%", float64(stats.MemoryUsage)/float64(stats.MemoryLimit)*100)
			}
			netIO = fmt.Sprintf("%s / %s", formatStatsBytes(stats.NetworkRx), formatStatsBytes(stats.NetworkTx))
		}

		statusColor := getStatsStatusColor(status)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name,
			statusColor(status),
			cpuStr,
			memStr,
			memPercent,
			netIO,
		)
	}

	w.Flush()
	fmt.Println()

	return nil
}

func showServiceStats(ctx context.Context, serviceMgr *service.Manager, dockerClient *docker.Client, instanceName string) error {
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found", instanceName)
	}

	fmt.Println()
	color.Cyan("Resource Statistics: %s", instanceName)
	fmt.Println()

	if instance.IsMultiContainer {
		// Show stats for each container
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "CONTAINER\tCPU %%\tMEM USAGE\tMEM LIMIT\tMEM %%\n")
		fmt.Fprintf(w, "---------\t-----\t---------\t---------\t-----\n")

		for _, container := range instance.Containers {
			stats := getContainerStats(ctx, dockerClient, container.ContainerID)
			if stats != nil {
				memPercent := float64(0)
				if stats.MemoryLimit > 0 {
					memPercent = float64(stats.MemoryUsage) / float64(stats.MemoryLimit) * 100
				}
				fmt.Fprintf(w, "%s\t%.1f%%\t%s\t%s\t%.1f%%\n",
					container.Name,
					stats.CPUPercent,
					formatStatsBytes(stats.MemoryUsage),
					formatStatsBytes(stats.MemoryLimit),
					memPercent,
				)
			} else {
				fmt.Fprintf(w, "%s\t-\t-\t-\t-\n", container.Name)
			}
		}
		w.Flush()
	} else {
		// Show detailed stats for single container
		stats := getContainerStats(ctx, dockerClient, instance.ContainerName)
		if stats == nil {
			color.Yellow("Container is not running")
			return nil
		}

		fmt.Printf("CPU Usage:       %.2f%%\n", stats.CPUPercent)
		fmt.Printf("Memory Usage:    %s / %s\n",
			formatStatsBytes(stats.MemoryUsage),
			formatStatsBytes(stats.MemoryLimit))
		if stats.MemoryLimit > 0 {
			fmt.Printf("Memory %%:        %.1f%%\n", float64(stats.MemoryUsage)/float64(stats.MemoryLimit)*100)
		}
		fmt.Printf("Network I/O:     %s rx / %s tx\n",
			formatStatsBytes(stats.NetworkRx),
			formatStatsBytes(stats.NetworkTx))
		fmt.Printf("Block I/O:       %s read / %s write\n",
			formatStatsBytes(stats.BlockRead),
			formatStatsBytes(stats.BlockWrite))
		fmt.Printf("PIDs:            %d\n", stats.PIDs)
	}

	fmt.Println()
	return nil
}

func watchStats(ctx context.Context, serviceMgr *service.Manager, dockerClient *docker.Client, cfgMgr *config.Manager, args []string) error {
	ticker := time.NewTicker(time.Duration(statsInterval) * time.Second)
	defer ticker.Stop()

	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[H\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	for {
		// Move to top of screen
		fmt.Print("\033[H")

		if len(args) > 0 {
			showServiceStats(ctx, serviceMgr, dockerClient, args[0])
		} else {
			showAllStats(ctx, serviceMgr, dockerClient, cfgMgr)
		}

		color.New(color.Faint).Printf("Updating every %ds. Press Ctrl+C to exit.\n", statsInterval)

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}

// ContainerStatsExtended holds extended container statistics
type ContainerStatsExtended struct {
	CPUPercent  float64
	MemoryUsage uint64
	MemoryLimit uint64
	NetworkRx   uint64
	NetworkTx   uint64
	BlockRead   uint64
	BlockWrite  uint64
	PIDs        uint64
}

func getContainerStats(ctx context.Context, dockerClient *docker.Client, containerID string) *ContainerStatsExtended {
	stats, err := dockerClient.ContainerStats(ctx, containerID)
	if err != nil {
		return nil
	}

	return &ContainerStatsExtended{
		CPUPercent:  stats.CPUPercent,
		MemoryUsage: stats.MemoryUsage,
		MemoryLimit: stats.MemoryLimit,
	}
}

func formatStatsBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getStatsStatusColor(status string) func(a ...interface{}) string {
	switch status {
	case "running":
		return color.New(color.FgGreen).SprintFunc()
	default:
		return color.New(color.FgYellow).SprintFunc()
	}
}
