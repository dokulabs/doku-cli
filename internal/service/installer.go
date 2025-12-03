package service

import (
	"context"
	"fmt"
	"strings"

	dockerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/dns"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/monitoring"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
)

// Installer handles service installation
type Installer struct {
	dockerClient *docker.Client
	configMgr    *config.Manager
	catalogMgr   *catalog.Manager
	domain       string
	protocol     string
}

// NewInstaller creates a new service installer
func NewInstaller(dockerClient *docker.Client, configMgr *config.Manager, catalogMgr *catalog.Manager) (*Installer, error) {
	// Get domain and protocol from config
	cfg, err := configMgr.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	domain := cfg.Preferences.Domain
	if domain == "" {
		domain = "doku.local"
	}

	protocol := cfg.Preferences.Protocol
	if protocol == "" {
		protocol = "https"
	}

	return &Installer{
		dockerClient: dockerClient,
		configMgr:    configMgr,
		catalogMgr:   catalogMgr,
		domain:       domain,
		protocol:     protocol,
	}, nil
}

// InstallOptions holds options for service installation
type InstallOptions struct {
	ServiceName  string            // Service name from catalog
	Version      string            // Version to install (empty = latest)
	InstanceName string            // Custom instance name (empty = auto-generate)
	Environment  map[string]string // Override environment variables
	MemoryLimit  string            // Override memory limit
	CPULimit     string            // Override CPU limit
	Volumes      map[string]string // Volume mappings (host:container)
	PortMappings map[string]string // Port mappings (containerPort:hostPort as strings)
	Internal     bool              // If true, don't expose via Traefik

	// Dependency management (Phase 3)
	SkipDependencies bool // If true, skip dependency resolution
	AutoInstallDeps  bool // If true, auto-install dependencies without prompting
	IsDepend         bool // Internal: true if this is being installed as a dependency
	Replace          bool // If true, replace existing instance without prompting

	// Data reuse options
	ReuseExistingData bool // If true, reuse existing volumes and env files
	ForceCleanData    bool // If true, delete existing data without prompting
}

