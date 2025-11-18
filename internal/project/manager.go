package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/dns"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/pkg/types"
)

// Manager handles project lifecycle operations
type Manager struct {
	docker    *docker.Client
	configMgr *config.Manager
	builder   *Builder
	runner    *Runner
}

// AddOptions contains options for adding a project
type AddOptions struct {
	ProjectPath  string            // Path to project directory
	Name         string            // Project name (optional, defaults to directory name)
	Dockerfile   string            // Path to Dockerfile (optional, defaults to ./Dockerfile)
	Port         int               // Main port to expose
	Ports        []string          // Additional port mappings
	Environment  map[string]string // Environment variables
	Dependencies []string          // Service dependencies (e.g., postgres:16)
	Domain       string            // Custom domain (optional)
	Internal     bool              // Internal only (no Traefik)
}

// BuildOptions contains options for building a project
type BuildOptions struct {
	Name    string // Project name
	NoCache bool   // Build without cache
	Pull    bool   // Pull base image before building
	Tag     string // Custom tag
}

// RunOptions contains options for running a project
type RunOptions struct {
	Name        string // Project name
	Build       bool   // Build before running
	InstallDeps bool   // Auto-install dependencies
	Detach      bool   // Run in background
}

// NewManager creates a new project manager
func NewManager(dockerClient *docker.Client, cfgMgr *config.Manager) (*Manager, error) {
	builder := NewBuilder(dockerClient)
	runner := NewRunner(dockerClient, cfgMgr)

	return &Manager{
		docker:    dockerClient,
		configMgr: cfgMgr,
		builder:   builder,
		runner:    runner,
	}, nil
}

// Add adds a new project to Doku
func (m *Manager) Add(opts AddOptions) (*types.Project, error) {
	// Validate project path exists
	absPath, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("invalid project path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path does not exist: %s", absPath)
	}

	// Determine project name
	projectName := opts.Name
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	// Validate project name
	if err := validateProjectName(projectName); err != nil {
		return nil, err
	}

	// Check if project already exists
	if _, err := m.Get(projectName); err == nil {
		return nil, fmt.Errorf("project '%s' already exists", projectName)
	}

	// Determine Dockerfile path
	dockerfilePath := opts.Dockerfile
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	// Make Dockerfile path absolute if relative
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(absPath, dockerfilePath)
	}

	// Validate Dockerfile exists
	if err := m.builder.ValidateDockerfile(dockerfilePath); err != nil {
		return nil, err
	}

	// Determine domain
	domain := opts.Domain
	isFullSubdomain := opts.Domain != "" // If explicitly provided, it's a full subdomain
	if domain == "" {
		cfg, err := m.configMgr.Get()
		if err != nil {
			return nil, err
		}
		domain = cfg.Preferences.Domain
	}

	// Create URL if not internal
	url := ""
	if !opts.Internal && opts.Port > 0 {
		if isFullSubdomain {
			// Domain was explicitly provided as full subdomain
			url = fmt.Sprintf("https://%s", domain)
		} else {
			// Domain from config (base domain), construct full subdomain
			url = fmt.Sprintf("https://%s.%s", projectName, domain)
		}
	}

	// Create project object
	project := &types.Project{
		Name:          projectName,
		Path:          absPath,
		Dockerfile:    dockerfilePath,
		Status:        types.StatusStopped,
		ContainerName: fmt.Sprintf("doku-%s", projectName),
		URL:           url,
		Port:          opts.Port,
		CreatedAt:     time.Now(),
		Dependencies:  opts.Dependencies,
		Environment:   opts.Environment,
	}

	// Add port mappings
	if len(opts.Ports) > 0 {
		if project.Environment == nil {
			project.Environment = make(map[string]string)
		}
		project.Environment["DOKU_PORTS"] = strings.Join(opts.Ports, ",")
	}

	// Save to config
	if err := m.configMgr.AddProject(project); err != nil {
		return nil, fmt.Errorf("failed to save project: %w", err)
	}

	return project, nil
}

// Get retrieves a project by name
func (m *Manager) Get(name string) (*types.Project, error) {
	return m.configMgr.GetProject(name)
}

