package project

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
)

// Runner handles running project containers
type Runner struct {
	docker    *docker.Client
	configMgr *config.Manager
}

// ContainerRunOptions contains options for running a container
type ContainerRunOptions struct {
	Project *types.Project
	Image   string
	Detach  bool
}

// NewRunner creates a new container runner
func NewRunner(dockerClient *docker.Client, cfgMgr *config.Manager) *Runner {
	return &Runner{
		docker:    dockerClient,
		configMgr: cfgMgr,
	}
}

// Run runs a project container
func (r *Runner) Run(opts ContainerRunOptions) error {
	// Get config
	cfg, err := r.configMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Check dependencies
	if len(opts.Project.Dependencies) > 0 {
		missing, err := r.checkDependencies(opts.Project.Dependencies)
		if err != nil {
			return err
		}

		if len(missing) > 0 {
			return fmt.Errorf("missing dependencies: %s. Run with --install-deps to install automatically", strings.Join(missing, ", "))
		}
	}

	// Prepare port bindings
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	if opts.Project.Port > 0 {
		containerPort := nat.Port(fmt.Sprintf("%d/tcp", opts.Project.Port))
		exposedPorts[containerPort] = struct{}{}

		// Don't bind to host port if not internal and using Traefik
		if opts.Project.URL == "" {
			// Internal or no URL - bind to host
			portBindings[containerPort] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", opts.Project.Port),
				},
			}
		}
	}

	// Parse additional port mappings from environment
	if portsEnv, exists := opts.Project.Environment["DOKU_PORTS"]; exists {
		ports := strings.Split(portsEnv, ",")
		for _, portMapping := range ports {
			parts := strings.Split(portMapping, ":")
			if len(parts) == 2 {
				hostPort := parts[0]
				containerPort := nat.Port(parts[1] + "/tcp")
				exposedPorts[containerPort] = struct{}{}
				portBindings[containerPort] = []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: hostPort,
					},
				}
			}
		}
	}

	// Prepare environment variables
	env := []string{}
	for key, value := range opts.Project.Environment {
		if !strings.HasPrefix(key, "DOKU_") { // Skip internal Doku variables
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Inject dependency connection strings
	if err := r.injectDependencyEnvVars(opts.Project, &env); err != nil {
		return err
	}

	// Prepare Traefik labels
	labels := map[string]string{
		"doku.managed": "true",
		"doku.type":    "project",
		"doku.name":    opts.Project.Name,
	}

	// Add Traefik labels if project has a URL
	if opts.Project.URL != "" {
		domain := strings.TrimPrefix(opts.Project.URL, "https://")
		domain = strings.TrimPrefix(domain, "http://")

		labels["traefik.enable"] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.rule", opts.Project.Name)] = fmt.Sprintf("Host(`%s`)", domain)
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", opts.Project.Name)] = "websecure"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", opts.Project.Name)] = "true"
		labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", opts.Project.Name)] = fmt.Sprintf("%d", opts.Project.Port)
	}

	// Container config
	containerConfig := &container.Config{
		Image:        opts.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       labels,
	}

	// Host config
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Network config - connect to Doku network
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			cfg.Network.Name: {
				Aliases: []string{opts.Project.Name},
			},
		},
	}

	// Remove existing container if present
	if err := r.docker.ContainerRemove(opts.Project.ContainerName, true); err != nil {
		// Only show warning if it's not a "container not found" error
		if !strings.Contains(err.Error(), "No such container") {
			fmt.Printf("Warning: failed to remove existing container: %v\n", err)
		}
	}

	// Create container
	containerID, err := r.docker.ContainerCreate(
		containerConfig,
		hostConfig,
		networkConfig,
		opts.Project.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := r.docker.ContainerStart(containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Update project with container ID
	if err := r.configMgr.Update(func(c *types.Config) error {
		if proj, exists := c.Projects[opts.Project.Name]; exists {
			proj.ContainerID = containerID
			proj.Status = types.StatusRunning
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update project config: %w", err)
	}

	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	green.Println("✓ Project started successfully")
	fmt.Println()

	if opts.Project.URL != "" {
		fmt.Println("Access your project:")
		cyan.Printf("  URL: %s\n", opts.Project.URL)
	} else {
		fmt.Println("Project is running:")
		cyan.Printf("  Container: %s\n", opts.Project.ContainerName)
		if opts.Project.Port > 0 {
			cyan.Printf("  Port: http://localhost:%d\n", opts.Project.Port)
		}
	}
	fmt.Println()

	return nil
}

// InstallDependencies installs missing project dependencies
func (r *Runner) InstallDependencies(project *types.Project) error {
	if len(project.Dependencies) == 0 {
		return nil
	}

	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	// Check which dependencies are missing
	missing, err := r.checkDependencies(project.Dependencies)
	if err != nil {
		return err
	}

	if len(missing) == 0 {
		green.Println("✓ All dependencies are already installed")
		return nil
	}

	fmt.Println()
	cyan.Printf("→ Installing %d missing dependencies...\n", len(missing))
	fmt.Println()

	// Create catalog manager
	catalogMgr := catalog.NewManager(r.configMgr.GetCatalogDir())

	// Create installer
	installer, err := service.NewInstaller(r.docker, r.configMgr, catalogMgr)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Install each missing dependency
	for _, dep := range missing {
		parts := strings.Split(dep, ":")
		serviceName := parts[0]
		version := ""
		if len(parts) > 1 {
			version = parts[1]
		}

		fmt.Printf("Installing %s...\n", dep)

		// Check if service exists in catalog
		_, err := catalogMgr.GetService(serviceName)
		if err != nil {
			return fmt.Errorf("service '%s' not found in catalog", serviceName)
		}

		// Install service
		opts := service.InstallOptions{
			ServiceName:      serviceName,
			Version:          version,
			InstanceName:     serviceName, // Use service name as instance name
			AutoInstallDeps:  true,
			SkipDependencies: false,
		}

		if _, err := installer.Install(opts); err != nil {
			return fmt.Errorf("failed to install %s: %w", dep, err)
		}

		green.Printf("  ✓ Installed %s\n", dep)
	}

	fmt.Println()
	green.Println("✓ All dependencies installed successfully")
	fmt.Println()

	return nil
}

// checkDependencies checks which dependencies are not installed
func (r *Runner) checkDependencies(dependencies []string) ([]string, error) {
	cfg, err := r.configMgr.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	missing := []string{}

	for _, dep := range dependencies {
		// Parse dependency (format: service:version or just service)
		parts := strings.Split(dep, ":")
		serviceName := parts[0]

		// Check if service is installed
		if _, exists := cfg.Instances[serviceName]; !exists {
			missing = append(missing, dep)
		}
	}

	return missing, nil
}

// injectDependencyEnvVars injects connection string environment variables for dependencies
func (r *Runner) injectDependencyEnvVars(project *types.Project, env *[]string) error {
	if len(project.Dependencies) == 0 {
		return nil
	}

	cfg, err := r.configMgr.Get()
	if err != nil {
		return err
	}

	for _, dep := range project.Dependencies {
		parts := strings.Split(dep, ":")
		serviceName := parts[0]

		// Get instance
		instance, exists := cfg.Instances[serviceName]
		if !exists {
			continue // Skip if not installed
		}

		// Inject connection string based on service type
		switch instance.ServiceType {
		case "postgres", "postgresql":
			connStr := fmt.Sprintf("postgresql://postgres@%s.%s:5432", serviceName, cfg.Preferences.Domain)
			*env = append(*env, fmt.Sprintf("DATABASE_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("POSTGRES_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("DB_HOST=%s.%s", serviceName, cfg.Preferences.Domain))
			*env = append(*env, fmt.Sprintf("DB_PORT=5432"))

		case "mysql":
			connStr := fmt.Sprintf("mysql://root@%s.%s:3306", serviceName, cfg.Preferences.Domain)
			*env = append(*env, fmt.Sprintf("DATABASE_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("MYSQL_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("DB_HOST=%s.%s", serviceName, cfg.Preferences.Domain))
			*env = append(*env, fmt.Sprintf("DB_PORT=3306"))

		case "redis":
			connStr := fmt.Sprintf("redis://%s.%s:6379", serviceName, cfg.Preferences.Domain)
			*env = append(*env, fmt.Sprintf("REDIS_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("REDIS_HOST=%s.%s", serviceName, cfg.Preferences.Domain))
			*env = append(*env, fmt.Sprintf("REDIS_PORT=6379"))

		case "rabbitmq":
			connStr := fmt.Sprintf("amqp://guest:guest@%s.%s:5672", serviceName, cfg.Preferences.Domain)
			*env = append(*env, fmt.Sprintf("RABBITMQ_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("AMQP_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("RABBITMQ_HOST=%s.%s", serviceName, cfg.Preferences.Domain))

		case "mongodb":
			connStr := fmt.Sprintf("mongodb://%s.%s:27017", serviceName, cfg.Preferences.Domain)
			*env = append(*env, fmt.Sprintf("MONGODB_URL=%s", connStr))
			*env = append(*env, fmt.Sprintf("MONGO_URL=%s", connStr))
		}
	}

	return nil
}

// PromptInstallDependencies prompts user to install missing dependencies
func (r *Runner) PromptInstallDependencies(project *types.Project) (bool, error) {
	missing, err := r.checkDependencies(project.Dependencies)
	if err != nil {
		return false, err
	}

	if len(missing) == 0 {
		return false, nil
	}

	yellow := color.New(color.FgYellow)
	yellow.Printf("\n⚠️  Project requires the following services:\n")
	for _, dep := range missing {
		fmt.Printf("  • %s\n", dep)
	}
	fmt.Println()

	installDeps := false
	prompt := &survey.Confirm{
		Message: "Would you like to install these dependencies now?",
		Default: true,
	}

	if err := survey.AskOne(prompt, &installDeps); err != nil {
		return false, err
	}

	return installDeps, nil
}
