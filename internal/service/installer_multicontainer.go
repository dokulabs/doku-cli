package service

import (
	"fmt"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/dokulabs/doku-cli/internal/dependencies"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/monitoring"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
)

// Phase 3: Multi-Container & Dependency Management Methods

// resolveDependencies resolves and installs dependencies for a service
func (i *Installer) resolveDependencies(opts InstallOptions) error {
	// Create dependency resolver
	resolver := dependencies.NewResolver(i.catalogMgr, i.configMgr)

	// Resolve dependencies
	result, err := resolver.Resolve(opts.ServiceName, opts.Version)
	if err != nil {
		if dependencies.IsCircularDependency(err) {
			return fmt.Errorf("circular dependency detected: %w\nPlease fix the catalog configuration", err)
		}
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Get missing dependencies
	missing := resolver.GetMissingDependencies(result)
	if len(missing) == 0 {
		// All dependencies already installed
		return nil
	}

	// Show dependency tree
	fmt.Println()
	color.Cyan("📦 Dependencies required:")
	for _, dep := range missing {
		fmt.Printf("  • %s (%s)\n", dep.ServiceName, dep.Version)
	}
	fmt.Println()

	// Auto-install or prompt
	if !opts.AutoInstallDeps {
		// In interactive mode, we would ask user for confirmation here
		// For now, assume yes if we reach this point
		color.Cyan("Installing dependencies automatically...")
	}

	// Install each dependency in order
	for _, dep := range missing {
		// Skip the root service itself (it will be installed by the main Install call)
		if dep.ServiceName == opts.ServiceName {
			continue
		}

		if !dep.IsInstalled && dep.Required {
			fmt.Println()
			color.Cyan("Installing dependency: %s...", dep.ServiceName)

			depOpts := InstallOptions{
				ServiceName:      dep.ServiceName,
				Version:          dep.Version,
				InstanceName:     dep.ServiceName, // Use service name as instance name
				Environment:      dep.Environment,
				Internal:         true,  // Dependencies are internal by default
				SkipDependencies: false, // Allow nested dependencies
				AutoInstallDeps:  true,  // Auto-install nested deps
				IsDepend:         true,  // Mark as dependency installation
			}

			if _, err := i.Install(depOpts); err != nil {
				return fmt.Errorf("failed to install dependency %s: %w", dep.ServiceName, err)
			}

			color.Green("✓ %s installed", dep.ServiceName)
		}
	}

	fmt.Println()
	return nil
}

// installMultiContainer installs a multi-container service
func (i *Installer) installMultiContainer(
	opts InstallOptions,
	spec *types.ServiceSpec,
	instanceName string,
	version string,
) (*types.Instance, error) {
	fmt.Println()
	color.Cyan("Installing multi-container service: %s", instanceName)
	fmt.Printf("  Containers: %d\n", len(spec.Containers))
	fmt.Println()

	cfg, _ := i.configMgr.Get()

	// Create instance
	instance := &types.Instance{
		Name:             instanceName,
		ServiceType:      opts.ServiceName,
		Version:          version,
		IsMultiContainer: true,
		Containers:       make([]types.ContainerInfo, 0, len(spec.Containers)),
		Dependencies:     spec.GetDependencyNames(),
		Status:           "creating",
		Environment:      opts.Environment,
	}

	// Find primary container
	primaryContainer := spec.GetPrimaryContainer()
	if primaryContainer == nil {
		return nil, fmt.Errorf("no primary container defined")
	}

	// Run init containers (migrations, setup scripts, etc.)
	if len(spec.InitContainers) > 0 {
		if err := i.runInitContainers(spec, instanceName); err != nil {
			return nil, fmt.Errorf("failed to run init containers: %w", err)
		}
	}

	// Install each container
	for idx, containerSpec := range spec.Containers {
		isPrimary := (primaryContainer != nil && containerSpec.Name == primaryContainer.Name)

		fmt.Printf("Creating container: %s", containerSpec.Name)
		if isPrimary {
			fmt.Printf(" (primary)")
		}
		fmt.Println("...")

		// Build full container name
		containerName := i.buildMultiContainerName(instanceName, containerSpec.Name)

		// Merge environment variables (service-level → container-level → user overrides)
		env := i.mergeEnvironment(spec.Environment, containerSpec.Environment)
		env = i.mergeEnvironment(env, opts.Environment)

		// Add monitoring instrumentation
		if cfg.Monitoring.Enabled && cfg.Monitoring.Tool != "none" {
			monitoringEnv := monitoring.GetInstrumentationEnv(instanceName, &cfg.Monitoring)
			env = i.mergeEnvironment(env, monitoringEnv)
		}

		// Pull image
		if err := i.dockerClient.ImagePull(containerSpec.Image); err != nil {
			i.cleanupMultiContainerInstall(instance)
			return nil, fmt.Errorf("failed to pull image %s: %w", containerSpec.Image, err)
		}

		// Determine the port for this container (for Traefik routing)
		containerPort := 0
		if isPrimary && len(containerSpec.Ports) > 0 {
			// Extract the internal port from the first port mapping (e.g., "3301:3301" -> 3301)
			portMapping := containerSpec.Ports[0]
			if colonIdx := strings.Index(portMapping, ":"); colonIdx > 0 {
				fmt.Sscanf(portMapping[colonIdx+1:], "%d", &containerPort)
			}
		}
		// Fallback to service-level port if available
		if containerPort == 0 && isPrimary {
			containerPort = spec.Port
		}

		// Create container configuration
		containerConfig := &dockerTypes.Config{
			Image:  containerSpec.Image,
			Env:    i.envMapToSlice(env),
			Labels: i.generateMultiContainerLabels(instanceName, opts.ServiceName, containerSpec.Name, isPrimary, opts.Internal, containerPort),
		}

		// Override command/entrypoint if specified
		if len(containerSpec.Command) > 0 {
			containerConfig.Cmd = containerSpec.Command
		}
		if len(containerSpec.Entrypoint) > 0 {
			containerConfig.Entrypoint = containerSpec.Entrypoint
		}

		// Create host configuration
		hostConfig := &dockerTypes.HostConfig{
			RestartPolicy: dockerTypes.RestartPolicy{
				Name: "unless-stopped",
			},
			Mounts:    i.createMultiContainerMounts(instanceName, containerSpec),
			LogConfig: *monitoring.GetDockerLoggingConfig(&cfg.Monitoring),
		}

		// Apply resource limits
		if containerSpec.Resources != nil {
			memLimit := containerSpec.Resources.MemoryMax
			cpuLimit := containerSpec.Resources.CPUMax
			if err := i.applyResourceLimits(hostConfig, memLimit, cpuLimit); err != nil {
				i.cleanupMultiContainerInstall(instance)
				return nil, fmt.Errorf("failed to apply resource limits: %w", err)
			}
		}

		// Create container
		containerID, err := i.dockerClient.ContainerCreate(
			containerConfig,
			hostConfig,
			nil,
			containerName,
		)
		if err != nil {
			i.cleanupMultiContainerInstall(instance)
			return nil, fmt.Errorf("failed to create container %s: %w", containerSpec.Name, err)
		}

		// Connect to doku-network with aliases BEFORE starting
		networkMgr := docker.NewNetworkManager(i.dockerClient)
		aliases := i.buildNetworkAliases(instanceName, containerSpec.Name, isPrimary)
		if err := networkMgr.ConnectContainerWithAliases("doku-network", containerID, aliases); err != nil {
			i.cleanupMultiContainerInstall(instance)
			return nil, fmt.Errorf("failed to connect container %s to network: %w", containerSpec.Name, err)
		}

		// Add to instance
		instance.Containers = append(instance.Containers, types.ContainerInfo{
			Name:        containerSpec.Name,
			ContainerID: containerID,
			FullName:    containerName,
			Primary:     isPrimary,
			Status:      "created",
			Ports:       containerSpec.Ports,
			Image:       containerSpec.Image,
		})

		color.Green("✓ Container %s created", containerSpec.Name)

		// Brief pause between containers
		if idx < len(spec.Containers)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println()
	color.Cyan("Starting containers in dependency order...")
	fmt.Println()

	// Start all containers in correct order
	if err := i.startMultiContainerService(instance, spec); err != nil {
		i.cleanupMultiContainerInstall(instance)
		return nil, fmt.Errorf("failed to start containers: %w", err)
	}

	// Set instance URL (based on primary container)
	if !opts.Internal {
		instance.URL = i.buildServiceURL(instanceName)
	}

	// Update instance status
	instance.Status = types.StatusRunning

	// Save instance to config
	if err := i.configMgr.AddInstance(instance); err != nil {
		i.cleanupMultiContainerInstall(instance)
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

// buildMultiContainerName builds the full container name for multi-container services
func (i *Installer) buildMultiContainerName(instanceName, containerName string) string {
	return fmt.Sprintf("doku-%s-%s", instanceName, containerName)
}

// buildNetworkAliases creates network aliases for a container
func (i *Installer) buildNetworkAliases(instanceName, containerName string, isPrimary bool) []string {
	// Extract base service name (remove numeric suffix if present)
	serviceName := instanceName
	if strings.Contains(instanceName, "-") {
		parts := strings.Split(instanceName, "-")
		// Check if last part is numeric
		if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", new(int)); err == nil && len(parts) > 1 {
			serviceName = strings.Join(parts[:len(parts)-1], "-")
		}
	}

	aliases := []string{
		fmt.Sprintf("doku-%s-%s", instanceName, containerName), // Full doku name
		fmt.Sprintf("%s-%s", serviceName, containerName),       // Service-container name (e.g., signoz-query-service)
		containerName, // Short name (for intra-service communication)
	}

	// Primary container gets service-level alias
	if isPrimary {
		aliases = append(aliases, instanceName) // Instance name alias
		if serviceName != instanceName {
			aliases = append(aliases, serviceName) // Service name alias
		}
	}

	return aliases
}

// generateMultiContainerLabels generates Docker labels for multi-container services
func (i *Installer) generateMultiContainerLabels(instanceName, serviceName, containerName string, isPrimary bool, internal bool, port int) map[string]string {
	labels := map[string]string{
		"doku.managed":   "true",
		"doku.service":   serviceName,
		"doku.instance":  instanceName,
		"doku.container": containerName,
		"doku.primary":   fmt.Sprintf("%t", isPrimary),
		"doku.multi":     "true",
	}

	if !internal && isPrimary && port > 0 {
		labels["traefik.enable"] = "true"
		labels["traefik.http.routers."+instanceName+".rule"] = fmt.Sprintf("Host(`%s.%s`)", instanceName, i.domain)
		labels["traefik.http.routers."+instanceName+".entrypoints"] = "web,websecure"
		labels["traefik.http.services."+instanceName+".loadbalancer.server.port"] = fmt.Sprintf("%d", port)

		// Enable TLS if using HTTPS protocol
		if i.protocol == "https" {
			labels["traefik.http.routers."+instanceName+".tls"] = "true"
		}
	}

	return labels
}

// createMultiContainerMounts creates volume mounts for multi-container services
func (i *Installer) createMultiContainerMounts(instanceName string, containerSpec types.ContainerSpec) []mount.Mount {
	var mounts []mount.Mount

	for idx, vol := range containerSpec.Volumes {
		// Check if this is a bind mount (contains ":")
		if strings.Contains(vol, ":") {
			parts := strings.Split(vol, ":")
			if len(parts) >= 2 {
				source := parts[0]
				target := parts[1]
				readOnly := len(parts) == 3 && parts[2] == "ro"

				// Substitute ${CATALOG_DIR} placeholder
				catalogDir := i.catalogMgr.GetCatalogDir()
				serviceName := strings.Split(instanceName, "-")[0] // Extract service name
				serviceVersionDir := fmt.Sprintf("%s/services/%s/%s/versions/latest", catalogDir, containerSpec.Name, serviceName)

				// For SignOz, use monitoring/signoz path
				if strings.Contains(instanceName, "signoz") {
					serviceVersionDir = fmt.Sprintf("%s/services/monitoring/signoz/versions/latest", catalogDir)
				}

				source = strings.ReplaceAll(source, "${CATALOG_DIR}", serviceVersionDir)

				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   source,
					Target:   target,
					ReadOnly: readOnly,
				})
			}
		} else {
			// Use named volumes for simple volume paths
			volumeName := fmt.Sprintf("doku-%s-%s-%d", instanceName, containerSpec.Name, idx)
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: vol,
			})
		}
	}

	return mounts
}

