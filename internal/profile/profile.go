package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ProfileType represents the type of profile
type ProfileType string

const (
	ProfileDevelopment ProfileType = "development"
	ProfileProduction  ProfileType = "production"
	ProfileCustom      ProfileType = "custom"
)

// Profile represents a service configuration profile
type Profile struct {
	Name        string            `toml:"name"`
	Type        ProfileType       `toml:"type"`
	Description string            `toml:"description"`
	Environment map[string]string `toml:"environment"`
	Resources   ResourceProfile   `toml:"resources"`
	Volumes     []VolumeProfile   `toml:"volumes"`
	Replicas    int               `toml:"replicas"`
	Features    FeaturesProfile   `toml:"features"`
}

// ResourceProfile defines resource allocation for a profile
type ResourceProfile struct {
	MemoryLimit string `toml:"memory_limit"`
	MemoryMin   string `toml:"memory_min"`
	CPULimit    string `toml:"cpu_limit"`
	CPUMin      string `toml:"cpu_min"`
}

// VolumeProfile defines volume configuration for a profile
type VolumeProfile struct {
	Source      string `toml:"source"`
	Target      string `toml:"target"`
	Type        string `toml:"type"` // bind, volume, tmpfs
	Persistent  bool   `toml:"persistent"`
	Description string `toml:"description"`
}

// FeaturesProfile defines feature flags for a profile
type FeaturesProfile struct {
	Debug          bool `toml:"debug"`
	SSL            bool `toml:"ssl"`
	Logging        bool `toml:"logging"`
	Metrics        bool `toml:"metrics"`
	HealthCheck    bool `toml:"health_check"`
	AutoRestart    bool `toml:"auto_restart"`
	ResourceLimits bool `toml:"resource_limits"`
}

// ServiceProfiles holds profiles for a specific service
type ServiceProfiles struct {
	Service  string              `toml:"service"`
	Default  string              `toml:"default"`
	Profiles map[string]*Profile `toml:"profiles"`
}

// Manager handles service profile operations
type Manager struct {
	profilesDir string
}

// NewManager creates a new profile manager
func NewManager(dokuDir string) *Manager {
	return &Manager{
		profilesDir: filepath.Join(dokuDir, "profiles"),
	}
}

// GetProfilesDir returns the profiles directory path
func (m *Manager) GetProfilesDir() string {
	return m.profilesDir
}

// EnsureProfilesDir creates the profiles directory if it doesn't exist
func (m *Manager) EnsureProfilesDir() error {
	return os.MkdirAll(m.profilesDir, 0755)
}

// GetServiceProfiles loads profiles for a specific service
func (m *Manager) GetServiceProfiles(serviceName string) (*ServiceProfiles, error) {
	profilePath := filepath.Join(m.profilesDir, serviceName+".toml")

	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no profiles found for service '%s'", serviceName)
	}

	var profiles ServiceProfiles
	if _, err := toml.DecodeFile(profilePath, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse profiles: %w", err)
	}

	return &profiles, nil
}

// SaveServiceProfiles saves profiles for a specific service
func (m *Manager) SaveServiceProfiles(profiles *ServiceProfiles) error {
	if err := m.EnsureProfilesDir(); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	profilePath := filepath.Join(m.profilesDir, profiles.Service+".toml")

	f, err := os.Create(profilePath)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(profiles); err != nil {
		return fmt.Errorf("failed to write profiles: %w", err)
	}

	return nil
}

// GetProfile gets a specific profile for a service
func (m *Manager) GetProfile(serviceName, profileName string) (*Profile, error) {
	profiles, err := m.GetServiceProfiles(serviceName)
	if err != nil {
		return nil, err
	}

	profile, exists := profiles.Profiles[profileName]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found for service '%s'", profileName, serviceName)
	}

	return profile, nil
}

// ListProfiles returns all profiles for a service
func (m *Manager) ListProfiles(serviceName string) ([]string, error) {
	profiles, err := m.GetServiceProfiles(serviceName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(profiles.Profiles))
	for name := range profiles.Profiles {
		names = append(names, name)
	}

	return names, nil
}

// CreateDefaultProfiles creates default development and production profiles for a service
func (m *Manager) CreateDefaultProfiles(serviceName string) (*ServiceProfiles, error) {
	profiles := &ServiceProfiles{
		Service: serviceName,
		Default: "development",
		Profiles: map[string]*Profile{
			"development": GetDevelopmentProfile(serviceName),
			"production":  GetProductionProfile(serviceName),
		},
	}

	if err := m.SaveServiceProfiles(profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

// HasProfiles checks if a service has profiles defined
func (m *Manager) HasProfiles(serviceName string) bool {
	profilePath := filepath.Join(m.profilesDir, serviceName+".toml")
	_, err := os.Stat(profilePath)
	return err == nil
}

// DeleteProfiles removes profiles for a service
func (m *Manager) DeleteProfiles(serviceName string) error {
	profilePath := filepath.Join(m.profilesDir, serviceName+".toml")
	if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete profiles: %w", err)
	}
	return nil
}

// ListAllServices returns all services that have profiles defined
func (m *Manager) ListAllServices() ([]string, error) {
	if _, err := os.Stat(m.profilesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(m.profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	services := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".toml" {
			serviceName := entry.Name()[:len(entry.Name())-5] // Remove .toml extension
			services = append(services, serviceName)
		}
	}

	return services, nil
}

// GetDevelopmentProfile returns a default development profile
func GetDevelopmentProfile(serviceName string) *Profile {
	return &Profile{
		Name:        "development",
		Type:        ProfileDevelopment,
		Description: fmt.Sprintf("Development profile for %s with debug settings", serviceName),
		Environment: map[string]string{
			"LOG_LEVEL": "debug",
		},
		Resources: ResourceProfile{
			MemoryLimit: "512m",
			MemoryMin:   "128m",
			CPULimit:    "0.5",
			CPUMin:      "0.1",
		},
		Replicas: 1,
		Features: FeaturesProfile{
			Debug:          true,
			SSL:            false,
			Logging:        true,
			Metrics:        false,
			HealthCheck:    true,
			AutoRestart:    false,
			ResourceLimits: false,
		},
	}
}

// GetProductionProfile returns a default production profile
func GetProductionProfile(serviceName string) *Profile {
	return &Profile{
		Name:        "production",
		Type:        ProfileProduction,
		Description: fmt.Sprintf("Production profile for %s with optimized settings", serviceName),
		Environment: map[string]string{
			"LOG_LEVEL": "info",
		},
		Resources: ResourceProfile{
			MemoryLimit: "2g",
			MemoryMin:   "512m",
			CPULimit:    "2.0",
			CPUMin:      "0.5",
		},
		Replicas: 1,
		Features: FeaturesProfile{
			Debug:          false,
			SSL:            true,
			Logging:        true,
			Metrics:        true,
			HealthCheck:    true,
			AutoRestart:    true,
			ResourceLimits: true,
		},
	}
}

// MergeEnvironment merges profile environment variables with existing ones
// Profile values take precedence
func (p *Profile) MergeEnvironment(existing map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy existing values
	for k, v := range existing {
		result[k] = v
	}

	// Override with profile values
	for k, v := range p.Environment {
		result[k] = v
	}

	return result
}
