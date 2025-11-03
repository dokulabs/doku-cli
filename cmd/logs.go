package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       string
	logsTimestamps bool
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
