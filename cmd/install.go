package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	installName               string
	installEnv                []string
	installMemory             string
	installCPU                string
	installVolumes            []string
	installPorts              []string
	installYes                bool
	installInternal           bool
	installSkipDeps           bool
	installDisableAutoInstall bool // When true, prompts before installing dependencies
	installPath               string // Path to custom project with Dockerfile
)

var installCmd = &cobra.Command{
	Use:   "install <service>[:<version>]",
	Short: "Install a service from the catalog",
	Long: `Install and start a service from the catalog or custom project.

Examples:
  # Catalog services
  doku install postgres          # Install latest PostgreSQL
  doku install postgres:16       # Install PostgreSQL 16
  doku install redis --name cache  # Install with custom name
  doku install mysql --env MYSQL_ROOT_PASSWORD=secret
  doku install postgres --memory 2g --cpu 1.0
  doku install postgres --port 5432  # Map single port
  doku install rabbitmq --port 5672 --port 15672  # Map multiple ports
  doku install rabbitmq --port 5673:5672 --port 15673:15672  # Map to different host ports
  doku install user-service --internal  # Install as internal (no external access)

  # Custom projects with Dockerfile
  doku install frontend --path=./frontend  # Install from custom Dockerfile
  doku install api --path=./api --internal  # Install as internal service
  doku install worker --path=./worker --env QUEUE_URL=redis://redis:6379`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installName, "name", "n", "", "Custom instance name")
	installCmd.Flags().StringSliceVarP(&installEnv, "env", "e", []string{}, "Environment variables (KEY=VALUE)")
	installCmd.Flags().StringVar(&installMemory, "memory", "", "Memory limit (e.g., 512m, 1g)")
	installCmd.Flags().StringVar(&installCPU, "cpu", "", "CPU limit (e.g., 0.5, 1.0)")
	installCmd.Flags().StringSliceVar(&installVolumes, "volume", []string{}, "Volume mounts (host:container)")
	installCmd.Flags().StringSliceVarP(&installPorts, "port", "p", []string{}, "Port mappings (host:container or port). Can be specified multiple times")
	installCmd.Flags().BoolVarP(&installYes, "yes", "y", false, "Skip confirmation prompts")
	installCmd.Flags().BoolVar(&installInternal, "internal", false, "Install as internal service (no Traefik exposure)")
	installCmd.Flags().BoolVar(&installSkipDeps, "skip-deps", false, "Skip dependency resolution and installation")
	installCmd.Flags().BoolVar(&installDisableAutoInstall, "no-auto-install-deps", false, "Prompt before installing dependencies (interactive mode)")
	installCmd.Flags().StringVar(&installPath, "path", "", "Path to custom project with Dockerfile")
}