// List returns all projects
func (m *Manager) List() ([]*types.Project, error) {
	cfg, err := m.configMgr.Get()
	if err != nil {
		return nil, err
	}

	projects := make([]*types.Project, 0, len(cfg.Projects))
	for _, project := range cfg.Projects {
		// Update status from Docker
		status, err := m.getContainerStatus(project.ContainerName)
		if err == nil {
			project.Status = status
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// Build builds a project's Docker image
func (m *Manager) Build(opts BuildOptions) error {
	project, err := m.Get(opts.Name)
	if err != nil {
		return err
	}

	// Validate Dockerfile still exists
	if err := m.builder.ValidateDockerfile(project.Dockerfile); err != nil {
		return err
	}

	// Determine image tag
	imageTag := opts.Tag
	if imageTag == "" {
		imageTag = fmt.Sprintf("doku-project-%s:latest", project.Name)
	}

	// Build options
	buildOpts := DockerBuildOptions{
		ContextPath:    project.Path,
		DockerfilePath: project.Dockerfile,
		Tags:           []string{imageTag},
		NoCache:        opts.NoCache,
		Pull:           opts.Pull,
	}

	// Execute build
	imageID, err := m.builder.Build(buildOpts)
	if err != nil {
		return err
	}

	// Update project with image info
	if err := m.configMgr.Update(func(c *types.Config) error {
		if proj, exists := c.Projects[project.Name]; exists {
			proj.Environment["DOKU_IMAGE_ID"] = imageID
			proj.Environment["DOKU_IMAGE_TAG"] = imageTag
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update project config: %w", err)
	}

	return nil
}

// Run runs a project
func (m *Manager) Run(opts RunOptions) error {
	project, err := m.Get(opts.Name)
	if err != nil {
		return err
	}

	// Build if requested or if no image exists
	imageTag := fmt.Sprintf("doku-project-%s:latest", project.Name)
	if opts.Build || !m.imageExists(imageTag) {
		buildOpts := BuildOptions{
			Name: opts.Name,
		}
		if err := m.Build(buildOpts); err != nil {
			return fmt.Errorf("failed to build project: %w", err)
		}
	}

	// Install dependencies if requested
	if opts.InstallDeps && len(project.Dependencies) > 0 {
		if err := m.runner.InstallDependencies(project); err != nil {
			return err
		}
	}

	// Run the project
	runOpts := ContainerRunOptions{
		Project: project,
		Image:   imageTag,
		Detach:  opts.Detach,
	}

	if err := m.runner.Run(runOpts); err != nil {
		return err
	}

	// Update status
	return m.configMgr.Update(func(c *types.Config) error {
		if proj, exists := c.Projects[project.Name]; exists {
			proj.Status = types.StatusRunning
		}
		return nil
	})
}

// Start starts a stopped project
func (m *Manager) Start(name string) error {
	project, err := m.Get(name)
	if err != nil {
		return err
	}

	// Check if container exists
	exists, err := m.docker.ContainerExists(project.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}

	if !exists {
		return fmt.Errorf("container not found, please run: doku project run %s", name)
	}

	// Get container ID
	containerID := project.ContainerID
	if containerID == "" {
		// Try to find it
		inspect, err := m.docker.ContainerInspect(project.ContainerName)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}
		containerID = inspect.ID
	}

	// Start the container
	if err := m.docker.ContainerStart(containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Update status
	return m.configMgr.Update(func(c *types.Config) error {
		if proj, exists := c.Projects[name]; exists {
			proj.Status = types.StatusRunning
			proj.ContainerID = containerID
		}
		return nil
	})
}

// Stop stops a running project
func (m *Manager) Stop(name string) error {
	project, err := m.Get(name)
	if err != nil {
		return err
	}

	// Check if container exists
	exists, err := m.docker.ContainerExists(project.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}

	if !exists {
		return fmt.Errorf("container not found")
	}

	// Get container ID
	containerID := project.ContainerID
	if containerID == "" {
		inspect, err := m.docker.ContainerInspect(project.ContainerName)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}
		containerID = inspect.ID
	}

	// Stop the container
	timeout := 10
	if err := m.docker.ContainerStop(containerID, &timeout); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Update status
	return m.configMgr.Update(func(c *types.Config) error {
		if proj, exists := c.Projects[name]; exists {
			proj.Status = types.StatusStopped
		}
		return nil
	})
}

// Restart restarts a project
func (m *Manager) Restart(name string) error {
	project, err := m.Get(name)
	if err != nil {
		return err
	}

	// Check if container exists
	exists, err := m.docker.ContainerExists(project.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}

	if !exists {
		return fmt.Errorf("container not found, please run: doku project run %s", name)
	}

	// Get container ID
	containerID := project.ContainerID
	if containerID == "" {
		inspect, err := m.docker.ContainerInspect(project.ContainerName)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}
		containerID = inspect.ID
	}

	// Restart the container
	timeout := 10
	if err := m.docker.ContainerRestart(containerID, &timeout); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}

	return nil
}

// Remove removes a project
func (m *Manager) Remove(name string, removeImage bool) error {
	project, err := m.Get(name)
	if err != nil {
		return err
	}

	// Check if container exists
	exists, err := m.docker.ContainerExists(project.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}

	if exists {
		// Get container ID
		containerID := project.ContainerID
		if containerID == "" {
			inspect, err := m.docker.ContainerInspect(project.ContainerName)
			if err == nil {
				containerID = inspect.ID
			}
		}

		if containerID != "" {
			// Stop if running
			timeout := 10
			if err := m.docker.ContainerStop(containerID, &timeout); err != nil {
				// Log warning but continue with removal
				fmt.Printf("Warning: failed to stop container: %v\n", err)
			}

			// Remove container
			if err := m.docker.ContainerRemove(containerID, true); err != nil {
				return fmt.Errorf("failed to remove container: %w", err)
			}
		}
	}

	// Remove image if requested
	if removeImage {
		imageTag := fmt.Sprintf("doku-project-%s:latest", project.Name)
		if err := m.docker.ImageRemove(imageTag, true); err != nil {
			// Don't fail if image doesn't exist
			fmt.Printf("Warning: failed to remove image: %v\n", err)
		}
	}

	// Remove DNS entry if project has a URL
	if project.URL != "" {
		// Extract subdomain from URL (e.g., "https://ui.doku.local" -> "ui.doku.local")
		subdomain := strings.TrimPrefix(project.URL, "https://")
		subdomain = strings.TrimPrefix(subdomain, "http://")

		dnsMgr := dns.NewManager()
		if err := dnsMgr.RemoveSingleEntry(subdomain); err != nil {
			// Only show warning, don't fail the removal
			fmt.Printf("Warning: failed to remove DNS entry: %v\n", err)
		}
	}

	// Remove from config
	return m.configMgr.RemoveProject(name)
}

// GetStatus returns the current status of a project
func (m *Manager) GetStatus(name string) (types.ServiceStatus, error) {
	project, err := m.Get(name)
	if err != nil {
		return types.StatusUnknown, err
	}

	return m.getContainerStatus(project.ContainerName)
}

// getContainerStatus gets the status of a container by name
func (m *Manager) getContainerStatus(containerName string) (types.ServiceStatus, error) {
	exists, err := m.docker.ContainerExists(containerName)
	if err != nil {
		return types.StatusUnknown, err
	}

	if !exists {
		return types.StatusStopped, nil
	}

	// Get detailed container info
	inspect, err := m.docker.ContainerInspect(containerName)
	if err != nil {
		return types.StatusUnknown, err
	}

	if inspect.State.Running {
		return types.StatusRunning, nil
	} else if inspect.State.ExitCode != 0 {
		return types.StatusFailed, nil
	}

	return types.StatusStopped, nil
}

// imageExists checks if a Docker image exists
func (m *Manager) imageExists(imageTag string) bool {
	exists, err := m.docker.ImageExists(imageTag)
	return err == nil && exists
}

// validateProjectName validates a project name
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Must be lowercase alphanumeric with hyphens
	for _, c := range name {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return fmt.Errorf("project name must contain only lowercase letters, numbers, and hyphens")
		}
	}

	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("project name cannot start or end with a hyphen")
	}

	return nil
}