// Install installs a service from the catalog
func (i *Installer) Install(opts InstallOptions) (*types.Instance, error) {
	// Step 1: Resolve dependencies (Phase 3)
	if !opts.SkipDependencies && !opts.IsDepend {
		if err := i.resolveDependencies(opts); err != nil {
			return nil, err
		}
	}

	// Get service spec from catalog
	spec, err := i.catalogMgr.GetServiceVersion(opts.ServiceName, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get service spec: %w", err)
	}

	service, err := i.catalogMgr.GetService(opts.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// Determine actual version
	version := opts.Version
	if version == "" || version == "latest" {
		// Find the version key that matches the spec
		for v, s := range service.Versions {
			if s == spec {
				version = v
				break
			}
		}
	}

	// Generate instance name if not provided
	instanceName := opts.InstanceName
	if instanceName == "" {
		instanceName, err = i.generateInstanceName(opts.ServiceName, version)
		if err != nil {
			return nil, fmt.Errorf("failed to generate instance name: %w", err)
		}
	}

	// Check if instance already exists
	if i.configMgr.HasInstance(instanceName) {
		// If this is a dependency installation, fail immediately (don't prompt)
		if opts.IsDepend {
			return nil, fmt.Errorf("instance '%s' already exists", instanceName)
		}

		// If Replace flag is set, remove existing instance
		if !opts.Replace {
			// Prompt user to confirm replacement
			fmt.Println()
			color.Yellow("⚠️  Instance '%s' already exists", instanceName)
			fmt.Println()
			fmt.Printf("Do you want to remove and reinstall it? This will:\n")
			fmt.Printf("  • Remove the existing '%s' instance\n", instanceName)
			fmt.Printf("  • Keep dependencies (zookeeper, clickhouse, etc.)\n")
			fmt.Printf("  • Install a fresh instance\n")
			fmt.Println()
			fmt.Print("Remove and reinstall? (y/N): ")

			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))

			if response != "y" && response != "yes" {
				return nil, fmt.Errorf("installation cancelled: instance '%s' already exists", instanceName)
			}

			opts.Replace = true
		}

		// Remove existing instance (preserve volumes during reinstall)
		color.Cyan("Removing existing instance '%s'...", instanceName)
		mgr := NewManager(i.dockerClient, i.configMgr)
		if err := mgr.Remove(instanceName, false, false); err != nil {
			return nil, fmt.Errorf("failed to remove existing instance: %w", err)
		}
		color.Green("✓ Existing instance removed")
		fmt.Println()
	}

	// Step 2: Check for existing data (volumes, env files) from previous installation
	var existingData *ExistingData
	if !opts.IsDepend {
		existingData, err = i.checkExistingData(instanceName)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing data: %w", err)
		}

		if existingData.HasData {
			// If force clean data, delete everything
			if opts.ForceCleanData {
				color.Cyan("Cleaning existing data for '%s'...", instanceName)
				if err := i.deleteExistingData(instanceName, existingData); err != nil {
					return nil, fmt.Errorf("failed to delete existing data: %w", err)
				}
				existingData = nil // Reset since we deleted it
			} else if !opts.ReuseExistingData {
				// Prompt user
				reuseData, shouldProceed := i.promptForExistingData(instanceName, existingData)
				if !shouldProceed {
					return nil, fmt.Errorf("installation cancelled by user")
				}

				if !reuseData {
					// User wants to delete existing data
					color.Cyan("Cleaning existing data...")
					if err := i.deleteExistingData(instanceName, existingData); err != nil {
						return nil, fmt.Errorf("failed to delete existing data: %w", err)
					}
					existingData = nil // Reset since we deleted it
				} else {
					color.Green("✓ Will reuse existing data")
					fmt.Println()
				}
			}
		}
	}

	// Step 3: Check if multi-container service (Phase 3)
	if spec.IsMultiContainer() {
		return i.installMultiContainer(opts, spec, instanceName, version, existingData)
	}

	// Single-container installation (existing logic)
	// Merge environment variables: catalog defaults + user overrides
	env := i.mergeEnvironment(spec.Environment, opts.Environment)

	// If we have existing env data and user chose to reuse, merge with existing
	if existingData != nil && len(existingData.EnvVars) > 0 {
		// Existing values take precedence (user's data is preserved)
		env = i.mergeWithExistingEnv(existingData.EnvVars, env)
		color.Cyan("Merged %d existing environment variables", len(existingData.EnvVars))
	}

	// Add monitoring instrumentation environment variables
	cfg, _ := i.configMgr.Get()
	if cfg.Monitoring.Enabled && cfg.Monitoring.Tool != "none" {
		monitoringEnv := monitoring.GetInstrumentationEnv(instanceName, &cfg.Monitoring)
		env = i.mergeEnvironment(env, monitoringEnv)
	}

	// Determine resource limits
	memoryLimit := opts.MemoryLimit
	if memoryLimit == "" && spec.Resources != nil {
		memoryLimit = spec.Resources.MemoryMax
	}

	cpuLimit := opts.CPULimit
	if cpuLimit == "" && spec.Resources != nil {
		cpuLimit = spec.Resources.CPUMax
	}

	// Create container name
	containerName := docker.GenerateContainerName(instanceName)

	// Check if image exists locally first
	imageExists, err := i.dockerClient.ImageExists(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to check image existence: %w", err)
	}

	if imageExists {
		fmt.Printf("Using cached image %s\n", spec.Image)
	} else {
		// Pull image if not in cache
		fmt.Printf("Pulling image %s...\n", spec.Image)
		if err := i.dockerClient.ImagePull(spec.Image); err != nil {
			return nil, fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// Create container configuration
	containerConfig := &dockerTypes.Config{
		Image:        spec.Image,
		Env:          i.envMapToSlice(env),
		Labels:       i.generateLabels(instanceName, service, spec, opts.Internal),
		ExposedPorts: i.createExposedPorts(opts.PortMappings),
	}

	// Set custom command if specified in the service spec
	if len(spec.Command) > 0 {
		containerConfig.Cmd = spec.Command
	}

	// Create host configuration
	hostConfig := &dockerTypes.HostConfig{
		RestartPolicy: dockerTypes.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts:       i.createMounts(instanceName, spec, opts.Volumes),
		LogConfig:    *monitoring.GetDockerLoggingConfig(&cfg.Monitoring),
		PortBindings: i.createPortBindings(opts.PortMappings),
	}

	// Apply resource limits
	if err := i.applyResourceLimits(hostConfig, memoryLimit, cpuLimit); err != nil {
		return nil, fmt.Errorf("failed to apply resource limits: %w", err)
	}

	// Create container
	fmt.Printf("Creating container %s...\n", instanceName)
	containerID, err := i.dockerClient.ContainerCreate(
		containerConfig,
		hostConfig,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Connect to doku-network with aliases
	networkMgr := docker.NewNetworkManager(i.dockerClient)

	// Build network aliases: service name and instance name
	aliases := []string{opts.ServiceName}
	if instanceName != opts.ServiceName {
		aliases = append(aliases, instanceName)
	}

	if err := networkMgr.ConnectContainerWithAliases("doku-network", containerID, aliases); err != nil {
		// Cleanup on failure
		i.dockerClient.ContainerRemove(containerName, true)
		return nil, fmt.Errorf("failed to connect to network: %w", err)
	}

	// Start container
	fmt.Printf("Starting container...\n")
	if err := i.dockerClient.ContainerStart(containerID); err != nil {
		// Cleanup on failure
		networkMgr.DisconnectContainer("doku-network", containerName, true)
		i.dockerClient.ContainerRemove(containerName, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Build service URL
	serviceURL := i.buildServiceURL(instanceName)

	// Save environment to env file
	envMgr := envfile.NewManager(i.configMgr.GetDokuDir())
	envPath := envMgr.GetServiceEnvPath(instanceName, "")
	if err := envMgr.Save(envPath, env); err != nil {
		// Cleanup on failure
		networkMgr.DisconnectContainer("doku-network", containerName, true)
		i.dockerClient.ContainerRemove(containerName, true)
		return nil, fmt.Errorf("failed to save environment file: %w", err)
	}

	// Create instance record (Environment field kept for backward compatibility but not primary source)
	instance := &types.Instance{
		Name:             instanceName,
		ServiceType:      opts.ServiceName,
		Version:          version,
		Status:           types.StatusRunning,
		ContainerName:    containerName,
		ContainerID:      containerID, // Phase 3: Added for consistency
		IsMultiContainer: false,       // Phase 3: Single-container
		URL:              serviceURL,
		ConnectionString: i.buildConnectionString(instanceName, spec, env),
		Environment:      env, // Kept for backward compatibility during migration
		Volumes:          opts.Volumes,
		Resources: types.ResourceConfig{
			MemoryLimit: memoryLimit,
			CPULimit:    cpuLimit,
		},
		Network: types.NetworkConfig{
			Name:         "doku-network",
			InternalPort: spec.Port,
			PortMappings: opts.PortMappings,
		},
		Traefik: types.TraefikInstanceConfig{
			Enabled:   true,
			Subdomain: instanceName,
			Port:      spec.Port,
			Protocol:  spec.Protocol,
		},
	}

	// Save instance to config
	if err := i.configMgr.AddInstance(instance); err != nil {
		return nil, fmt.Errorf("failed to save instance: %w", err)
	}

	// Add DNS entry if automatic DNS setup is enabled
	if err := i.updateDNS(instanceName); err != nil {
		// Don't fail installation if DNS update fails, just warn
		color.Yellow("⚠️  Failed to add DNS entry: %v", err)
		color.Yellow("You may need to manually add: 127.0.0.1 %s.%s", instanceName, i.domain)
	}

	return instance, nil
}

// generateInstanceName generates a unique instance name
func (i *Installer) generateInstanceName(serviceName, version string) (string, error) {
	baseName := serviceName
	if version != "" && version != "latest" {
		baseName = fmt.Sprintf("%s-%s", serviceName, strings.ReplaceAll(version, ".", "-"))
	}

	// Check if base name is available
	if !i.configMgr.HasInstance(baseName) {
		return baseName, nil
	}

	// Try with incrementing suffix
	for num := 2; num <= 100; num++ {
		name := fmt.Sprintf("%s-%d", baseName, num)
		if !i.configMgr.HasInstance(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("could not generate unique instance name")
}

// mergeEnvironment merges default and override environment variables
func (i *Installer) mergeEnvironment(defaults, overrides map[string]string) map[string]string {
	env := make(map[string]string)

	// Copy defaults
	for k, v := range defaults {
		env[k] = v
	}

	// Apply overrides
	for k, v := range overrides {
		env[k] = v
	}

	return env
}

// envMapToSlice converts environment map to slice for Docker
func (i *Installer) envMapToSlice(env map[string]string) []string {
	slice := make([]string, 0, len(env))
	for k, v := range env {
		slice = append(slice, fmt.Sprintf("%s=%s", k, v))
	}
	return slice
}

// generateLabels generates Traefik and management labels
func (i *Installer) generateLabels(instanceName string, service *types.CatalogService, spec *types.ServiceSpec, internal bool) map[string]string {
	labels := make(map[string]string)

	// Management labels (always added)
	labels["managed-by"] = "doku"
	labels["doku.service"] = service.Name
	labels["doku.instance"] = instanceName
	labels["doku.version"] = spec.Image

	// Traefik labels for HTTP routing (only if NOT internal)
	if !internal && (spec.Protocol == "http" || spec.Protocol == "https") {
		routerName := fmt.Sprintf("doku-%s", instanceName)
		labels["traefik.enable"] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.rule", routerName)] = fmt.Sprintf("Host(`%s.%s`)", instanceName, i.domain)
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName)] = "web,websecure"
		labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName)] = fmt.Sprintf("%d", spec.Port)

		// Enable TLS if using HTTPS
		if i.protocol == "https" {
			labels[fmt.Sprintf("traefik.http.routers.%s.tls", routerName)] = "true"
		}
	} else if internal {
		// Explicitly disable Traefik for internal services
		labels["traefik.enable"] = "false"
	}

	// Add monitoring labels
	cfg, _ := i.configMgr.Get()
	if cfg.Monitoring.Enabled && cfg.Monitoring.Tool != "none" {
		monitoringLabels := monitoring.GetServiceLabels(instanceName, &cfg.Monitoring)
		for k, v := range monitoringLabels {
			labels[k] = v
		}
	}

	return labels
}

// createMounts creates volume mounts
func (i *Installer) createMounts(instanceName string, spec *types.ServiceSpec, customVolumes map[string]string) []mount.Mount {
	mounts := []mount.Mount{}

	// Create named volumes for each spec volume
	for idx, volumePath := range spec.Volumes {
		// Check if this is a bind mount (contains ":")
		if strings.Contains(volumePath, ":") {
			parts := strings.Split(volumePath, ":")
			if len(parts) >= 2 {
				source := parts[0]
				target := parts[1]
				readOnly := len(parts) == 3 && parts[2] == "ro"

				// Substitute ${CATALOG_DIR} placeholder
				catalogDir := i.catalogMgr.GetCatalogDir()
				serviceName := strings.Split(instanceName, "-")[0] // Extract service name from instance

				// Determine service category based on service name
				serviceCategory := "database" // default
				if strings.Contains(instanceName, "clickhouse") {
					serviceCategory = "database"
				}

				serviceVersionDir := fmt.Sprintf("%s/services/%s/%s/versions/latest", catalogDir, serviceCategory, serviceName)
				source = strings.ReplaceAll(source, "${CATALOG_DIR}", serviceVersionDir)

				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   source,
					Target:   target,
					ReadOnly: readOnly,
				})
			}
		} else {
			volumeName := docker.GenerateVolumeName(instanceName, fmt.Sprintf("%s-%d", volumePath, idx))

			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: volumePath,
			})
		}
	}

	// Add custom volume mounts
	for hostPath, containerPath := range customVolumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}

	return mounts
}

