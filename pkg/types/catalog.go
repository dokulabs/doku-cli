package types

// ServiceCatalog represents the complete catalog structure
type ServiceCatalog struct {
	Version  string                     `toml:"version"`
	Services map[string]*CatalogService `toml:"services"`
}

// CatalogService represents a service definition in the catalog
type CatalogService struct {
	Name        string                  `toml:"name"`
	Description string                  `toml:"description"`
	Category    string                  `toml:"category"` // database, cache, queue, monitoring, etc.
	Icon        string                  `toml:"icon"`     // emoji or icon reference
	Versions    map[string]*ServiceSpec `toml:"versions"`
	Tags        []string                `toml:"tags"`
	Links       *ServiceLinks           `toml:"links"`
}

// ServiceSpec represents a specific version of a service
type ServiceSpec struct {
	// Single-container fields (backward compatible)
	Image         string                `toml:"image" yaml:"image"`                 // Docker image with tag
	Description   string                `toml:"description" yaml:"description"`     // Version-specific description
	Port          int                   `toml:"port" yaml:"port"`                   // Main service port (exposed via Traefik)
	AdminPort     int                   `toml:"admin_port" yaml:"admin_port"`       // Optional admin/management port
	Protocol      string                `toml:"protocol" yaml:"protocol"`           // http, tcp, grpc, etc.
	Ports         []string              `toml:"ports" yaml:"ports"`                 // Additional port mappings (e.g., "9000:9000")
	Environment   map[string]string     `toml:"environment" yaml:"environment"`     // Default environment variables
	Volumes       []string              `toml:"volumes" yaml:"volumes"`             // Volume mount paths
	Command       []string              `toml:"command" yaml:"command"`             // Custom command
	Healthcheck   *Healthcheck          `toml:"healthcheck" yaml:"healthcheck"`     // Health check configuration
	Resources     *ResourceRequirements `toml:"resources" yaml:"resources"`         // CPU/memory requirements
	Configuration *ServiceConfiguration `toml:"configuration" yaml:"configuration"` // Configuration options

	// Multi-container support (new)
	Containers     []ContainerSpec `toml:"containers" yaml:"containers"`           // Multiple containers for this service
	InitContainers []InitContainer `toml:"init_containers" yaml:"init_containers"` // Init containers that run once before service starts

	// Dependency management (enhanced)
	Dependencies []DependencySpec `toml:"dependencies" yaml:"dependencies"` // Service dependencies with configuration
}

// ContainerSpec defines a single container within a multi-container service
type ContainerSpec struct {
	Name        string                `toml:"name" yaml:"name"`               // Container name (e.g., "frontend", "query-service")
	Image       string                `toml:"image" yaml:"image"`             // Docker image with tag
	Primary     bool                  `toml:"primary" yaml:"primary"`         // Is this the primary/main container (default: first)
	Ports       []string              `toml:"ports" yaml:"ports"`             // Port mappings (e.g., "3301:3301")
	Environment map[string]string     `toml:"environment" yaml:"environment"` // Container-specific environment variables
	Volumes     []string              `toml:"volumes" yaml:"volumes"`         // Volume mount paths
	DependsOn   []string              `toml:"depends_on" yaml:"depends_on"`   // Internal (same service) or external dependencies
	Healthcheck *Healthcheck          `toml:"healthcheck" yaml:"healthcheck"` // Container health check
	Resources   *ResourceRequirements `toml:"resources" yaml:"resources"`     // Container resource limits
	Command     []string              `toml:"command" yaml:"command"`         // Custom command override
	Entrypoint  []string              `toml:"entrypoint" yaml:"entrypoint"`   // Custom entrypoint override
}

// InitContainer defines a container that runs once before the service starts
// Useful for migrations, setup scripts, etc.
type InitContainer struct {
	Name        string            `toml:"name" yaml:"name"`               // Init container name (e.g., "migrator-sync")
	Image       string            `toml:"image" yaml:"image"`             // Docker image with tag
	Command     []string          `toml:"command" yaml:"command"`         // Command to run
	Environment map[string]string `toml:"environment" yaml:"environment"` // Environment variables
	DependsOn   []string          `toml:"depends_on" yaml:"depends_on"`   // Dependencies (must complete before this runs)
}

// DependencySpec defines a service dependency with configuration
type DependencySpec struct {
	Name        string            `toml:"name" yaml:"name"`               // Service name (e.g., "clickhouse")
	Version     string            `toml:"version" yaml:"version"`         // Version constraint (default: "latest")
	Required    bool              `toml:"required" yaml:"required"`       // Is this dependency required (default: true)
	Environment map[string]string `toml:"environment" yaml:"environment"` // Override environment variables for dependency
}

// ServiceLinks contains useful links for a service
type ServiceLinks struct {
	Homepage      string `toml:"homepage"`
	Documentation string `toml:"documentation"`
	Repository    string `toml:"repository"`
}

