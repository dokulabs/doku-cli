package service

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/pkg/types"
	dockerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
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
	Internal     bool              // If true, don't expose via Traefik
}

// Install installs a service from the catalog
func (i *Installer) Install(opts InstallOptions) (*types.Instance, error) {
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

	// Validate instance name doesn't exist
	if i.configMgr.HasInstance(instanceName) {
		return nil, fmt.Errorf("instance '%s' already exists", instanceName)
	}

	// Merge environment variables
	env := i.mergeEnvironment(spec.Environment, opts.Environment)

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

	// Pull image
	fmt.Printf("Pulling image %s...\n", spec.Image)
	if err := i.dockerClient.ImagePull(spec.Image); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container configuration
	containerConfig := &dockerTypes.Config{
		Image:  spec.Image,
		Env:    i.envMapToSlice(env),
		Labels: i.generateLabels(instanceName, service, spec, opts.Internal),
	}

	// Create host configuration
	hostConfig := &dockerTypes.HostConfig{
		RestartPolicy: dockerTypes.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: i.createMounts(instanceName, spec, opts.Volumes),
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

	// Connect to doku-network
	networkMgr := docker.NewNetworkManager(i.dockerClient)
	if err := networkMgr.ConnectContainer("doku-network", containerName); err != nil {
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

	// Create instance record
	instance := &types.Instance{
		Name:             instanceName,
		ServiceType:      opts.ServiceName,
		Version:          version,
		Status:           types.StatusRunning,
		ContainerName:    containerName,
		URL:              serviceURL,
		ConnectionString: i.buildConnectionString(instanceName, spec, env),
		Environment:      env,
		Volumes:          opts.Volumes,
		Resources: types.ResourceConfig{
			MemoryLimit: memoryLimit,
			CPULimit:    cpuLimit,
		},
		Network: types.NetworkConfig{
			Name:         "doku-network",
			InternalPort: spec.Port,
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

	return labels
}

// createMounts creates volume mounts
func (i *Installer) createMounts(instanceName string, spec *types.ServiceSpec, customVolumes map[string]string) []mount.Mount {
	mounts := []mount.Mount{}

	// Create named volumes for each spec volume
	for _, volumePath := range spec.Volumes {
		volumeName := docker.GenerateVolumeName(instanceName, volumePath)

		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: volumePath,
		})
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