func runInstall(cmd *cobra.Command, args []string) error {
	serviceSpec := args[0]

	// Check if --path is provided (custom project installation)
	if installPath != "" {
		return installCustomProject(serviceSpec)
	}

	// Parse service:version
	parts := strings.SplitN(serviceSpec, ":", 2)
	serviceName := parts[0]
	version := ""
	if len(parts) > 1 {
		version = parts[1]
	}

	// Create managers
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("⚠️  Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	// Get service from catalog
	catalogService, err := catalogMgr.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found in catalog. Try 'doku catalog search %s'", serviceName, serviceName)
	}

	// Get service version
	spec, err := catalogMgr.GetServiceVersion(serviceName, version)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Determine actual version
	actualVersion := version
	if actualVersion == "" || actualVersion == "latest" {
		for v, s := range catalogService.Versions {
			if s == spec {
				actualVersion = v
				break
			}
		}
	}

	// Display service information
	fmt.Println()
	color.Cyan("Installing: %s %s %s", catalogService.Icon, catalogService.Name, actualVersion)
	fmt.Println(catalogService.Description)
	fmt.Println()

	// Show multi-container info or image
	if spec.IsMultiContainer() {
		fmt.Printf("Type: Multi-container service\n")
		fmt.Printf("Containers: %d\n", len(spec.Containers))
		for _, container := range spec.Containers {
			prefix := "  •"
			if container.Primary {
				prefix = "  ⭐"
			}
			fmt.Printf("%s %s (%s)\n", prefix, container.Name, container.Image)
		}
	} else {
		fmt.Printf("Image: %s\n", spec.Image)
	}

	if spec.Resources != nil {
		fmt.Printf("Memory: %s - %s\n", spec.Resources.MemoryMin, spec.Resources.MemoryMax)
		fmt.Printf("CPU: %s - %s cores\n", spec.Resources.CPUMin, spec.Resources.CPUMax)
	}

	// Show dependencies if any
	if len(spec.Dependencies) > 0 {
		fmt.Println()
		color.Cyan("Dependencies:")
		for _, dep := range spec.Dependencies {
			required := "optional"
			if dep.Required {
				required = "required"
			}
			fmt.Printf("  • %s (%s) - %s\n", dep.Name, dep.Version, required)
		}
	}
	fmt.Println()

	// Parse environment variables
	envOverrides := make(map[string]string)
	for _, env := range installEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envOverrides[parts[0]] = parts[1]
		}
	}

	// Interactive configuration if not using --yes
	if !installYes && spec.Configuration != nil && len(spec.Configuration.Options) > 0 {
		color.Cyan("Configuration:")
		fmt.Println()

		for _, opt := range spec.Configuration.Options {
			// Skip if already provided via --env
			if _, exists := envOverrides[opt.EnvVar]; exists {
				continue
			}

			// Get value from user
			value, err := promptForOption(opt)
			if err != nil {
				return err
			}

			if value != "" {
				envOverrides[opt.EnvVar] = value
			}
		}

		fmt.Println()
	}

	// Parse volumes
	volumeMounts := make(map[string]string)
	for _, vol := range installVolumes {
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) == 2 {
			volumeMounts[parts[0]] = parts[1]
		}
	}

	// Parse port mappings
	portMappings, err := parsePortMappings(installPorts, spec.Port)
	if err != nil {
		return fmt.Errorf("invalid port mapping: %w", err)
	}

	// Show instance name
	instanceName := installName
	if instanceName == "" {
		instanceName = serviceName
		if actualVersion != "" && actualVersion != "latest" {
			instanceName = fmt.Sprintf("%s-%s", serviceName, strings.ReplaceAll(actualVersion, ".", "-"))
		}
	}

	fmt.Printf("Instance name: %s\n", color.CyanString(instanceName))

	// Get config for URL
	cfg, _ := cfgMgr.Get()
	protocol := cfg.Preferences.Protocol
	if protocol == "" {
		protocol = "https"
	}
	domain := cfg.Preferences.Domain
	if domain == "" {
		domain = "doku.local"
	}

	// Allow user to customize domain
	if !installYes && (spec.Protocol == "http" || spec.Protocol == "https") {
		fmt.Println()
		domainPrompt := &survey.Input{
			Message: "Domain for this service:",
			Default: domain,
			Help:    "The domain where this service will be accessible (e.g., doku.local, myapp.local)",
		}
		if err := survey.AskOne(domainPrompt, &domain); err != nil {
			return err
		}
	}

	if spec.Protocol == "http" || spec.Protocol == "https" {
		fmt.Printf("URL: %s://%s.%s\n", protocol, instanceName, domain)
	}
	fmt.Println()

	// Confirm installation
	if !installYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with installation?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Installation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create installer
	installer, err := service.NewInstaller(dockerClient, cfgMgr, catalogMgr)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Install service
	opts := service.InstallOptions{
		ServiceName:      serviceName,
		Version:          actualVersion,
		InstanceName:     installName,
		Environment:      envOverrides,
		MemoryLimit:      installMemory,
		CPULimit:         installCPU,
		Volumes:          volumeMounts,
		PortMappings:     portMappings,
		Internal:         installInternal,
		SkipDependencies: installSkipDeps,
		AutoInstallDeps:  !installDisableAutoInstall,
	}

	instance, err := installer.Install(opts)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Success message
	fmt.Println()
	color.Green("✓ Successfully installed %s", instance.Name)
	fmt.Println()

	// Show DNS setup message for manual mode
	if cfg.Preferences.DNSSetup == "manual" && (spec.Protocol == "http" || spec.Protocol == "https") {
		color.New(color.Bold, color.FgYellow).Println("📝 Manual DNS Setup Required:")
		fmt.Println()
		fmt.Printf("Add this entry to your DNS or /etc/hosts:\n")
		color.Cyan("  127.0.0.1 %s.%s", instance.Name, domain)
		fmt.Println()
		fmt.Println()
	}

	// Show multi-container status if applicable
	if instance.IsMultiContainer {
		fmt.Printf("Containers running: %d\n", len(instance.Containers))
		for _, container := range instance.Containers {
			status := "✓"
			if container.Status != "running" {
				status = "✗"
			}
			fmt.Printf("  %s %s\n", status, container.Name)
		}
		fmt.Println()
	}

	// Show connection information
	if spec.Protocol == "http" || spec.Protocol == "https" {
		color.Cyan("Access your service:")
		fmt.Printf("  URL: %s\n", instance.URL)
		if cfg.Preferences.DNSSetup == "manual" {
			color.New(color.Faint).Printf("  (requires DNS setup above)\n")
		}
	} else if !instance.IsMultiContainer {
		color.Cyan("Connection information:")
		fmt.Printf("  Host: %s\n", instance.Name)
		fmt.Printf("  Port: %d\n", instance.Network.InternalPort)
	}

	// Show admin port if available
	// For services with admin ports (like RabbitMQ), the main URL routes to the admin/management UI
	// The AMQP/protocol port is accessed via direct connection
	if spec.AdminPort > 0 {
		fmt.Printf("  Management UI: %s (port %d)\n", instance.URL, spec.AdminPort)
	}

	// Show monitoring status
	monitoringCfg, _ := cfgMgr.GetMonitoringConfig()
	if monitoringCfg != nil && monitoringCfg.Enabled && monitoringCfg.Tool != "none" {
		fmt.Println()
		color.New(color.FgGreen, color.Faint).Printf("✓ Monitoring: Sending data to %s\n", getMonitoringToolName(monitoringCfg.Tool))
		color.New(color.Faint).Printf("  Dashboard: %s\n", monitoringCfg.URL)
	}

	fmt.Println()

	// Show useful commands
	color.Cyan("Useful commands:")
	fmt.Printf("  doku env %s      # Show environment variables\n", instance.Name)
	fmt.Printf("  doku info %s     # Show detailed information\n", instance.Name)
	fmt.Printf("  doku logs %s     # View logs\n", instance.Name)
	fmt.Printf("  doku stop %s     # Stop service\n", instance.Name)
	fmt.Printf("  doku remove %s   # Remove service\n", instance.Name)
	fmt.Println()

	return nil
}