// startMultiContainerService starts containers in dependency order
func (i *Installer) startMultiContainerService(instance *types.Instance, spec *types.ServiceSpec) error {
	// Build internal dependency graph for containers
	depGraph := make(map[string][]string)
	for _, containerSpec := range spec.Containers {
		// Filter to only internal dependencies (same service)
		internalDeps := make([]string, 0)
		for _, dep := range containerSpec.DependsOn {
			// Check if this is an internal dependency (exists in our containers)
			isInternal := false
			for _, c := range spec.Containers {
				if c.Name == dep {
					isInternal = true
					break
				}
			}
			if isInternal {
				internalDeps = append(internalDeps, dep)
			}
			// External dependencies are already installed by resolveDependencies
		}
		depGraph[containerSpec.Name] = internalDeps
	}

	// Topological sort for startup order
	startOrder, err := topologicalSortContainers(depGraph, spec.Containers)
	if err != nil {
		return err
	}

	// Start containers in order
	for _, containerName := range startOrder {
		// Find container info
		var containerInfo *types.ContainerInfo
		for idx := range instance.Containers {
			if instance.Containers[idx].Name == containerName {
				containerInfo = &instance.Containers[idx]
				break
			}
		}

		if containerInfo == nil {
			continue
		}

		fmt.Printf("Starting %s...\n", containerName)

		if err := i.dockerClient.ContainerStart(containerInfo.ContainerID); err != nil {
			return fmt.Errorf("failed to start %s: %w", containerName, err)
		}

		containerInfo.Status = "running"
		color.Green("✓ %s started", containerName)

		// Brief pause to let container initialize
		time.Sleep(2 * time.Second)
	}

	return nil
}

