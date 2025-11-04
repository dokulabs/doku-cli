package types

import "time"

// ServiceStatus represents the status of a service instance
type ServiceStatus string

const (
	StatusRunning ServiceStatus = "running"
	StatusStopped ServiceStatus = "stopped"
	StatusFailed  ServiceStatus = "failed"
	StatusUnknown ServiceStatus = "unknown"
)

// Service represents a service from the catalog
type Service struct {
	Name         string
	DisplayName  string
	Description  string
	Category     string
	Icon         string
	Tags         []string
	OfficialDocs string
	Versions     VersionConfig
	Docker       DockerConfig
	Traefik      TraefikConfig
	Discovery    DiscoveryConfig
}

// VersionConfig holds version information
type VersionConfig struct {
	Default    string
	Supported  []string
	Deprecated []string
	EOL        []string
}

// DockerConfig holds Docker-specific configuration
type DockerConfig struct {
	Image       string
	DefaultTag  string
	Ports       []PortConfig
	Environment []EnvVar
	Volumes     []VolumeConfig
	HealthCheck *HealthCheck
}

// PortConfig defines a port mapping
type PortConfig struct {
	Container    int
	HostDefault  int
	Description  string
	ExposeToHost bool
}

// EnvVar defines an environment variable
type EnvVar struct {
	Key         string
	Default     string
	Description string
	Required    bool
	Prompt      string
}

// VolumeConfig defines a volume mount
type VolumeConfig struct {
	ContainerPath string
	HostPath      string
	Description   string
	Required      bool
}

// HealthCheck defines container health check
type HealthCheck struct {
	Test     []string
	Interval string
	Timeout  string
	Retries  int
}

// TraefikConfig holds Traefik routing configuration
type TraefikConfig struct {
	Enabled         bool
	Port            int
	HasWebInterface bool
	WebPort         int
	CustomRules     []string
}

// DiscoveryConfig holds service discovery metadata
type DiscoveryConfig struct {
	Type               string
	Protocol           string
	DefaultPort        int
	ConnectionTemplate string
	EnvVars            map[string]string
	CompatibleWith     []string
}

// Instance represents an installed service instance
type Instance struct {
	Name        string
	ServiceType string
	Version     string
	Status      ServiceStatus

	// Single-container fields (backward compatible)
	ContainerName string
	ContainerID   string // Docker container ID

	// Multi-container support (new)
	IsMultiContainer bool            `yaml:"is_multi_container"` // Whether this is a multi-container service
	Containers       []ContainerInfo `yaml:"containers"`         // Container information for multi-container services

	// Dependencies
	Dependencies []string `yaml:"dependencies"` // List of service dependencies

	URL              string
	ConnectionString string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Network          NetworkConfig
	Resources        ResourceConfig
	Traefik          TraefikInstanceConfig
	Volumes          map[string]string
	Environment      map[string]string
}

// ContainerInfo holds information about a container in a multi-container service
type ContainerInfo struct {
	Name        string   `yaml:"name"`      // Container name (e.g., "frontend", "query-service")
	ContainerID string   `yaml:"id"`        // Docker container ID
	FullName    string   `yaml:"full_name"` // Full container name (e.g., "doku-signoz-frontend")
	Primary     bool     `yaml:"primary"`   // Is this the primary/main container?
	Status      string   `yaml:"status"`    // Container status (running, stopped, etc.)
	Ports       []string `yaml:"ports"`     // Port mappings
	Image       string   `yaml:"image"`     // Docker image used
}

// NetworkConfig holds network configuration for an instance
type NetworkConfig struct {
	Name         string
	InternalPort int
	HostPort     int
}

// ResourceConfig holds resource limits and usage
type ResourceConfig struct {
	MemoryLimit string
	MemoryUsage string
	CPULimit    string
	CPUUsage    string
}

// TraefikInstanceConfig holds Traefik configuration for an instance
type TraefikInstanceConfig struct {
	Enabled   bool
	Subdomain string
	Port      int
	Protocol  string
}