// promptForOption prompts user for a configuration option
func promptForOption(opt types.ConfigOption) (string, error) {
	// Build prompt message
	message := opt.Description
	if opt.Default != "" {
		message = fmt.Sprintf("%s (default: %s)", message, opt.Default)
	}

	var value string

	switch opt.Type {
	case "bool":
		var boolValue bool
		defaultBool := opt.Default == "true"
		prompt := &survey.Confirm{
			Message: message,
			Default: defaultBool,
		}
		if err := survey.AskOne(prompt, &boolValue); err != nil {
			return "", err
		}
		if boolValue {
			value = "true"
		} else {
			value = "false"
		}

	case "select":
		prompt := &survey.Select{
			Message: message,
			Options: opt.Options,
			Default: opt.Default,
		}
		if err := survey.AskOne(prompt, &value); err != nil {
			return "", err
		}

	default: // string, int
		prompt := &survey.Input{
			Message: message,
			Default: opt.Default,
		}
		if err := survey.AskOne(prompt, &value); err != nil {
			return "", err
		}
	}

	return value, nil
}

// getMonitoringToolName returns the display name for a monitoring tool
func getMonitoringToolName(tool string) string {
	switch tool {
	case "signoz":
		return "SignOz"
	case "sentry":
		return "Sentry"
	default:
		return tool
	}
}

// parsePortMappings parses port mapping strings into a map[containerPort]hostPort
// Supports formats:
//   - "5432"         -> maps container port 5432 to host port 5432
//   - "5433:5432"    -> maps container port 5432 to host port 5433
func parsePortMappings(portStrings []string, defaultPort int) (map[string]string, error) {
	if len(portStrings) == 0 {
		return nil, nil
	}

	mappings := make(map[string]string)

	for _, portStr := range portStrings {
		parts := strings.Split(portStr, ":")

		if len(parts) == 1 {
			// Format: "5432" - map container port to same host port
			// Validate it's a number
			if _, err := strconv.Atoi(parts[0]); err != nil {
				return nil, fmt.Errorf("invalid port number '%s': %w", parts[0], err)
			}
			mappings[parts[0]] = parts[0]
		} else if len(parts) == 2 {
			// Format: "5433:5432" - map container port to different host port
			// Validate both are numbers
			if _, err := strconv.Atoi(parts[0]); err != nil {
				return nil, fmt.Errorf("invalid host port '%s': %w", parts[0], err)
			}
			if _, err := strconv.Atoi(parts[1]); err != nil {
				return nil, fmt.Errorf("invalid container port '%s': %w", parts[1], err)
			}
			mappings[parts[1]] = parts[0] // containerPort -> hostPort
		} else {
			return nil, fmt.Errorf("invalid port mapping format '%s' (use 'port' or 'host:container')", portStr)
		}
	}

	return mappings, nil
}