// applyResourceLimits applies CPU and memory limits
func (i *Installer) applyResourceLimits(hostConfig *dockerTypes.HostConfig, memoryLimit, cpuLimit string) error {
	if memoryLimit != "" {
		memBytes, err := docker.ParseMemoryString(memoryLimit)
		if err != nil {
			return fmt.Errorf("invalid memory limit: %w", err)
		}
		hostConfig.Resources.Memory = memBytes
	}

	if cpuLimit != "" {
		cpuQuota, cpuPeriod, err := docker.ParseCPUString(cpuLimit)
		if err != nil {
			return fmt.Errorf("invalid CPU limit: %w", err)
		}
		hostConfig.Resources.CPUQuota = cpuQuota
		hostConfig.Resources.CPUPeriod = cpuPeriod
	}

	return nil
}

// buildServiceURL builds the service access URL
func (i *Installer) buildServiceURL(instanceName string) string {
	return fmt.Sprintf("%s://%s.%s", i.protocol, instanceName, i.domain)
}

// buildConnectionString builds a connection string for the service
func (i *Installer) buildConnectionString(instanceName string, spec *types.ServiceSpec, env map[string]string) string {
	// For HTTP services, return URL
	if spec.Protocol == "http" || spec.Protocol == "https" {
		return i.buildServiceURL(instanceName)
	}

	// For TCP services, return connection info
	// This is simplified - real implementation would be service-specific
	return fmt.Sprintf("%s:%d", instanceName, spec.Port)
}

