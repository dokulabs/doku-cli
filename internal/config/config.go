package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dokulabs/doku-cli/pkg/types"
)

const (
	DefaultDomain   = "doku.local"
	DefaultProtocol = "https"
	ConfigFileName  = "config.toml"
	DokuDirName     = ".doku"
)

// Manager handles configuration operations
type Manager struct {
	configPath string
	dokuDir    string
	config     *types.Config
}

// New creates a new configuration manager
func New() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dokuDir := filepath.Join(homeDir, DokuDirName)
	configPath := filepath.Join(dokuDir, ConfigFileName)

	return &Manager{
		configPath: configPath,
		dokuDir:    dokuDir,
	}, nil
}

// NewWithCustomPath creates a new configuration manager with a custom doku directory path
// This is primarily used for testing purposes
func NewWithCustomPath(dokuDir string) (*Manager, error) {
	configPath := filepath.Join(dokuDir, ConfigFileName)

	return &Manager{
		configPath: configPath,
		dokuDir:    dokuDir,
	}, nil
}

// Initialize creates the Doku directory and default configuration
func (m *Manager) Initialize() error {
	// Create .doku directory if it doesn't exist
	if err := os.MkdirAll(m.dokuDir, 0755); err != nil {
		return fmt.Errorf("failed to create doku directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"catalog", "traefik", "certs", "services", "projects"}
	for _, subdir := range subdirs {
		path := filepath.Join(m.dokuDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	// Initialize default config if it doesn't exist
	if !m.Exists() {
		if err := m.CreateDefault(); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
	}

	return nil
}

// Exists checks if the configuration file exists
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.configPath)
	return err == nil
}

// CreateDefault creates a default configuration file
func (m *Manager) CreateDefault() error {
	config := m.getDefaultConfig()
	return m.Save(config)
}

// Load reads the configuration from disk
func (m *Manager) Load() (*types.Config, error) {
	if !m.Exists() {
		return nil, fmt.Errorf("config file does not exist: %s", m.configPath)
	}

	var config types.Config
	if _, err := toml.DecodeFile(m.configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Initialize maps if nil
	if config.Instances == nil {
		config.Instances = make(map[string]*types.Instance)
	}
	if config.Projects == nil {
		config.Projects = make(map[string]*types.Project)
	}

	m.config = &config
	return &config, nil
}

// Save writes the configuration to disk
func (m *Manager) Save(config *types.Config) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create temporary file
	tmpFile := m.configPath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}

	// Encode config to TOML
	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(config); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to close temp config file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, m.configPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to save config file: %w", err)
	}

	m.config = config
	return nil
}

// Get returns the current configuration (loads if not cached)
func (m *Manager) Get() (*types.Config, error) {
	if m.config != nil {
		return m.config, nil
	}
	return m.Load()
}

// Update updates specific fields in the configuration
func (m *Manager) Update(updateFn func(*types.Config) error) error {
	config, err := m.Get()
	if err != nil {
		return err
	}

	if err := updateFn(config); err != nil {
		return err
	}

	return m.Save(config)
}

// GetDokuDir returns the path to the .doku directory
func (m *Manager) GetDokuDir() string {
	return m.dokuDir
}

// GetCatalogDir returns the path to the catalog directory
func (m *Manager) GetCatalogDir() string {
	return filepath.Join(m.dokuDir, "catalog")
}

// GetTraefikDir returns the path to the traefik directory
func (m *Manager) GetTraefikDir() string {
	return filepath.Join(m.dokuDir, "traefik")
}

// GetCertsDir returns the path to the certs directory
func (m *Manager) GetCertsDir() string {
	return filepath.Join(m.dokuDir, "certs")
}

// GetServicesDir returns the path to the services directory
func (m *Manager) GetServicesDir() string {
	return filepath.Join(m.dokuDir, "services")
}

// GetProjectsDir returns the path to the projects directory
func (m *Manager) GetProjectsDir() string {
	return filepath.Join(m.dokuDir, "projects")
}

// SetDomain updates the domain preference
func (m *Manager) SetDomain(domain string) error {
	return m.Update(func(c *types.Config) error {
		c.Preferences.Domain = domain
		return nil
	})
}

// SetProtocol updates the protocol preference
func (m *Manager) SetProtocol(protocol string) error {
	if protocol != "http" && protocol != "https" {
		return fmt.Errorf("invalid protocol: %s (must be 'http' or 'https')", protocol)
	}

	return m.Update(func(c *types.Config) error {
		c.Preferences.Protocol = protocol
		return nil
	})
}

// AddInstance adds a new service instance to the configuration
func (m *Manager) AddInstance(instance *types.Instance) error {
	return m.Update(func(c *types.Config) error {
		if c.Instances == nil {
			c.Instances = make(map[string]*types.Instance)
		}
		c.Instances[instance.Name] = instance
		return nil
	})
}

// RemoveInstance removes a service instance from the configuration
func (m *Manager) RemoveInstance(name string) error {
	return m.Update(func(c *types.Config) error {
		delete(c.Instances, name)
		return nil
	})
}

// GetInstance retrieves a service instance by name
func (m *Manager) GetInstance(name string) (*types.Instance, error) {
	config, err := m.Get()
	if err != nil {
		return nil, err
	}

	instance, exists := config.Instances[name]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", name)
	}

	return instance, nil
}

// ListInstances returns all service instances
func (m *Manager) ListInstances() ([]*types.Instance, error) {
	config, err := m.Get()
	if err != nil {
		return nil, err
	}

	instances := make([]*types.Instance, 0, len(config.Instances))
	for _, instance := range config.Instances {
		instances = append(instances, instance)
	}

	return instances, nil
}

