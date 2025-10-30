package catalog

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dokulabs/doku-cli/pkg/types"
)

const (
	// DefaultCatalogURL is the URL to the catalog repository releases
	DefaultCatalogURL = "https://github.com/dokulabs/doku-catalog/releases/latest/download/catalog.toml"
	CatalogFileName   = "catalog.toml"
)

// Manager handles catalog operations
type Manager struct {
	catalogDir string
	catalogURL string
}

// NewManager creates a new catalog manager
func NewManager(catalogDir string) *Manager {
	return &Manager{
		catalogDir: catalogDir,
		catalogURL: DefaultCatalogURL,
	}
}

// SetCatalogURL sets a custom catalog URL (for testing or custom catalogs)
func (m *Manager) SetCatalogURL(url string) {
	m.catalogURL = url
}

// GetCatalogPath returns the path to the local catalog file
func (m *Manager) GetCatalogPath() string {
	return filepath.Join(m.catalogDir, CatalogFileName)
}

// FetchCatalog downloads the catalog from the configured URL
func (m *Manager) FetchCatalog() error {
	// Ensure catalog directory exists
	if err := os.MkdirAll(m.catalogDir, 0755); err != nil {
		return fmt.Errorf("failed to create catalog directory: %w", err)
	}

	// Download catalog
	resp, err := http.Get(m.catalogURL)
	if err != nil {
		return fmt.Errorf("failed to download catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download catalog: HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tmpPath := m.GetCatalogPath() + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Copy content to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save catalog: %w", err)
	}

	// Close the file before renaming
	tmpFile.Close()

	// Atomically replace the catalog file
	if err := os.Rename(tmpPath, m.GetCatalogPath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to update catalog: %w", err)
	}

	return nil
}

// LoadCatalog loads and parses the catalog from disk
func (m *Manager) LoadCatalog() (*types.ServiceCatalog, error) {
	catalogPath := m.GetCatalogPath()

	// Check if catalog exists
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("catalog not found, please run 'doku catalog update'")
	}

	// Parse catalog
	var catalog types.ServiceCatalog
	if _, err := toml.DecodeFile(catalogPath, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse catalog: %w", err)
	}

	return &catalog, nil
}

// GetService retrieves a specific service from the catalog
func (m *Manager) GetService(serviceName string) (*types.CatalogService, error) {
	catalog, err := m.LoadCatalog()
	if err != nil {
		return nil, err
	}

	service, exists := catalog.Services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found in catalog", serviceName)
	}

	return service, nil
}

// GetServiceVersion retrieves a specific version of a service
func (m *Manager) GetServiceVersion(serviceName, version string) (*types.ServiceSpec, error) {
	service, err := m.GetService(serviceName)
	if err != nil {
		return nil, err
	}

	// If version is empty, use latest
	if version == "" || version == "latest" {
		version = m.getLatestVersion(service)
	}

	spec, exists := service.Versions[version]
	if !exists {
		return nil, fmt.Errorf("version '%s' not found for service '%s'", version, serviceName)
	}

	return spec, nil
}

// getLatestVersion returns the latest version of a service
// For now, we'll use a simple heuristic (first version in map)
// TODO: Implement proper semantic versioning logic
func (m *Manager) getLatestVersion(service *types.CatalogService) string {
	if len(service.Versions) == 0 {
		return ""
	}

	// Return the first version as "latest"
	// TODO: Implement semantic version sorting
	for version := range service.Versions {
		return version
	}

	return ""
}

// ListServices returns a list of all available services
func (m *Manager) ListServices() ([]*types.CatalogService, error) {
	catalog, err := m.LoadCatalog()
	if err != nil {
		return nil, err
	}

	services := make([]*types.CatalogService, 0, len(catalog.Services))
	for _, service := range catalog.Services {
		services = append(services, service)
	}

	return services, nil
}

// ListServicesByCategory returns services filtered by category
func (m *Manager) ListServicesByCategory(category string) ([]*types.CatalogService, error) {
	allServices, err := m.ListServices()
	if err != nil {
		return nil, err
	}

	filtered := make([]*types.CatalogService, 0)
	for _, service := range allServices {
		if strings.EqualFold(service.Category, category) {
			filtered = append(filtered, service)
		}
	}

	return filtered, nil
}

// SearchServices searches for services by name, description, or tags
func (m *Manager) SearchServices(query string) ([]*types.CatalogService, error) {
	allServices, err := m.ListServices()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	results := make([]*types.CatalogService, 0)

	for _, service := range allServices {
		// Search in name
		if strings.Contains(strings.ToLower(service.Name), query) {
			results = append(results, service)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(service.Description), query) {
			results = append(results, service)
			continue
		}

		// Search in tags
		for _, tag := range service.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, service)
				break
			}
		}
	}

	return results, nil
}

// CatalogExists checks if the catalog file exists
func (m *Manager) CatalogExists() bool {
	_, err := os.Stat(m.GetCatalogPath())
	return err == nil
}

// GetCatalogVersion returns the version of the loaded catalog
func (m *Manager) GetCatalogVersion() (string, error) {
	catalog, err := m.LoadCatalog()
	if err != nil {
		return "", err
	}

	return catalog.Version, nil
}

// ValidateCatalog validates the catalog structure
func (m *Manager) ValidateCatalog() error {
	catalog, err := m.LoadCatalog()
	if err != nil {
		return err
	}

	// Check version
	if catalog.Version == "" {
		return fmt.Errorf("catalog version is missing")
	}

	// Check services
	if len(catalog.Services) == 0 {
		return fmt.Errorf("catalog contains no services")
	}

	// Validate each service
	for name, service := range catalog.Services {
		if service.Name == "" {
			return fmt.Errorf("service '%s' has no name", name)
		}

		if len(service.Versions) == 0 {
			return fmt.Errorf("service '%s' has no versions", name)
		}

		// Validate each version
		for version, spec := range service.Versions {
			if spec.Image == "" {
				return fmt.Errorf("service '%s' version '%s' has no image", name, version)
			}

			if spec.Port == 0 {
				return fmt.Errorf("service '%s' version '%s' has no port", name, version)
			}
		}
	}

	return nil
}