// Healthcheck defines health check configuration
type Healthcheck struct {
	Test     []string `toml:"test"`         // Health check command
	Interval string   `toml:"interval"`     // Check interval (e.g., "30s")
	Timeout  string   `toml:"timeout"`      // Check timeout
	Retries  int      `toml:"retries"`      // Number of retries
	Start    string   `toml:"start_period"` // Start period before checks begin
}

// ResourceRequirements defines default resource requirements
type ResourceRequirements struct {
	MemoryMin string `toml:"memory_min"` // Minimum memory (e.g., "256m")
	MemoryMax string `toml:"memory_max"` // Maximum memory (e.g., "1g")
	CPUMin    string `toml:"cpu_min"`    // Minimum CPU (e.g., "0.25")
	CPUMax    string `toml:"cpu_max"`    // Maximum CPU (e.g., "1.0")
}

// ServiceConfiguration defines configurable options
type ServiceConfiguration struct {
	Options []ConfigOption `toml:"options" yaml:"options"` // Configuration options
}

// ConfigOption represents a single configuration option
type ConfigOption struct {
	Name        string   `toml:"name" yaml:"name"`                                 // Option name
	Description string   `toml:"description" yaml:"description"`                   // Option description
	Type        string   `toml:"type" yaml:"type"`                                 // string, int, bool, select
	Default     string   `toml:"default" yaml:"default"`                           // Default value
	Required    bool     `toml:"required" yaml:"required"`                         // Whether required
	EnvVar      string   `toml:"env_var" yaml:"env_var"`                           // Environment variable name
	Options     []string `toml:"options,omitempty" yaml:"options,omitempty"`       // For select type
	Validation  string   `toml:"validation,omitempty" yaml:"validation,omitempty"` // Validation regex
}

// ConnectionInfo represents service connection information
type ConnectionInfo struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	URL      string            `json:"url"`
	Protocol string            `json:"protocol"`
	Env      map[string]string `json:"env"` // Environment variables for connection
}

// Validation methods

// IsMultiContainer returns true if this service has multiple containers
func (s *ServiceSpec) IsMultiContainer() bool {
	return len(s.Containers) > 0
}

// GetPrimaryContainer returns the primary container spec for multi-container services
// Returns nil if this is a single-container service or no primary is set
func (s *ServiceSpec) GetPrimaryContainer() *ContainerSpec {
	if !s.IsMultiContainer() {
		return nil
	}

	// First check if any container explicitly marked as primary
	for i := range s.Containers {
		if s.Containers[i].Primary {
			return &s.Containers[i]
		}
	}

	// If no explicit primary, return first container
	if len(s.Containers) > 0 {
		return &s.Containers[0]
	}

	return nil
}

// GetContainerByName finds a container by name in a multi-container service
func (s *ServiceSpec) GetContainerByName(name string) *ContainerSpec {
	for i := range s.Containers {
		if s.Containers[i].Name == name {
			return &s.Containers[i]
		}
	}
	return nil
}

// HasDependencies returns true if this service has dependencies
func (s *ServiceSpec) HasDependencies() bool {
	return len(s.Dependencies) > 0
}

// GetDependencyNames returns a list of dependency service names
func (s *ServiceSpec) GetDependencyNames() []string {
	names := make([]string, len(s.Dependencies))
	for i, dep := range s.Dependencies {
		names[i] = dep.Name
	}
	return names
}

// Validate checks if the ServiceSpec is valid
func (s *ServiceSpec) Validate() error {
	// Must have either Image (single-container) or Containers (multi-container)
	if s.Image == "" && len(s.Containers) == 0 {
		return &ValidationError{Field: "image/containers", Message: "service must have either 'image' or 'containers' defined"}
	}

	// Cannot have both Image and Containers
	if s.Image != "" && len(s.Containers) > 0 {
		return &ValidationError{Field: "image/containers", Message: "service cannot have both 'image' and 'containers' defined"}
	}

	// Single-container validation
	if s.Image != "" {
		if s.Port <= 0 {
			return &ValidationError{Field: "port", Message: "port must be greater than 0"}
		}
	}

	// Multi-container validation
	if len(s.Containers) > 0 {
		// Check for duplicate container names
		names := make(map[string]bool)
		primaryCount := 0

		for _, container := range s.Containers {
			if container.Name == "" {
				return &ValidationError{Field: "containers.name", Message: "container name cannot be empty"}
			}
			if container.Image == "" {
				return &ValidationError{Field: "containers.image", Message: "container image cannot be empty"}
			}
			if names[container.Name] {
				return &ValidationError{Field: "containers.name", Message: "duplicate container name: " + container.Name}
			}
			names[container.Name] = true

			if container.Primary {
				primaryCount++
			}
		}

		// Can have at most one primary container
		if primaryCount > 1 {
			return &ValidationError{Field: "containers.primary", Message: "only one container can be marked as primary"}
		}

		// Must have a port if not all containers are internal
		if s.Port <= 0 {
			return &ValidationError{Field: "port", Message: "multi-container service must specify main port for Traefik exposure"}
		}
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
