package catalog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/dokulabs/doku-cli/pkg/types"
)

// HierarchicalLoader loads catalog from hierarchical YAML structure
type HierarchicalLoader struct {
	catalogDir string
}

// NewHierarchicalLoader creates a new hierarchical catalog loader
func NewHierarchicalLoader(catalogDir string) *HierarchicalLoader {
	return &HierarchicalLoader{
		catalogDir: catalogDir,
	}
}

// Load loads the complete catalog from hierarchical structure
func (l *HierarchicalLoader) Load() (*types.ServiceCatalog, error) {
	// Load catalog metadata
	metadataPath := filepath.Join(l.catalogDir, "catalog.yaml")
	metadata, err := l.loadCatalogMetadata(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load catalog metadata: %w", err)
	}

	catalog := &types.ServiceCatalog{
		Version:  metadata.Version,
		Services: make(map[string]*types.CatalogService),
	}

	// Scan services directory
	servicesDir := filepath.Join(l.catalogDir, "services")
	if _, err := os.Stat(servicesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("services directory not found: %s", servicesDir)
	}

	// Walk through categories
	categories, err := os.ReadDir(servicesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read services directory: %w", err)
	}

	for _, categoryEntry := range categories {
		if !categoryEntry.IsDir() {
			continue
		}

		categoryDir := filepath.Join(servicesDir, categoryEntry.Name())
		services, err := os.ReadDir(categoryDir)
		if err != nil {
			continue // Skip invalid directories
		}

		// Process each service in the category
		for _, serviceEntry := range services {
			if !serviceEntry.IsDir() {
				continue
			}

			serviceID := serviceEntry.Name()
			serviceDir := filepath.Join(categoryDir, serviceID)

			service, err := l.loadService(serviceDir)
			if err != nil {
				return nil, fmt.Errorf("failed to load service %s: %w", serviceID, err)
			}

			catalog.Services[serviceID] = service
		}
	}

	return catalog, nil
}

// loadCatalogMetadata loads the root catalog.yaml metadata
func (l *HierarchicalLoader) loadCatalogMetadata(path string) (*CatalogMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata CatalogMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// loadService loads a service from its directory
func (l *HierarchicalLoader) loadService(serviceDir string) (*types.CatalogService, error) {
	// Load service.yaml
	servicePath := filepath.Join(serviceDir, "service.yaml")
	serviceData, err := os.ReadFile(servicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service file: %w", err)
	}

	var metadata ServiceMetadata
	if err := yaml.Unmarshal(serviceData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse service metadata: %w", err)
	}

	// Create service
	service := &types.CatalogService{
		Name:        metadata.Name,
		Description: metadata.Description,
		Category:    metadata.Category,
		Icon:        metadata.Icon,
		Tags:        metadata.Tags,
		Versions:    make(map[string]*types.ServiceSpec),
	}

	if metadata.Links != nil {
		service.Links = &types.ServiceLinks{
			Homepage:      metadata.Links.Homepage,
			Documentation: metadata.Links.Documentation,
			Repository:    metadata.Links.Repository,
		}
	}

	// Load versions
	versionsDir := filepath.Join(serviceDir, "versions")
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("versions directory not found for service")
	}

	versions, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	for _, versionEntry := range versions {
		if !versionEntry.IsDir() {
			continue
		}

		version := versionEntry.Name()
		versionDir := filepath.Join(versionsDir, version)

		spec, err := l.loadVersionSpec(versionDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load version %s: %w", version, err)
		}

		service.Versions[version] = spec
	}

	return service, nil
}

// loadVersionSpec loads a version configuration
func (l *HierarchicalLoader) loadVersionSpec(versionDir string) (*types.ServiceSpec, error) {
	configPath := filepath.Join(versionDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config VersionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse version config: %w", err)
	}

	spec := &types.ServiceSpec{
		Image:         config.Image,
		Description:   config.Description,
		Port:          config.Port,
		AdminPort:     config.AdminPort,
		Protocol:      config.Protocol,
		Volumes:       config.Volumes,
		Command:       config.Command,
		Environment:   config.Environment,
		Dependencies:  config.Dependencies,
	}

	// Copy healthcheck if present
	if config.Healthcheck != nil {
		spec.Healthcheck = config.Healthcheck
	}

	// Copy resources if present
	if config.Resources != nil {
		spec.Resources = config.Resources
	}

	// Copy configuration if present
	if config.Configuration != nil {
		spec.Configuration = config.Configuration
	}

	return spec, nil
}

// CatalogMetadata represents the root catalog.yaml structure
type CatalogMetadata struct {
	Version       string `yaml:"version"`
	Format        string `yaml:"format"`
	SchemaVersion string `yaml:"catalog_schema_version"`
}

// ServiceMetadata represents service.yaml structure
type ServiceMetadata struct {
	Name              string        `yaml:"name"`
	Description       string        `yaml:"description"`
	Category          string        `yaml:"category"`
	Icon              string        `yaml:"icon"`
	Tags              []string      `yaml:"tags"`
	Links             *ServiceLinks `yaml:"links,omitempty"`
	AvailableVersions []string      `yaml:"available_versions"`
	LatestVersion     string        `yaml:"latest_version"`
}

// ServiceLinks represents service links
type ServiceLinks struct {
	Homepage      string `yaml:"homepage,omitempty"`
	Documentation string `yaml:"documentation,omitempty"`
	Repository    string `yaml:"repository,omitempty"`
}

// VersionConfig represents version config.yaml structure
type VersionConfig struct {
	Version       string                       `yaml:"version"`
	Image         string                       `yaml:"image"`
	Description   string                       `yaml:"description"`
	Port          int                          `yaml:"port"`
	AdminPort     int                          `yaml:"admin_port,omitempty"`
	Protocol      string                       `yaml:"protocol"`
	Volumes       []string                     `yaml:"volumes,omitempty"`
	Command       []string                     `yaml:"command,omitempty"`
	Environment   map[string]string            `yaml:"environment,omitempty"`
	Healthcheck   *types.Healthcheck           `yaml:"healthcheck,omitempty"`
	Resources     *types.ResourceRequirements  `yaml:"resources,omitempty"`
	Configuration *types.ServiceConfiguration  `yaml:"configuration,omitempty"`
	Dependencies  []string                     `yaml:"dependencies,omitempty"`
}

// ListCategories returns all available categories
func (l *HierarchicalLoader) ListCategories() ([]string, error) {
	servicesDir := filepath.Join(l.catalogDir, "services")
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read services directory: %w", err)
	}

	categories := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			categories = append(categories, entry.Name())
		}
	}

	sort.Strings(categories)
	return categories, nil
}

// ListServicesByCategory lists all services in a category
func (l *HierarchicalLoader) ListServicesByCategory(category string) ([]string, error) {
	categoryDir := filepath.Join(l.catalogDir, "services", category)
	entries, err := os.ReadDir(categoryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read category directory: %w", err)
	}

	services := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			services = append(services, entry.Name())
		}
	}

	sort.Strings(services)
	return services, nil
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
