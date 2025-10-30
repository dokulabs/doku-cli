package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	installName     string
	installEnv      []string
	installMemory   string
	installCPU      string
	installVolumes  []string
	installYes      bool
	installInternal bool
)

var installCmd = &cobra.Command{
	Use:   "install <service>[:<version>]",
	Short: "Install a service from the catalog",
	Long: `Install and start a service from the catalog.

Examples:
  doku install postgres          # Install latest PostgreSQL
  doku install postgres:16       # Install PostgreSQL 16
  doku install redis --name cache  # Install with custom name
  doku install mysql --env MYSQL_ROOT_PASSWORD=secret
  doku install postgres --memory 2g --cpu 1.0
  doku install user-service --internal  # Install as internal (no external access)`,
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
	installCmd.Flags().BoolVarP(&installYes, "yes", "y", false, "Skip confirmation prompts")
	installCmd.Flags().BoolVar(&installInternal, "internal", false, "Install as internal service (no Traefik exposure)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	serviceSpec := args[0]

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

	// Show image and resource info
	fmt.Printf("Image: %s\n", spec.Image)
	if spec.Resources != nil {
		fmt.Printf("Memory: %s - %s\n", spec.Resources.MemoryMin, spec.Resources.MemoryMax)
		fmt.Printf("CPU: %s - %s cores\n", spec.Resources.CPUMin, spec.Resources.CPUMax)
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
		ServiceName:  serviceName,
		Version:      actualVersion,
		InstanceName: installName,
		Environment:  envOverrides,
		MemoryLimit:  installMemory,
		CPULimit:     installCPU,
		Volumes:      volumeMounts,
		Internal:     installInternal,
	}

	instance, err := installer.Install(opts)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Success message
	fmt.Println()
	color.Green("✓ Successfully installed %s", instance.Name)
	fmt.Println()

	// Show connection information
	if spec.Protocol == "http" || spec.Protocol == "https" {
		color.Cyan("Access your service:")
		fmt.Printf("  URL: %s\n", instance.URL)
	} else {
		color.Cyan("Connection information:")
		fmt.Printf("  Host: %s\n", instance.Name)
		fmt.Printf("  Port: %d\n", instance.Network.InternalPort)
	}

	// Show admin port if available
	if spec.AdminPort > 0 {
		adminURL := fmt.Sprintf("%s://%s-admin.%s", protocol, instanceName, domain)
		fmt.Printf("  Admin: %s (port %d)\n", adminURL, spec.AdminPort)
	}

	fmt.Println()

	// Show useful commands
	color.Cyan("Useful commands:")
	fmt.Printf("  doku list            # List all services\n")
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
