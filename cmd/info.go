package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	infoShowEnv bool
)

var infoCmd = &cobra.Command{
	Use:   "info <service>",
	Short: "Show detailed information about a service",
	Long: `Display detailed information about an installed service including:
  • Status and uptime
  • Access URLs and connection strings
  • Environment variables
  • Resource usage and limits
  • Volume mounts
  • Network configuration`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)

	infoCmd.Flags().BoolVarP(&infoShowEnv, "env", "e", false, "Show environment variables")
}

func runInfo(cmd *cobra.Command, args []string) error {
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

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance
	instance, err := serviceMgr.Get(instanceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", instanceName)
	}

	// Get container info from Docker
	containerInfo, err := dockerClient.ContainerInspect(instance.ContainerName)
	if err != nil {
		color.Yellow("⚠️  Warning: Could not get container information")
		containerInfo = dockerTypes.ContainerJSON{} // Empty struct
	}

	// Update status
	updateStatus(instance, containerInfo)

	// Display information
	displayServiceInfo(instance, cfg, containerInfo, infoShowEnv)

	return nil
}

func updateStatus(instance *types.Instance, containerInfo dockerTypes.ContainerJSON) {
	if containerInfo.State == nil {
		instance.Status = types.StatusUnknown
		return
	}

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
}

func displayServiceInfo(instance *types.Instance, cfg *types.Config, containerInfo dockerTypes.ContainerJSON, showEnv bool) {
	// Header
	fmt.Println()
	statusIcon := getInfoStatusIcon(instance.Status)
	fmt.Printf("%s ", statusIcon)
	color.New(color.Bold, color.FgCyan).Printf("%s", instance.Name)
	fmt.Printf(" ")
	getInfoStatusColor(instance.Status)(" [%s]", string(instance.Status))
	fmt.Println()
	fmt.Println(strings.Repeat("=", len(instance.Name)+20))
	fmt.Println()

	// Basic Information
	color.New(color.Bold).Println("Service Information")
	fmt.Printf("  Type: %s\n", color.CyanString(instance.ServiceType))
	if instance.Version != "" {
		fmt.Printf("  Version: %s\n", instance.Version)
	}
	fmt.Printf("  Container: %s\n", instance.ContainerName)
	fmt.Printf("  Created: %s\n", instance.CreatedAt.Format("2006-01-02 15:04:05"))
	if instance.Status == types.StatusRunning && containerInfo.State != nil {
		fmt.Printf("  Uptime: %s\n", formatUptime(containerInfo.State.StartedAt))
	}
	fmt.Println()

	// Access Information
	color.New(color.Bold).Println("Access")
	if instance.Traefik.Enabled {
		fmt.Printf("  URL: %s\n", color.GreenString(instance.URL))
		fmt.Printf("  Protocol: %s\n", instance.Traefik.Protocol)
		fmt.Printf("  Subdomain: %s.%s\n", instance.Traefik.Subdomain, cfg.Preferences.Domain)
	} else {
		fmt.Printf("  Type: %s\n", color.YellowString("Internal only"))
		if instance.Network.InternalPort > 0 {
			fmt.Printf("  Internal Port: %d\n", instance.Network.InternalPort)
		}
	}
	fmt.Println()

	// Connection Information
	if instance.ConnectionString != "" {
		color.New(color.Bold).Println("Connection")
		fmt.Printf("  String: %s\n", color.GreenString(instance.ConnectionString))

		// Show connection examples based on service type
		showConnectionExamples(instance)
		fmt.Println()
	}

	// Network Information
	color.New(color.Bold).Println("Network")
	fmt.Printf("  Network: %s\n", instance.Network.Name)
	if instance.Network.InternalPort > 0 {
		fmt.Printf("  Internal Port: %d\n", instance.Network.InternalPort)
	}
	if instance.Network.HostPort > 0 {
		fmt.Printf("  Host Port: %d\n", instance.Network.HostPort)
	}
	fmt.Println()

	// Resource Information
	color.New(color.Bold).Println("Resources")
	if instance.Resources.MemoryLimit != "" {
		fmt.Printf("  Memory Limit: %s\n", instance.Resources.MemoryLimit)
	} else {
		fmt.Printf("  Memory Limit: %s\n", color.New(color.Faint).Sprint("unlimited"))
	}

	if instance.Resources.CPULimit != "" {
		fmt.Printf("  CPU Limit: %s\n", instance.Resources.CPULimit)
	} else {
		fmt.Printf("  CPU Limit: %s\n", color.New(color.Faint).Sprint("unlimited"))
	}
	fmt.Println()

	// Volume Information
	if len(instance.Volumes) > 0 {
		color.New(color.Bold).Println("Volumes")
		for name, path := range instance.Volumes {
			fmt.Printf("  %s → %s\n", color.CyanString(name), path)
		}
		fmt.Println()
	}

	// Environment Variables
	if showEnv && len(instance.Environment) > 0 {
		color.New(color.Bold).Println("Environment Variables")
		for key, value := range instance.Environment {
			// Mask sensitive values
			displayValue := value
			if isSensitiveKey(key) {
				displayValue = maskValue(value)
			}
			fmt.Printf("  %s=%s\n", color.YellowString(key), displayValue)
		}
		fmt.Println()
	} else if !showEnv && len(instance.Environment) > 0 {
		color.New(color.Faint).Printf("Use --env flag to show environment variables\n\n")
	}

	// Container Details
	if containerInfo.State != nil && instance.Status == types.StatusFailed {
		color.New(color.Bold, color.FgRed).Println("Error Information")
		if containerInfo.State.Error != "" {
			fmt.Printf("  Error: %s\n", color.RedString(containerInfo.State.Error))
		}
		fmt.Printf("  Exit Code: %d\n", containerInfo.State.ExitCode)
		fmt.Println()
	}

	// Management Commands
	color.New(color.Bold).Println("Management Commands")
	if instance.Status == types.StatusRunning {
		fmt.Println("  Stop:    " + color.CyanString("doku stop %s", instance.Name))
		fmt.Println("  Restart: " + color.CyanString("doku restart %s", instance.Name))
		fmt.Println("  Logs:    " + color.CyanString("doku logs %s", instance.Name))
	} else {
		fmt.Println("  Start:   " + color.CyanString("doku start %s", instance.Name))
	}
	fmt.Println("  Remove:  " + color.CyanString("doku remove %s", instance.Name))
	fmt.Println()
}