// createExposedPorts creates exposed ports for the container
func (i *Installer) createExposedPorts(portMappings map[string]string) nat.PortSet {
	if len(portMappings) == 0 {
		// No port mapping requested
		return nil
	}

	portSet := nat.PortSet{}
	for containerPortStr := range portMappings {
		containerPortSpec := nat.Port(fmt.Sprintf("%s/tcp", containerPortStr))
		portSet[containerPortSpec] = struct{}{}
	}

	return portSet
}

// createPortBindings creates port bindings for container-to-host port mapping
func (i *Installer) createPortBindings(portMappings map[string]string) nat.PortMap {
	if len(portMappings) == 0 {
		// No port mapping requested
		return nil
	}

	portMap := nat.PortMap{}
	for containerPortStr, hostPortStr := range portMappings {
		containerPortSpec := nat.Port(fmt.Sprintf("%s/tcp", containerPortStr))
		portMap[containerPortSpec] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPortStr,
			},
		}
	}

	return portMap
}

// updateDNS adds DNS entry for the service if automatic DNS setup is enabled
func (i *Installer) updateDNS(instanceName string) error {
	// Get config to check DNS setup preference
	cfg, err := i.configMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Only add DNS if automatic setup is enabled
	if cfg.Preferences.DNSSetup != "hosts" {
		return nil // Not an error, just skip
	}

	// Import dns package
	dnsMgr := dns.NewManager()

	// Add DNS entry for this service
	if err := dnsMgr.AddServiceDomain(instanceName, i.domain); err != nil {
		return err
	}

	fmt.Printf("✓ Added %s.%s to /etc/hosts\n", instanceName, i.domain)
	return nil
}