// installCustomProject installs a custom project from a Dockerfile
func installCustomProject(serviceName string) error {
	// Create managers
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to create project manager: %w", err)
	}

	// Parse environment variables
	envOverrides := make(map[string]string)
	for _, env := range installEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envOverrides[parts[0]] = parts[1]
		}
	}

	// Parse port mappings
	var mainPort int
	additionalPorts := []string{}
	if len(installPorts) > 0 {
		// First port is the main port
		parts := strings.Split(installPorts[0], ":")
		if len(parts) >= 1 {
			mainPort, _ = strconv.Atoi(parts[len(parts)-1])
		}
		// Rest are additional ports
		if len(installPorts) > 1 {
			additionalPorts = installPorts[1:]
		}
	}

	// Determine instance name
	instanceName := installName
	if instanceName == "" {
		instanceName = serviceName
	}

	// Get config for domain
	cfg, _ := cfgMgr.Get()
	protocol := cfg.Preferences.Protocol
	if protocol == "" {
		protocol = "https"
	}
	domain := cfg.Preferences.Domain
	if domain == "" {
		domain = "doku.local"
	}

	// Display information
	fmt.Println()
	color.Cyan("Installing custom project: %s", instanceName)
	fmt.Printf("Path: %s\n", installPath)
	if installInternal {
		fmt.Println("Mode: Internal (no Traefik exposure)")
	} else {
		fmt.Println("Mode: Public (Traefik enabled)")
		if mainPort > 0 {
			fmt.Printf("URL: %s://%s.%s\n", protocol, instanceName, domain)
		}
	}
	if mainPort > 0 {
		fmt.Printf("Port: %d\n", mainPort)
	}
	fmt.Println()

	// Confirm if not using --yes
	if !installYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with installation?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			color.Yellow("Installation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Step 1: Add project
	color.Cyan("Step 1/3: Adding project...")
	addOpts := project.AddOptions{
		ProjectPath:  installPath,
		Name:         instanceName,
		Port:         mainPort,
		Ports:        additionalPorts,
		Environment:  envOverrides,
		Internal:     installInternal,
	}

	proj, err := projectMgr.Add(addOpts)
	if err != nil {
		return fmt.Errorf("failed to add project: %w", err)
	}
	color.Green("✓ Project added")
	fmt.Println()

	// Step 2: Build project
	color.Cyan("Step 2/3: Building Docker image...")
	buildOpts := project.BuildOptions{
		Name: instanceName,
	}

	if err := projectMgr.Build(buildOpts); err != nil {
		return fmt.Errorf("failed to build project: %w", err)
	}
	color.Green("✓ Build completed")
	fmt.Println()

	// Step 3: Run project
	color.Cyan("Step 3/3: Starting container...")
	runOpts := project.RunOptions{
		Name:   instanceName,
		Detach: true,
	}

	if err := projectMgr.Run(runOpts); err != nil {
		return fmt.Errorf("failed to run project: %w", err)
	}
	color.Green("✓ Container started")
	fmt.Println()

	// Success message
	color.Green("✓ Successfully installed %s", instanceName)
	fmt.Println()

	// Show connection information
	if !installInternal && proj.URL != "" {
		color.Cyan("Access your service:")
		fmt.Printf("  URL: %s\n", proj.URL)
	} else if mainPort > 0 {
		color.Cyan("Connection information:")
		fmt.Printf("  Host: %s\n", instanceName)
		fmt.Printf("  Port: %d\n", mainPort)
	}

	fmt.Println()

	// Show useful commands
	color.Cyan("Useful commands:")
	fmt.Printf("  doku env %s      # Show environment variables\n", instanceName)
	fmt.Printf("  doku logs %s     # View logs\n", instanceName)
	fmt.Printf("  doku stop %s     # Stop service\n", instanceName)
	fmt.Printf("  doku remove %s   # Remove service\n", instanceName)
	fmt.Println()

	return nil
}
