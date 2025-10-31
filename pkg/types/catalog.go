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
	Image         string                `toml:"image"`         // Docker image with tag
	Description   string                `toml:"description"`   // Version-specific description
	Port          int                   `toml:"port"`          // Main service port
	AdminPort     int                   `toml:"admin_port"`    // Optional admin/management port
	Protocol      string                `toml:"protocol"`      // http, tcp, grpc, etc.
	Environment   map[string]string     `toml:"environment"`   // Default environment variables
	Volumes       []string              `toml:"volumes"`       // Volume mount paths
	Command       []string              `toml:"command"`       // Custom command
	Healthcheck   *Healthcheck          `toml:"healthcheck"`   // Health check configuration
	Resources     *ResourceRequirements `toml:"resources"`     // CPU/memory requirements
	Configuration *ServiceConfiguration `toml:"configuration"` // Configuration options
	Dependencies  []string              `toml:"dependencies"`  // Other services this depends on
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
	Options []ConfigOption `toml:"options"` // Configuration options
}

// ConfigOption represents a single configuration option
type ConfigOption struct {
	Name        string   `toml:"name"`                 // Option name
	Description string   `toml:"description"`          // Option description
	Type        string   `toml:"type"`                 // string, int, bool, select
	Default     string   `toml:"default"`              // Default value
	Required    bool     `toml:"required"`             // Whether required
	EnvVar      string   `toml:"env_var"`              // Environment variable name
	Options     []string `toml:"options,omitempty"`    // For select type
	Validation  string   `toml:"validation,omitempty"` // Validation regex
}

// ConnectionInfo represents service connection information
type ConnectionInfo struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	URL      string            `json:"url"`
	Protocol string            `json:"protocol"`
	Env      map[string]string `json:"env"` // Environment variables for connection
}
