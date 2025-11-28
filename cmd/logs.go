package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       string
	logsTimestamps bool
	logsContainer  string
	logsAll        bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "View logs from a service",
	Long: `View logs from a service instance.

By default, shows recent logs and exits. Use --follow to stream logs in real-time.

Examples:
  doku logs postgres-main                  # Show recent logs
  doku logs postgres-main -f               # Stream logs (follow mode)
  doku logs postgres-main --tail 50        # Show last 50 lines
  doku logs postgres-main -f --tail 20     # Follow, starting with last 20 lines`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (stream in real-time)")
	logsCmd.Flags().StringVar(&logsTail, "tail", "all", "Number of lines to show from the end of the logs")
	logsCmd.Flags().BoolVarP(&logsTimestamps, "timestamps", "t", false, "Show timestamps")
	logsCmd.Flags().StringVarP(&logsContainer, "container", "c", "", "Specific container name (for multi-container services)")
	logsCmd.Flags().BoolVarP(&logsAll, "all", "a", false, "Show logs from all containers (multi-container only)")
}

func runLogs(cmd *cobra.Command, args []string) error {
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
	var containerName string
	var isTraefik bool

	if instanceName == "traefik" || instanceName == "doku-traefik" {
		containerName = "doku-traefik"
		isTraefik = true

		// Check if Traefik container exists
		exists, err := dockerClient.ContainerExists(containerName)
		if err != nil {
			return fmt.Errorf("failed to check Traefik container: %w", err)
		}
		if !exists {
			return fmt.Errorf("Traefik container not found. Run 'doku init' first")
		}
	} else {
		// Regular service - get from service manager
		serviceMgr := service.NewManager(dockerClient, cfgMgr)

		// Get instance to check if it exists
		instance, err := serviceMgr.Get(instanceName)
		if err != nil {
			return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
		}

		// Handle multi-container services
		if instance.IsMultiContainer {
			return handleMultiContainerLogs(dockerClient, instance, logsFollow, logsContainer, logsAll)
		}

		containerName = instance.ContainerName

		// Check if service is running
		status, err := serviceMgr.GetStatus(instanceName)
		if err != nil {
			color.Yellow("⚠️  Could not determine service status")
		}

		// If not running and not following, just show available logs
		if !logsFollow && status != "running" {
			color.Yellow("Note: Service is not currently running. Showing historical logs.")
			fmt.Println()
		}
	}

	// Get logs using Docker client directly for better control
	logsReader, err := dockerClient.ContainerLogs(containerName, logsFollow)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	defer logsReader.Close()

	// Show info about what we're viewing
	if isTraefik && logsFollow {
		color.New(color.Faint).Println("Viewing Traefik logs (Press Ctrl+C to stop)...")
		fmt.Println()
	}

	// Setup signal handler for clean shutdown on Ctrl+C
	if logsFollow {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println() // New line after ^C
			color.New(color.Faint).Println("Log streaming stopped")
			os.Exit(0)
		}()
	}

	// Stream logs to stdout
	if _, err := io.Copy(os.Stdout, logsReader); err != nil {
		// Only return error if it's not EOF or broken pipe (which are normal for follow mode)
		if err != io.EOF && err.Error() != "write |1: broken pipe" {
			return fmt.Errorf("error reading logs: %w", err)
		}
	}

	// Show monitoring hint (only if not following and monitoring is enabled)
	if !logsFollow {
		cfg, _ := cfgMgr.Get()
		if cfg.Monitoring.Enabled && cfg.Monitoring.Tool != "none" {
			fmt.Println()
			color.New(color.Faint).Println("💡 Tip: View all service logs and metrics in one place with 'doku monitor'")
		}
	}

	return nil
}

// handleMultiContainerLogs handles log viewing for multi-container services
func handleMultiContainerLogs(dockerClient *docker.Client, instance *types.Instance, follow bool, containerName string, showAll bool) error {
	// If --all flag is set, show logs from all containers
	if showAll {
		fmt.Println()
		color.Cyan("📋 Logs from all containers in %s:", instance.Name)
		fmt.Println()

		for _, container := range instance.Containers {
			color.New(color.Bold).Printf("=== %s ===\n", container.Name)
			logsReader, err := dockerClient.ContainerLogs(container.ContainerID, false)
			if err != nil {
				color.Yellow("Failed to get logs from %s: %v\n", container.Name, err)
				continue
			}

			if _, err := io.Copy(os.Stdout, logsReader); err != nil {
				// Log copy errors are non-fatal for multi-container display
				if err != io.EOF {
					color.Yellow("Warning: error reading logs from %s: %v\n", container.Name, err)
				}
			}
			logsReader.Close()
			fmt.Println()
		}

		return nil
	}

	// If --container flag is set, show logs from specific container
	if containerName != "" {
		var targetContainer *types.ContainerInfo
		for i := range instance.Containers {
			if instance.Containers[i].Name == containerName {
				targetContainer = &instance.Containers[i]
				break
			}
		}

		if targetContainer == nil {
			return fmt.Errorf("container '%s' not found in service '%s'.\nAvailable containers: %s",
				containerName, instance.Name, getContainerNames(instance.Containers))
		}

		if follow {
			color.New(color.Faint).Printf("Viewing logs from %s (Press Ctrl+C to stop)...\n", containerName)
			fmt.Println()
		}

		logsReader, err := dockerClient.ContainerLogs(targetContainer.ContainerID, follow)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer logsReader.Close()

		// Setup signal handler for clean shutdown on Ctrl+C
		if follow {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			go func() {
				<-sigChan
				fmt.Println()
				color.New(color.Faint).Println("Log streaming stopped")
				os.Exit(0)
			}()
		}

		if _, err := io.Copy(os.Stdout, logsReader); err != nil {
			// Only return error if it's not EOF or broken pipe (normal for follow mode)
			if err != io.EOF && !strings.Contains(err.Error(), "broken pipe") {
				return fmt.Errorf("error reading logs: %w", err)
			}
		}
		return nil
	}

	// No specific container selected - show options
	fmt.Println()
	color.Yellow("⚠️  %s is a multi-container service with %d containers:", instance.Name, len(instance.Containers))
	fmt.Println()
	fmt.Println("Available containers:")
	for _, container := range instance.Containers {
		fmt.Printf("  • %s\n", container.Name)
	}
	fmt.Println()
	color.Cyan("Usage:")
	fmt.Printf("  doku logs %s --container <name>  # View specific container logs\n", instance.Name)
	fmt.Printf("  doku logs %s --all               # View all container logs\n", instance.Name)
	fmt.Println()

	return nil
}

// getContainerNames returns a comma-separated list of container names
func getContainerNames(containers []types.ContainerInfo) string {
	names := make([]string, len(containers))
	for i, container := range containers {
		names[i] = container.Name
	}
	return strings.Join(names, ", ")
}