// HasInstance checks if an instance exists
func (m *Manager) HasInstance(name string) bool {
	config, err := m.Get()
	if err != nil {
		return false
	}

	_, exists := config.Instances[name]
	return exists
}

// UpdateInstance updates an existing instance
func (m *Manager) UpdateInstance(name string, instance *types.Instance) error {
	return m.Update(func(c *types.Config) error {
		if _, exists := c.Instances[name]; !exists {
			return fmt.Errorf("instance not found: %s", name)
		}
		c.Instances[name] = instance
		return nil
	})
}

// AddProject adds a new project to the configuration
func (m *Manager) AddProject(project *types.Project) error {
	return m.Update(func(c *types.Config) error {
		if c.Projects == nil {
			c.Projects = make(map[string]*types.Project)
		}
		c.Projects[project.Name] = project
		return nil
	})
}

// RemoveProject removes a project from the configuration
func (m *Manager) RemoveProject(name string) error {
	return m.Update(func(c *types.Config) error {
		delete(c.Projects, name)
		return nil
	})
}

// GetProject retrieves a project by name
func (m *Manager) GetProject(name string) (*types.Project, error) {
	config, err := m.Get()
	if err != nil {
		return nil, err
	}

	project, exists := config.Projects[name]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", name)
	}

	return project, nil
}

// UpdateCatalogVersion updates the catalog version and timestamp
func (m *Manager) UpdateCatalogVersion(version string) error {
	return m.Update(func(c *types.Config) error {
		c.Preferences.CatalogVersion = version
		c.Preferences.LastUpdate = time.Now()
		return nil
	})
}

// getDefaultConfig returns a default configuration
func (m *Manager) getDefaultConfig() *types.Config {
	return &types.Config{
		Preferences: types.PreferencesConfig{
			Protocol:       DefaultProtocol,
			Domain:         DefaultDomain,
			CatalogVersion: "",
			LastUpdate:     time.Now(),
			DNSSetup:       "none",
		},
		Network: types.NetworkGlobalConfig{
			Name:    "doku-network",
			Subnet:  "172.20.0.0/16",
			Gateway: "172.20.0.1",
		},
		Traefik: types.TraefikGlobalConfig{
			ContainerName:    "doku-traefik",
			Status:           types.StatusUnknown,
			DashboardEnabled: true,
			HTTPPort:         80,
			HTTPSPort:        443,
			DashboardURL:     "",
		},
		Certificates: types.CertificatesConfig{
			CACert:   filepath.Join(m.GetCertsDir(), "rootCA.pem"),
			CAKey:    filepath.Join(m.GetCertsDir(), "rootCA-key.pem"),
			CertsDir: m.GetCertsDir(),
		},
		Monitoring: types.MonitoringConfig{
			Tool:        "none",
			Enabled:     false,
			URL:         "",
			DSN:         "",
			APIKey:      "",
			InstallTime: time.Time{},
		},
		Instances: make(map[string]*types.Instance),
		Projects:  make(map[string]*types.Project),
	}
}

// IsInitialized checks if Doku has been initialized
func (m *Manager) IsInitialized() bool {
	// Check if .doku directory exists
	if _, err := os.Stat(m.dokuDir); os.IsNotExist(err) {
		return false
	}

	// Check if config file exists
	if !m.Exists() {
		return false
	}

	return true
}

// GetDomain returns the configured domain
func (m *Manager) GetDomain() (string, error) {
	config, err := m.Get()
	if err != nil {
		return "", err
	}
	return config.Preferences.Domain, nil
}

// GetProtocol returns the configured protocol
func (m *Manager) GetProtocol() (string, error) {
	config, err := m.Get()
	if err != nil {
		return "", err
	}
	return config.Preferences.Protocol, nil
}

// SetMonitoringTool sets the monitoring tool preference
func (m *Manager) SetMonitoringTool(tool string) error {
	validTools := map[string]bool{"dozzle": true, "none": true}
	if !validTools[tool] {
		return fmt.Errorf("invalid monitoring tool: %s (must be 'dozzle' or 'none')", tool)
	}

	return m.Update(func(c *types.Config) error {
		c.Monitoring.Tool = tool
		c.Monitoring.Enabled = (tool != "none")
		return nil
	})
}

// SetMonitoringURL sets the monitoring dashboard URL
func (m *Manager) SetMonitoringURL(url string) error {
	return m.Update(func(c *types.Config) error {
		c.Monitoring.URL = url
		return nil
	})
}

// SetMonitoringDSN sets the monitoring DSN/endpoint
func (m *Manager) SetMonitoringDSN(dsn string) error {
	return m.Update(func(c *types.Config) error {
		c.Monitoring.DSN = dsn
		return nil
	})
}

// ConfigureMonitoring sets up monitoring with all required fields
func (m *Manager) ConfigureMonitoring(tool, url, dsn string) error {
	return m.Update(func(c *types.Config) error {
		c.Monitoring.Tool = tool
		c.Monitoring.Enabled = (tool != "none")
		c.Monitoring.URL = url
		c.Monitoring.DSN = dsn
		c.Monitoring.InstallTime = time.Now()
		return nil
	})
}

// GetMonitoringConfig returns the monitoring configuration
func (m *Manager) GetMonitoringConfig() (*types.MonitoringConfig, error) {
	config, err := m.Get()
	if err != nil {
		return nil, err
	}
	return &config.Monitoring, nil
}