// topologicalSortContainers sorts containers by dependencies for startup order
func topologicalSortContainers(graph map[string][]string, containers []types.ContainerSpec) ([]string, error) {
	var result []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		if visiting[node] {
			return fmt.Errorf("circular dependency detected at: %s", node)
		}
		if visited[node] {
			return nil
		}

		visiting[node] = true

		// Visit dependencies first
		for _, dep := range graph[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[node] = false
		visited[node] = true
		result = append(result, node)

		return nil
	}

	// Visit all containers
	for _, container := range containers {
		if !visited[container.Name] {
			if err := visit(container.Name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// runInitContainers runs init containers in dependency order
// Init containers run once to completion (e.g., migrations, setup scripts)
func (i *Installer) runInitContainers(spec *types.ServiceSpec, instanceName string) error {
	fmt.Println()
	color.Cyan("Running init containers...")
	fmt.Println()

	// Sort init containers by dependencies
	sorted, err := i.sortInitContainers(spec.InitContainers)
	if err != nil {
		return err
	}

	// Run each init container in order
	for _, initContainer := range sorted {
		fmt.Printf("Running %s...\n", initContainer.Name)

		// Prepare command
		cmd := initContainer.Command
		if len(cmd) == 0 {
			return fmt.Errorf("init container %s has no command", initContainer.Name)
		}

		// Prepare environment
		env := make([]string, 0, len(initContainer.Environment))
		for k, v := range initContainer.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		// Run container with --rm flag (auto-remove after completion)
		containerName := fmt.Sprintf("doku-%s-init-%s", instanceName, initContainer.Name)

		// Pull image first
		if err := i.dockerClient.ImagePull(initContainer.Image); err != nil {
			return fmt.Errorf("failed to pull init container image %s: %w", initContainer.Image, err)
		}

		// Create and start the container
		containerID, err := i.dockerClient.RunContainer(
			initContainer.Image,
			containerName,
			cmd,
			env,
			"doku-network",
			true, // auto-remove after completion
		)
		if err != nil {
			return fmt.Errorf("failed to run init container %s: %w", initContainer.Name, err)
		}

		// Wait for container to complete
		if err := i.dockerClient.WaitForContainer(containerID); err != nil {
			// Get logs for debugging
			logs, _ := i.dockerClient.GetContainerLogsString(containerID)
			return fmt.Errorf("init container %s failed: %w\nLogs:\n%s", initContainer.Name, err, logs)
		}

		color.Green("✓ %s completed", initContainer.Name)
	}

	fmt.Println()
	return nil
}

// sortInitContainers sorts init containers by dependencies
func (i *Installer) sortInitContainers(initContainers []types.InitContainer) ([]types.InitContainer, error) {
	// Build dependency graph
	graph := make(map[string][]string)
	containerMap := make(map[string]types.InitContainer)

	for _, container := range initContainers {
		graph[container.Name] = container.DependsOn
		containerMap[container.Name] = container
	}

	// Topological sort
	var result []types.InitContainer
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected in init containers: %s", name)
		}
		if visited[name] {
			return nil
		}

		visiting[name] = true

		// Visit dependencies first
		for _, dep := range graph[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		result = append(result, containerMap[name])

		return nil
	}

	// Visit all containers
	for _, container := range initContainers {
		if !visited[container.Name] {
			if err := visit(container.Name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// cleanupMultiContainerInstall removes containers if installation fails
func (i *Installer) cleanupMultiContainerInstall(instance *types.Instance) {
	color.Yellow("⚠️  Installation failed, cleaning up...")

	networkMgr := docker.NewNetworkManager(i.dockerClient)

	for _, container := range instance.Containers {
		if container.ContainerID != "" {
			fmt.Printf("Removing %s...\n", container.Name)

			// Disconnect from network
			networkMgr.DisconnectContainer("doku-network", container.FullName, true)

			// Remove container
			if err := i.dockerClient.ContainerRemove(container.FullName, true); err != nil {
				color.Yellow("  Failed to remove %s: %v", container.Name, err)
			}
		}
	}

	color.Yellow("Cleanup complete")
}