func showConnectionExamples(instance *types.Instance) {
	serviceType := strings.ToLower(instance.ServiceType)

	switch serviceType {
	case "postgres", "postgresql":
		fmt.Println()
		fmt.Println("  Examples:")
		fmt.Printf("    Node.js:  %s\n", color.New(color.Faint).Sprint("const { Pool } = require('pg'); const pool = new Pool({ connectionString: '"+instance.ConnectionString+"' })"))
		fmt.Printf("    Python:   %s\n", color.New(color.Faint).Sprint("import psycopg2; conn = psycopg2.connect('"+instance.ConnectionString+"')"))

	case "mysql":
		fmt.Println()
		fmt.Println("  Examples:")
		fmt.Printf("    Node.js:  %s\n", color.New(color.Faint).Sprint("const mysql = require('mysql2'); const conn = mysql.createConnection('"+instance.ConnectionString+"')"))
		fmt.Printf("    Python:   %s\n", color.New(color.Faint).Sprint("import mysql.connector; conn = mysql.connector.connect('"+instance.ConnectionString+"')"))

	case "redis":
		fmt.Println()
		fmt.Println("  Examples:")
		fmt.Printf("    Node.js:  %s\n", color.New(color.Faint).Sprint("const redis = require('redis'); const client = redis.createClient({ url: '"+instance.ConnectionString+"' })"))
		fmt.Printf("    Python:   %s\n", color.New(color.Faint).Sprint("import redis; r = redis.from_url('"+instance.ConnectionString+"')"))

	case "mongodb", "mongo":
		fmt.Println()
		fmt.Println("  Examples:")
		fmt.Printf("    Node.js:  %s\n", color.New(color.Faint).Sprint("const { MongoClient } = require('mongodb'); const client = new MongoClient('"+instance.ConnectionString+"')"))
		fmt.Printf("    Python:   %s\n", color.New(color.Faint).Sprint("from pymongo import MongoClient; client = MongoClient('"+instance.ConnectionString+"')"))
	}
}

func getInfoStatusIcon(status types.ServiceStatus) string {
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

func getInfoStatusColor(status types.ServiceStatus) func(format string, a ...interface{}) {
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

func formatUptime(startedAt string) string {
	if startedAt == "" {
		return "N/A"
	}

	// Parse Docker's time format
	// This is a simplified version
	return "Active" // Would need proper time parsing from Docker format
}
