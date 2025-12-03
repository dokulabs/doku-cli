package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Network management and inspection commands",
	Long: `Commands for managing and inspecting Docker networks used by Doku.

Examples:
  doku network list                # List all Doku networks
  doku network inspect             # Inspect the Doku network
  doku network connections         # Show service connections`,
	Aliases: []string{"net"},
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Docker networks managed by Doku",
	Long: `List all Docker networks that are created and managed by Doku.

Example:
  doku network list`,
	Aliases: []string{"ls"},
	RunE:    runNetworkList,
}

var networkInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect the Doku network",
	Long: `Show detailed information about the Doku Docker network.

Displays:
  - Network configuration (subnet, gateway)
  - Connected containers
  - IP address assignments

Example:
  doku network inspect`,
	RunE: runNetworkInspect,
}

var networkConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Show service network connections",
	Long: `Show how services can connect to each other within the Doku network.

This displays:
  - Container hostnames for inter-service communication
  - Port information for each service
  - Connection strings for common services

Example:
  doku network connections`,
	Aliases: []string{"conn"},
	RunE:    runNetworkConnections,
}

func init() {
	rootCmd.AddCommand(networkCmd)

	networkCmd.AddCommand(networkListCmd)
	networkCmd.AddCommand(networkInspectCmd)
	networkCmd.AddCommand(networkConnectionsCmd)
}

func runNetworkList(cmd *cobra.Command, args []string) error {
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

	// Get Doku network name from config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	fmt.Println()
	color.Cyan("Doku Networks")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NETWORK\tDRIVER\tSUBNET\tGATEWAY\tCONTAINERS\n")
	fmt.Fprintf(w, "-------\t------\t------\t-------\t----------\n")

	// Main Doku network
	networkName := cfg.Network.Name
	if networkName == "" {
		networkName = "doku"
	}

	networkInfo, err := dockerClient.NetworkInspect(networkName)
	if err != nil {
		fmt.Fprintf(w, "%s\t-\t-\t-\t%s\n", networkName, color.RedString("not found"))
	} else {
		subnet := "-"
		gateway := "-"
		if len(networkInfo.IPAM.Config) > 0 {
			subnet = networkInfo.IPAM.Config[0].Subnet
			gateway = networkInfo.IPAM.Config[0].Gateway
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			networkName,
			networkInfo.Driver,
			subnet,
			gateway,
			len(networkInfo.Containers))
	}

	w.Flush()
	fmt.Println()

	return nil
}

func runNetworkInspect(cmd *cobra.Command, args []string) error {
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

	// Get Doku network name from config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	networkName := cfg.Network.Name
	if networkName == "" {
		networkName = "doku"
	}

	// Inspect network
	networkInfo, err := dockerClient.NetworkInspect(networkName)
	if err != nil {
		return fmt.Errorf("network '%s' not found. Run 'doku init' to create it", networkName)
	}

	fmt.Println()
	color.Cyan("Network: %s", networkName)
	fmt.Println()

	// Network details
	fmt.Printf("ID:       %s\n", networkInfo.ID[:12])
	fmt.Printf("Driver:   %s\n", networkInfo.Driver)
	fmt.Printf("Scope:    %s\n", networkInfo.Scope)
	fmt.Printf("Internal: %t\n", networkInfo.Internal)

	if len(networkInfo.IPAM.Config) > 0 {
		fmt.Println()
		color.New(color.Bold).Println("IPAM Configuration:")
		for _, ipamConfig := range networkInfo.IPAM.Config {
			fmt.Printf("  Subnet:   %s\n", ipamConfig.Subnet)
			fmt.Printf("  Gateway:  %s\n", ipamConfig.Gateway)
			if ipamConfig.IPRange != "" {
				fmt.Printf("  IP Range: %s\n", ipamConfig.IPRange)
			}
		}
	}

	// Connected containers
	if len(networkInfo.Containers) > 0 {
		fmt.Println()
		color.New(color.Bold).Println("Connected Containers:")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "CONTAINER\tIPv4 ADDRESS\tMAC ADDRESS\n")
		fmt.Fprintf(w, "---------\t------------\t-----------\n")

		// Sort containers by name
		names := make([]string, 0, len(networkInfo.Containers))
		nameMap := make(map[string]string) // name -> id
		for id := range networkInfo.Containers {
			container := networkInfo.Containers[id]
			names = append(names, container.Name)
			nameMap[container.Name] = id
		}
		sort.Strings(names)

		for _, name := range names {
			id := nameMap[name]
			container := networkInfo.Containers[id]
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				name,
				container.IPv4Address,
				container.MacAddress)
		}
		w.Flush()
	} else {
		fmt.Println()
		color.Yellow("No containers connected to this network")
	}

	fmt.Println()

	return nil
}