// Project represents a local user project
type Project struct {
	Name          string
	Path          string
	Dockerfile    string
	Status        ServiceStatus
	ContainerName string
	URL           string
	Port          int
	CreatedAt     time.Time
	Dependencies  []string
	Environment   map[string]string
}

// Config represents the main Doku configuration
type Config struct {
	Preferences  PreferencesConfig
	Network      NetworkGlobalConfig
	Traefik      TraefikGlobalConfig
	Certificates CertificatesConfig
	Monitoring   MonitoringConfig
	Instances    map[string]*Instance
	Projects     map[string]*Project
}

// PreferencesConfig holds user preferences
type PreferencesConfig struct {
	Protocol       string
	Domain         string
	CatalogVersion string
	LastUpdate     time.Time
	DNSSetup       string
}

// NetworkGlobalConfig holds global network configuration
type NetworkGlobalConfig struct {
	Name    string
	Subnet  string
	Gateway string
}

// TraefikGlobalConfig holds Traefik global configuration
type TraefikGlobalConfig struct {
	ContainerName    string
	Status           ServiceStatus
	DashboardEnabled bool
	HTTPPort         int
	HTTPSPort        int
	DashboardURL     string
}

// CertificatesConfig holds SSL certificate configuration
type CertificatesConfig struct {
	CACert   string
	CAKey    string
	CertsDir string
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Tool        string    `json:"tool" yaml:"tool"`                 // "signoz", "sentry", "none"
	Enabled     bool      `json:"enabled" yaml:"enabled"`           // Whether monitoring is enabled
	URL         string    `json:"url" yaml:"url"`                   // Dashboard URL
	DSN         string    `json:"dsn" yaml:"dsn"`                   // Endpoint (OTLP for SignOz, DSN for Sentry)
	APIKey      string    `json:"api_key" yaml:"api_key"`           // API key if needed
	InstallTime time.Time `json:"install_time" yaml:"install_time"` // When monitoring was installed
}

// Instance helper methods

// GetPrimaryContainer returns the primary container for multi-container instances
func (i *Instance) GetPrimaryContainer() *ContainerInfo {
	if !i.IsMultiContainer {
		return nil
	}

	for idx := range i.Containers {
		if i.Containers[idx].Primary {
			return &i.Containers[idx]
		}
	}

	// If no explicit primary, return first container
	if len(i.Containers) > 0 {
		return &i.Containers[0]
	}

	return nil
}

// GetContainerByName finds a container by name in multi-container instances
func (i *Instance) GetContainerByName(name string) *ContainerInfo {
	if !i.IsMultiContainer {
		return nil
	}

	for idx := range i.Containers {
		if i.Containers[idx].Name == name {
			return &i.Containers[idx]
		}
	}

	return nil
}

// GetAllContainerIDs returns all container IDs for this instance
func (i *Instance) GetAllContainerIDs() []string {
	if !i.IsMultiContainer {
		if i.ContainerID != "" {
			return []string{i.ContainerID}
		}
		return []string{}
	}

	ids := make([]string, 0, len(i.Containers))
	for _, container := range i.Containers {
		if container.ContainerID != "" {
			ids = append(ids, container.ContainerID)
		}
	}
	return ids
}

// GetMainContainerID returns the container ID for single-container or primary container for multi-container
func (i *Instance) GetMainContainerID() string {
	if !i.IsMultiContainer {
		return i.ContainerID
	}

	primary := i.GetPrimaryContainer()
	if primary != nil {
		return primary.ContainerID
	}

	return ""
}

// GetMainContainerName returns the container name for single-container or primary container for multi-container
func (i *Instance) GetMainContainerName() string {
	if !i.IsMultiContainer {
		return i.ContainerName
	}

	primary := i.GetPrimaryContainer()
	if primary != nil {
		return primary.FullName
	}

	return ""
}

// HasDependencies returns true if this instance has dependencies
func (i *Instance) HasDependencies() bool {
	return len(i.Dependencies) > 0
}

// ContainerCount returns the number of containers for this instance
func (i *Instance) ContainerCount() int {
	if !i.IsMultiContainer {
		return 1
	}
	return len(i.Containers)
}
