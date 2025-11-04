package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/certs"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/dns"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/internal/traefik"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	initDomain   string
	initProtocol string
	initSkipDNS  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Doku on your system",
	Long: `Initialize Doku on your system by:
  • Checking Docker availability
  • Setting up SSL certificates with mkcert
  • Configuring DNS (*.doku.local)
  • Creating Docker network
  • Installing Traefik reverse proxy
  • Downloading service catalog`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&initDomain, "domain", "doku.local", "Domain to use for services")
	initCmd.Flags().StringVar(&initProtocol, "protocol", "", "Protocol (http or https)")
	initCmd.Flags().BoolVar(&initSkipDNS, "skip-dns", false, "Skip DNS/hosts file configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	printHeader("Welcome to Doku Setup")

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if already initialized
	if cfgMgr.IsInitialized() {
		reinit := false
		prompt := &survey.Confirm{
			Message: "Doku is already initialized. Reinitialize?",
			Default: false,
		}
		survey.AskOne(prompt, &reinit)

		if !reinit {
			color.Yellow("⚠️  Initialization cancelled")
			return nil
		}
	}

	// Step 1: Check Docker
	printStep(1, "Checking Docker availability")
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("Docker is not available: %w", err)
	}
	defer dockerClient.Close()

	if err := dockerClient.Ping(); err != nil {
		return fmt.Errorf("Docker daemon is not running: %w", err)
	}

	version, _ := dockerClient.Version()
	printSuccess(fmt.Sprintf("Docker detected (version %s)", version.Version))

	// Step 2: Prompt for preferences
	printStep(2, "Configuration")

	// Protocol selection (if not provided via flag)
	if initProtocol == "" {
		protocolChoice := ""
		protocolPrompt := &survey.Select{
			Message: "Choose protocol for local services:",
			Options: []string{
				"HTTPS (recommended, with local certificates)",
				"HTTP only",
			},
			Default: "HTTPS (recommended, with local certificates)",
		}
		survey.AskOne(protocolPrompt, &protocolChoice)

		if protocolChoice == "HTTPS (recommended, with local certificates)" {
			initProtocol = "https"
		} else {
			initProtocol = "http"
		}
	}

	// Domain selection (if not provided via flag)
	if initDomain == "" {
		domainPrompt := &survey.Input{
			Message: "Domain name for services:",
			Default: "doku.local",
		}
		survey.AskOne(domainPrompt, &initDomain)
	}

	printSuccess(fmt.Sprintf("Protocol: %s, Domain: %s", initProtocol, initDomain))

	// Step 2.5: Monitoring tool selection
	fmt.Println()
	color.New(color.Bold, color.FgMagenta).Println("📊 Monitoring & Observability")
	fmt.Println()
	color.New(color.Faint).Println("Doku can automatically configure monitoring for all your services.")
	color.New(color.Faint).Println("View logs, traces, and metrics in one unified dashboard.")
	fmt.Println()

	monitoringChoice := ""
	monitoringPrompt := &survey.Select{
		Message: "Choose monitoring tool:",
		Options: []string{
			"SignOz (Recommended) - Full observability (logs, traces, metrics)",
			"Sentry - Error tracking & performance monitoring",
			"None - Skip monitoring setup",
		},
		Default: "SignOz (Recommended) - Full observability (logs, traces, metrics)",
		Help:    "SignOz is lightweight and provides comprehensive monitoring. Sentry focuses on error tracking.",
	}
	survey.AskOne(monitoringPrompt, &monitoringChoice)

	// Parse monitoring tool choice
	var monitoringTool string
	if monitoringChoice == "" || monitoringChoice == "None - Skip monitoring setup" {
		monitoringTool = "none"
	} else if strings.Contains(monitoringChoice, "SignOz") {
		monitoringTool = "signoz"
	} else if strings.Contains(monitoringChoice, "Sentry") {
		monitoringTool = "sentry"
	} else {
		monitoringTool = "none"
	}

	if monitoringTool != "none" {
		printSuccess(fmt.Sprintf("Monitoring tool: %s", monitoringTool))
		color.New(color.Faint).Println("  → All services will automatically send data to " + monitoringTool)
	} else {
		color.New(color.Faint).Println("  → Skipping monitoring setup (you can set it up later)")
	}

	// Step 3: Initialize configuration
	printStep(3, "Initializing configuration")
	if err := cfgMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Set preferences
	if err := cfgMgr.SetDomain(initDomain); err != nil {
		return fmt.Errorf("failed to set domain: %w", err)
	}
	if err := cfgMgr.SetProtocol(initProtocol); err != nil {
		return fmt.Errorf("failed to set protocol: %w", err)
	}
	if err := cfgMgr.SetMonitoringTool(monitoringTool); err != nil {
		return fmt.Errorf("failed to set monitoring tool: %w", err)
	}

	printSuccess(fmt.Sprintf("Configuration saved to %s", cfgMgr.GetDokuDir()))

	// Step 4: Setup SSL certificates (if HTTPS)
	if initProtocol == "https" {
		printStep(4, "Setting up SSL certificates")

		certMgr := certs.NewManager(cfgMgr.GetCertsDir(), initDomain)

		// Check if mkcert is installed
		if !certMgr.IsMkcertInstalled() {
			fmt.Println("⚠️  mkcert not found, attempting to install...")
			if err := certMgr.InstallMkcert(); err != nil {
				color.Yellow("⚠️  Could not install mkcert automatically")
				color.Yellow("Please install mkcert manually: https://github.com/FiloSottile/mkcert")
				return fmt.Errorf("mkcert installation required")
			}
			printSuccess("mkcert installed")
		}

		// Install CA
		if err := certMgr.InstallCA(); err != nil {
			return fmt.Errorf("failed to install CA: %w", err)
		}
		printSuccess("CA certificate installed to system trust store")

		// Generate certificates
		if err := certMgr.GenerateCertificates(); err != nil {
			return fmt.Errorf("failed to generate certificates: %w", err)
		}
		printSuccess(fmt.Sprintf("SSL certificates generated for %s and *.%s", initDomain, initDomain))
	}

	// Step 5: Configure DNS
	if !initSkipDNS {
		if initProtocol == "https" {
			printStep(5, "Configuring DNS")
		} else {
			printStep(4, "Configuring DNS")
		}

		dnsMgr := dns.NewManager()

		dnsMethod := ""
		dnsPrompt := &survey.Select{
			Message: "DNS setup method:",
			Options: []string{
				"Automatic (/etc/hosts modification)",
				"Manual (I'll configure DNS myself)",
			},
			Default: "Automatic (/etc/hosts modification)",
		}
		survey.AskOne(dnsPrompt, &dnsMethod)

		if dnsMethod == "Automatic (/etc/hosts modification)" {
			fmt.Println("⚠️  This requires administrator privileges")

			if err := dnsMgr.AddDokuDomain(initDomain); err != nil {
				color.Yellow("⚠️  Failed to automatically configure DNS")
				fmt.Println()
				color.New(color.Bold, color.FgYellow).Println("Manual DNS Setup Required:")
				fmt.Println()
				color.New(color.Bold).Printf("Add entries to %s:\n", dnsMgr.GetHostsFilePath())
				fmt.Println()
				fmt.Printf("  sudo sh -c \"cat >> %s << 'EOF'\n", dnsMgr.GetHostsFilePath())
				fmt.Println("# Doku local development")
				color.Cyan("127.0.0.1 %s", initDomain)
				color.Cyan("127.0.0.1 traefik.%s", initDomain)
				color.Cyan("# Add more entries as you install services:")
				color.Cyan("# 127.0.0.1 <service>.%s", initDomain)
				fmt.Println("EOF\"")
				fmt.Println()
				color.New(color.Faint).Println("Note: You'll need to add an entry for each service you install.")
				color.New(color.Faint).Println("You can continue for now and set up DNS later.")
				fmt.Println()
			} else {
				printSuccess(fmt.Sprintf("DNS entries added to %s", dnsMgr.GetHostsFilePath()))
			}

			// Update config with DNS setup method
			if err := cfgMgr.Update(func(c *types.Config) error {
				c.Preferences.DNSSetup = "hosts"
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update DNS setup method: %w", err)
			}
		} else {
			printSuccess("Skipping automatic DNS setup")
			color.Yellow(fmt.Sprintf("\nPlease configure DNS for *.%s to point to 127.0.0.1", initDomain))

			// Update config with DNS setup method
			if err := cfgMgr.Update(func(c *types.Config) error {
				c.Preferences.DNSSetup = "manual"
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update DNS setup method: %w", err)
			}
		}
	}

	// Step 6: Create Docker network
	stepNum := 6
	if initProtocol == "http" {
		stepNum = 5
	}
	if initSkipDNS {
		stepNum--
	}
	printStep(stepNum, "Setting up Docker network")

	networkMgr := docker.NewNetworkManager(dockerClient)
	if err := networkMgr.EnsureDokuNetwork("doku-network", "172.20.0.0/16", "172.20.0.1"); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	printSuccess("Docker network 'doku-network' created")

	// Step 7: Setup Traefik
	stepNum++
	printStep(stepNum, "Installing Traefik reverse proxy")

	traefikMgr := traefik.NewManager(
		dockerClient,
		cfgMgr.GetTraefikDir(),
		cfgMgr.GetCertsDir(),
		initDomain,
		initProtocol,
	)

	// Check if Traefik container already exists
	traefikExists, err := dockerClient.ContainerExists(traefik.TraefikContainerName)
	if err != nil {
		return fmt.Errorf("failed to check Traefik container: %w", err)
	}

	// If exists, ask user what to do
	if traefikExists {
		color.Yellow("⚠️  Traefik container already exists")

		recreate := false
		recreatePrompt := &survey.Confirm{
			Message: "Do you want to remove and recreate Traefik? (Recommended for clean setup)",
			Default: true,
		}
		if err := survey.AskOne(recreatePrompt, &recreate); err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}

		if recreate {
			fmt.Println("Removing existing Traefik container...")

			// Disconnect from network first
			networkMgr.DisconnectContainer("doku-network", traefik.TraefikContainerName, true)

			// Remove container
			if err := traefikMgr.RemoveContainer(); err != nil {
				return fmt.Errorf("failed to remove existing Traefik: %w", err)
			}

			// Setup Traefik (create and start)
			if err := traefikMgr.Setup(); err != nil {
				return fmt.Errorf("failed to setup Traefik: %w", err)
			}

			// Connect Traefik to doku-network
			if err := networkMgr.ConnectContainer("doku-network", traefik.TraefikContainerName); err != nil {
				return fmt.Errorf("failed to connect Traefik to network: %w", err)
			}

			dashboardURL := traefikMgr.GetDashboardURL()
			printSuccess(fmt.Sprintf("Traefik installed and running"))
			printSuccess(fmt.Sprintf("Dashboard: %s", dashboardURL))

			// Update config with Traefik status
			if err := cfgMgr.Update(func(c *types.Config) error {
				c.Traefik.DashboardURL = dashboardURL
				c.Traefik.Status = "running"
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update Traefik status: %w", err)
			}
		} else {
			// Use existing Traefik - just ensure it's running
			isRunning, err := traefikMgr.IsRunning()
			if err != nil {
				return fmt.Errorf("failed to check Traefik status: %w", err)
			}

			if !isRunning {
				fmt.Println("Starting existing Traefik container...")
				containerInfo, _ := dockerClient.ContainerInspect(traefik.TraefikContainerName)
				if err := dockerClient.ContainerStart(containerInfo.ID); err != nil {
					return fmt.Errorf("failed to start existing Traefik: %w", err)
				}
			}

			printSuccess("Using existing Traefik container")

			// Update config with Traefik status
			dashboardURL := traefikMgr.GetDashboardURL()
			if err := cfgMgr.Update(func(c *types.Config) error {
				c.Traefik.DashboardURL = dashboardURL
				c.Traefik.Status = "running"
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update Traefik status: %w", err)
			}
		}
	} else {
		// No existing Traefik, create fresh
		// Setup Traefik (create and start)
		if err := traefikMgr.Setup(); err != nil {
			return fmt.Errorf("failed to setup Traefik: %w", err)
		}

		// Connect Traefik to doku-network
		if err := networkMgr.ConnectContainer("doku-network", traefik.TraefikContainerName); err != nil {
			return fmt.Errorf("failed to connect Traefik to network: %w", err)
		}

		dashboardURL := traefikMgr.GetDashboardURL()
		printSuccess(fmt.Sprintf("Traefik installed and running"))
		printSuccess(fmt.Sprintf("Dashboard: %s", dashboardURL))

		// Update config with Traefik status
		if err := cfgMgr.Update(func(c *types.Config) error {
			c.Traefik.DashboardURL = dashboardURL
			c.Traefik.Status = "running"
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update Traefik status: %w", err)
		}
	}
	// Step 8: Download catalog
	stepNum++
	printStep(stepNum, "Downloading service catalog")

	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Try to fetch catalog
	if err := catalogMgr.FetchCatalog(); err != nil {
		color.Yellow("⚠️  Could not download catalog from GitHub: %v", err)
		color.Yellow("Catalog will be available after running: doku catalog update")
	} else {
		// Validate catalog
		if err := catalogMgr.ValidateCatalog(); err != nil {
			color.Yellow("⚠️  Catalog validation failed: %v", err)
		} else {
			// Get catalog version and count services
			version, _ := catalogMgr.GetCatalogVersion()
			services, _ := catalogMgr.ListServices()

			printSuccess(fmt.Sprintf("Catalog downloaded (version: %s, services: %d)", version, len(services)))

			// Update config with catalog version
			if version != "" {
				cfgMgr.UpdateCatalogVersion(version)
			}
		}
	}

	// Step 9: Install monitoring tool (if selected)
	if monitoringTool != "none" {
		stepNum++
		printStep(stepNum, fmt.Sprintf("Installing %s", getMonitoringDisplayName(monitoringTool)))

		if err := installMonitoringTool(dockerClient, cfgMgr, catalogMgr, monitoringTool, initProtocol, initDomain); err != nil {
			color.Yellow("⚠️  Failed to install %s: %v", monitoringTool, err)
			color.Yellow("You can install it manually later with: doku install %s", monitoringTool)
		} else {
			printSuccess(fmt.Sprintf("%s installed and configured", getMonitoringDisplayName(monitoringTool)))

			// Get monitoring URL
			monitoringURL := getMonitoringURL(cfgMgr, monitoringTool, initProtocol, initDomain)
			printSuccess(fmt.Sprintf("Dashboard: %s", monitoringURL))
			color.New(color.Faint).Println("  → All future services will automatically send data to " + monitoringTool)
		}
	}

	// Final success message
	printHeader("Setup Complete! 🎉")

	fmt.Println()
	color.Green("✓ Docker: Running")
	color.Green("✓ Network: doku-network created")
	color.Green("✓ Traefik: Running")
	if initProtocol == "https" {
		color.Green("✓ SSL: Certificates installed")
	}
	if !initSkipDNS {
		color.Green("✓ DNS: Configured")
	}
	if catalogMgr.CatalogExists() {
		services, _ := catalogMgr.ListServices()
		color.Green(fmt.Sprintf("✓ Catalog: %d services available", len(services)))
	}
	if monitoringTool != "none" {
		monitoringCfg, _ := cfgMgr.GetMonitoringConfig()
		if monitoringCfg != nil && monitoringCfg.Enabled {
			color.Green(fmt.Sprintf("✓ Monitoring: %s configured", getMonitoringDisplayName(monitoringTool)))
		}
	}

	fmt.Println()
	color.Cyan("Next steps:")
	fmt.Println("  • Browse catalog: doku catalog")
	fmt.Println("  • Install a service: doku install <service>")
	fmt.Println(fmt.Sprintf("  • View Traefik dashboard: %s", traefikMgr.GetDashboardURL()))
	if monitoringTool != "none" {
		monitoringURL := getMonitoringURL(cfgMgr, monitoringTool, initProtocol, initDomain)
		fmt.Println(fmt.Sprintf("  • View monitoring dashboard: %s", monitoringURL))
	}

	fmt.Println()
	color.Green(fmt.Sprintf("All services will be accessible at: %s://<service>.%s", initProtocol, initDomain))
	if monitoringTool != "none" {
		color.New(color.Faint).Println(fmt.Sprintf("→ All services will automatically send logs to %s", getMonitoringDisplayName(monitoringTool)))
	}

	return nil
}

// Helper functions for pretty output

func printHeader(message string) {
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Println("╔" + repeat("═", len(message)+2) + "╗")
	color.New(color.Bold, color.FgCyan).Printf("║ %s ║\n", message)
	color.New(color.Bold, color.FgCyan).Println("╚" + repeat("═", len(message)+2) + "╝")
	fmt.Println()
}

func printStep(step int, message string) {
	fmt.Println()
	color.New(color.Bold, color.FgYellow).Printf("[%d] %s\n", step, message)
}

func printSuccess(message string) {
	color.Green("✓ " + message)
}

func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

// getMonitoringDisplayName returns the display name for a monitoring tool
func getMonitoringDisplayName(tool string) string {
	switch tool {
	case "signoz":
		return "SignOz"
	case "sentry":
		return "Sentry"
	default:
		return tool
	}
}

// getMonitoringURL returns the dashboard URL for a monitoring tool
func getMonitoringURL(cfgMgr *config.Manager, tool, protocol, domain string) string {
	instanceName := tool
	return fmt.Sprintf("%s://%s.%s", protocol, instanceName, domain)
}

// installMonitoringTool installs the selected monitoring tool
func installMonitoringTool(dockerClient *docker.Client, cfgMgr *config.Manager, catalogMgr *catalog.Manager, tool, protocol, domain string) error {
	// Tool-specific pre-installation checks and dependency installation
	switch tool {
	case "sentry":
		// Sentry requires PostgreSQL and Redis
		if err := installSentryDependencies(dockerClient, cfgMgr, catalogMgr); err != nil {
			return fmt.Errorf("failed to install Sentry dependencies: %w", err)
		}
	}

	// Create service installer
	installer, err := service.NewInstaller(dockerClient, cfgMgr, catalogMgr)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Install options for the monitoring tool
	opts := service.InstallOptions{
		ServiceName:  tool,
		Version:      "latest",
		InstanceName: tool, // Use tool name as instance name for simplicity
		Environment:  make(map[string]string),
		Volumes:      make(map[string]string),
		Internal:     false, // Expose via Traefik
	}

	// Tool-specific environment configuration
	switch tool {
	case "sentry":
		// Configure Sentry to use our PostgreSQL and Redis
		opts.Environment["SENTRY_POSTGRES_HOST"] = "postgres"
		opts.Environment["SENTRY_REDIS_HOST"] = "redis"
		opts.Environment["SENTRY_SECRET_KEY"] = generateSentrySecretKey()
		opts.Environment["SENTRY_SINGLE_ORGANIZATION"] = "true"
	}

	// Install the monitoring tool
	fmt.Println("  Installing", getMonitoringDisplayName(tool), "...")
	instance, err := installer.Install(opts)
	if err != nil {
		return fmt.Errorf("failed to install %s: %w", tool, err)
	}

	// Wait for service to be ready
	fmt.Println("  Waiting for service to be ready...")
	time.Sleep(5 * time.Second)

	// Configure monitoring in the config
	monitoringURL := fmt.Sprintf("%s://%s.%s", protocol, tool, domain)
	var dsn string

	switch tool {
	case "signoz":
		// SignOz OTLP endpoint (internal Docker network)
		dsn = fmt.Sprintf("http://%s:4318", tool)

		// Save configuration immediately
		if err := cfgMgr.ConfigureMonitoring(tool, monitoringURL, dsn); err != nil {
			return fmt.Errorf("failed to save monitoring config: %w", err)
		}

		// Show success message
		fmt.Println()
		color.Green("✓ SignOz installed and configured")
		fmt.Println()
		color.Cyan("SignOz Dashboard:")
		fmt.Println("  URL:", monitoringURL)
		fmt.Println()
		color.Cyan("OpenTelemetry Configuration:")
		fmt.Println("  OTLP Endpoint:", dsn)
		fmt.Println("  Protocol: HTTP/Protobuf")
		fmt.Println()
		color.New(color.Faint).Println("All services installed after this will automatically send:")
		color.New(color.Faint).Println("  • Logs - Application and system logs")
		color.New(color.Faint).Println("  • Traces - Request flows and dependencies")
		color.New(color.Faint).Println("  • Metrics - Performance and resource usage")

	case "sentry":
		// For Sentry, we need user to configure DSN after first login
		// Save initial config without DSN
		if err := cfgMgr.ConfigureMonitoring(tool, monitoringURL, ""); err != nil {
			return fmt.Errorf("failed to save monitoring config: %w", err)
		}

		// Show setup instructions
		fmt.Println()
		color.Green("✓ Sentry installed successfully")
		fmt.Println()
		color.Cyan("Next steps to complete Sentry setup:")
		fmt.Println()
		fmt.Println("1. Open Sentry dashboard:", color.CyanString(monitoringURL))
		fmt.Println("2. Create your admin account")
		fmt.Println("3. Create a new project")
		fmt.Println("4. Copy the DSN from project settings")
		fmt.Println("5. Run: doku config set monitoring.dsn <your-dsn>")
		fmt.Println()
		color.New(color.Faint).Println("After configuring the DSN, all services will send errors and performance data to Sentry")
	}
	fmt.Println()

	_ = instance // Use instance variable
	return nil
}

// installSentryDependencies installs PostgreSQL and Redis for Sentry
func installSentryDependencies(dockerClient *docker.Client, cfgMgr *config.Manager, catalogMgr *catalog.Manager) error {
	serviceMgr := service.NewManager(dockerClient, cfgMgr)
	installer, err := service.NewInstaller(dockerClient, cfgMgr, catalogMgr)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Check and install PostgreSQL
	fmt.Println("  Checking PostgreSQL...")
	if !cfgMgr.HasInstance("postgres") {
		fmt.Println("  Installing PostgreSQL for Sentry...")
		opts := service.InstallOptions{
			ServiceName:  "postgres",
			Version:      "latest",
			InstanceName: "postgres",
			Environment: map[string]string{
				"POSTGRES_USER":     "sentry",
				"POSTGRES_PASSWORD": "sentry",
				"POSTGRES_DB":       "sentry",
			},
			Internal: true, // Internal service, no Traefik exposure
		}
		if _, err := installer.Install(opts); err != nil {
			return fmt.Errorf("failed to install PostgreSQL: %w", err)
		}
		// Wait for PostgreSQL to be ready
		time.Sleep(5 * time.Second)
	} else {
		// Check if running
		if status, _ := serviceMgr.GetStatus("postgres"); status != "running" {
			return fmt.Errorf("PostgreSQL is installed but not running. Run: doku start postgres")
		}
		fmt.Println("  ✓ PostgreSQL already installed")
	}

	// Check and install Redis
	fmt.Println("  Checking Redis...")
	if !cfgMgr.HasInstance("redis") {
		fmt.Println("  Installing Redis for Sentry...")
		opts := service.InstallOptions{
			ServiceName:  "redis",
			Version:      "latest",
			InstanceName: "redis",
			Internal:     true, // Internal service
		}
		if _, err := installer.Install(opts); err != nil {
			return fmt.Errorf("failed to install Redis: %w", err)
		}
		// Wait for Redis to be ready
		time.Sleep(3 * time.Second)
	} else {
		// Check if running
		if status, _ := serviceMgr.GetStatus("redis"); status != "running" {
			return fmt.Errorf("Redis is installed but not running. Run: doku start redis")
		}
		fmt.Println("  ✓ Redis already installed")
	}

	return nil
}

// generateSentrySecretKey generates a cryptographically secure random secret key for Sentry
func generateSentrySecretKey() string {
	// Generate 32 random bytes (256 bits) for strong security
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp-based key if crypto/rand fails (very unlikely)
		return fmt.Sprintf("doku-sentry-key-%d", time.Now().UnixNano())
	}
	// Convert to hex string (64 characters)
	return hex.EncodeToString(b)
}