func runNetworkConnections(cmd *cobra.Command, args []string) error {
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

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if len(cfg.Instances) == 0 && len(cfg.Projects) == 0 {
		color.Yellow("No services installed")
		return nil
	}

	networkName := cfg.Network.Name
	if networkName == "" {
		networkName = "doku"
	}

	fmt.Println()
	color.Cyan("Service Network Connections")
	fmt.Println()
	fmt.Printf("Network: %s\n", color.CyanString(networkName))
	fmt.Println()

	// List services and their connection info
	fmt.Println()
	color.New(color.Bold).Println("Service Connection Information:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tHOSTNAME\tINTERNAL PORT\tSTATUS\n")
	fmt.Fprintf(w, "-------\t--------\t-------------\t------\n")

	// Process instances
	for name, instance := range cfg.Instances {
		hostname := instance.ContainerName
		if hostname == "" {
			hostname = fmt.Sprintf("doku-%s", name)
		}

		port := instance.Network.InternalPort
		if port == 0 {
			port = instance.Traefik.Port
		}

		portStr := "-"
		if port > 0 {
			portStr = fmt.Sprintf("%d", port)
		}

		// Check if running
		status := "stopped"
		if info, err := dockerClient.ContainerInspect(hostname); err == nil && info.State != nil && info.State.Running {
			status = "running"
		}

		statusColor := color.New(color.FgYellow).SprintFunc()
		if status == "running" {
			statusColor = color.New(color.FgGreen).SprintFunc()
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			name,
			hostname,
			portStr,
			statusColor(status))
	}

	// Process projects
	for name, project := range cfg.Projects {
		hostname := project.ContainerName
		if hostname == "" {
			hostname = fmt.Sprintf("doku-%s", name)
		}

		portStr := "-"
		if project.Port > 0 {
			portStr = fmt.Sprintf("%d", project.Port)
		}

		// Check if running
		status := "stopped"
		if info, err := dockerClient.ContainerInspect(hostname); err == nil && info.State != nil && info.State.Running {
			status = "running"
		}

		statusColor := color.New(color.FgYellow).SprintFunc()
		if status == "running" {
			statusColor = color.New(color.FgGreen).SprintFunc()
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			name,
			hostname,
			portStr,
			statusColor(status))
	}

	w.Flush()

	// Show connection examples
	fmt.Println()
	color.New(color.Bold).Println("Connection Examples:")
	fmt.Println()

	for name, instance := range cfg.Instances {
		hostname := instance.ContainerName
		if hostname == "" {
			hostname = fmt.Sprintf("doku-%s", name)
		}

		port := instance.Network.InternalPort
		if port == 0 {
			port = instance.Traefik.Port
		}

		if port == 0 {
			continue
		}

		// Show connection string based on service type
		connStr := getConnectionExample(instance.ServiceType, hostname, port)
		if connStr != "" {
			fmt.Printf("  %s:\n", color.CyanString(name))
			fmt.Printf("    %s\n", connStr)
			fmt.Println()
		}
	}

	// Tips
	color.New(color.Faint).Println("Tip: Containers on the same network can connect using hostnames")
	color.New(color.Faint).Printf("Example: Connect to postgres from another container using: %s\n",
		color.CyanString("postgresql://user:pass@doku-postgres:5432/db"))
	fmt.Println()

	return nil
}

// getConnectionExample returns a connection string example for common services
func getConnectionExample(serviceType, hostname string, port int) string {
	switch strings.ToLower(serviceType) {
	case "postgres", "postgresql":
		return fmt.Sprintf("postgresql://user:password@%s:%d/database", hostname, port)
	case "mysql", "mariadb":
		return fmt.Sprintf("mysql://user:password@%s:%d/database", hostname, port)
	case "redis":
		return fmt.Sprintf("redis://%s:%d", hostname, port)
	case "mongodb", "mongo":
		return fmt.Sprintf("mongodb://%s:%d", hostname, port)
	case "rabbitmq":
		return fmt.Sprintf("amqp://user:password@%s:%d", hostname, port)
	case "elasticsearch":
		return fmt.Sprintf("http://%s:%d", hostname, port)
	case "minio":
		return fmt.Sprintf("http://%s:%d", hostname, port)
	case "clickhouse":
		return fmt.Sprintf("clickhouse://%s:%d", hostname, port)
	default:
		return fmt.Sprintf("http://%s:%d", hostname, port)
	}
}
