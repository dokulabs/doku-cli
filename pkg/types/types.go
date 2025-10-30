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
	Name        string
	DisplayName string
	Description string
	Category    string
	Icon        string
	Tags        []string
	OfficialDocs string
	Versions    VersionConfig
	Docker      DockerConfig
	Traefik     TraefikConfig
	Discovery   DiscoveryConfig
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
	Container     int
	HostDefault   int
	Description   string
	ExposeToHost  bool
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
	Enabled        bool
	Port           int
	HasWebInterface bool
	WebPort        int
	CustomRules    []string
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
	Name            string
	ServiceType     string
	Version         string
	Status          ServiceStatus
	ContainerName   string
	URL             string
	ConnectionString string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Network         NetworkConfig
	Resources       ResourceConfig
	Traefik         TraefikInstanceConfig
	Volumes         map[string]string
	Environment     map[string]string
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
	Preferences PreferencesConfig
	Network     NetworkGlobalConfig
	Traefik     TraefikGlobalConfig
	Certificates CertificatesConfig
	Instances   map[string]*Instance
	Projects    map[string]*Project
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
	ContainerName   string
	Status          ServiceStatus
	DashboardEnabled bool
	HTTPPort        int
	HTTPSPort       int
	DashboardURL    string
}

// CertificatesConfig holds SSL certificate configuration
type CertificatesConfig struct {
	CACert   string
	CAKey    string
	CertsDir string
}