// ExistingData holds information about existing volumes and env files
type ExistingData struct {
	Volumes      []string // List of existing volume names
	EnvFiles     []string // List of existing env file paths
	HasData      bool     // True if any existing data was found
	EnvVars      map[string]string // Existing env variables (if env file exists)
}

// checkExistingData checks for existing volumes and env files for an instance
func (i *Installer) checkExistingData(instanceName string) (*ExistingData, error) {
	ctx := context.Background()
	data := &ExistingData{
		EnvVars: make(map[string]string),
	}

	// Check for existing volumes with prefix "doku-<instanceName>-"
	volumePrefix := fmt.Sprintf("doku-%s-", instanceName)
	volumes, err := i.dockerClient.ListVolumesByPrefix(ctx, volumePrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, vol := range volumes {
		data.Volumes = append(data.Volumes, vol.Name)
	}

	// Check for existing env files (both single-container and multi-container)
	envMgr := envfile.NewManager(i.configMgr.GetDokuDir())

	// Check main env file for single-container services
	envPath := envMgr.GetServiceEnvPath(instanceName, "")
	if envMgr.Exists(envPath) {
		data.EnvFiles = append(data.EnvFiles, envPath)
		// Load existing env vars for single-container merge
		if existingEnv, err := envMgr.Load(envPath); err == nil {
			data.EnvVars = existingEnv
		}
	}

	// Check for multi-container env files (pattern: <instanceName>-<container>.env)
	multiContainerEnvFiles := envMgr.FindEnvFilesByPrefix(instanceName)
	for _, envFile := range multiContainerEnvFiles {
		if envFile != envPath { // Avoid duplicates
			data.EnvFiles = append(data.EnvFiles, envFile)
		}
	}

	data.HasData = len(data.Volumes) > 0 || len(data.EnvFiles) > 0
	return data, nil
}

// promptForExistingData prompts the user about what to do with existing data
// Returns: reuseData (true = reuse, false = delete), shouldProceed (false = cancel)
func (i *Installer) promptForExistingData(instanceName string, data *ExistingData) (reuseData bool, shouldProceed bool) {
	fmt.Println()
	color.Yellow("⚠️  Existing data found for '%s':", instanceName)
	fmt.Println()

	if len(data.Volumes) > 0 {
		fmt.Printf("  Docker volumes (%d):\n", len(data.Volumes))
		for _, vol := range data.Volumes {
			fmt.Printf("    • %s\n", vol)
		}
	}

	if len(data.EnvFiles) > 0 {
		fmt.Printf("  Environment files (%d):\n", len(data.EnvFiles))
		for _, envFile := range data.EnvFiles {
			fmt.Printf("    • %s\n", envFile)
		}
	}

	fmt.Println()
	fmt.Println("What would you like to do?")
	fmt.Println("  [R] Reuse existing data (recommended)")
	fmt.Println("  [D] Delete and start fresh")
	fmt.Println("  [C] Cancel installation")
	fmt.Println()
	fmt.Print("Choice [R/d/c]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "", "r", "reuse":
		return true, true
	case "d", "delete":
		return false, true
	case "c", "cancel":
		return false, false
	default:
		// Default to reuse for safety
		return true, true
	}
}

// deleteExistingData removes existing volumes and env files
func (i *Installer) deleteExistingData(instanceName string, data *ExistingData) error {
	ctx := context.Background()

	// Delete volumes
	for _, vol := range data.Volumes {
		fmt.Printf("  Deleting volume %s...\n", vol)
		if err := i.dockerClient.RemoveVolume(ctx, vol); err != nil {
			// Warn but don't fail - volume might be in use
			color.Yellow("    Warning: Could not delete volume %s: %v", vol, err)
		} else {
			color.Green("    ✓ Deleted %s", vol)
		}
	}

	// Delete env files
	envMgr := envfile.NewManager(i.configMgr.GetDokuDir())
	for _, envFile := range data.EnvFiles {
		fmt.Printf("  Deleting %s...\n", envFile)
		if err := envMgr.Delete(envFile); err != nil {
			color.Yellow("    Warning: Could not delete %s: %v", envFile, err)
		} else {
			color.Green("    ✓ Deleted %s", envFile)
		}
	}

	fmt.Println()
	return nil
}

// mergeWithExistingEnv merges new environment with existing, keeping existing values
func (i *Installer) mergeWithExistingEnv(existingEnv, newEnv map[string]string) map[string]string {
	merged := make(map[string]string)

	// Start with new (catalog default) environment
	for k, v := range newEnv {
		merged[k] = v
	}

	// Override with existing values (user's data takes precedence)
	for k, v := range existingEnv {
		merged[k] = v
	}

	return merged
}
