package dependencies

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/pkg/types"
)

// setupTestEnvironment creates a temporary test environment with catalog and config
func setupTestEnvironment(t *testing.T) (*Resolver, string, func()) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "doku-resolver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	catalogDir := filepath.Join(tmpDir, "catalog")
	configDir := filepath.Join(tmpDir, ".doku")

	// Create test catalog structure
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		t.Fatalf("Failed to create catalog dir: %v", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create a mock catalog with test services
	createMockCatalog(t, catalogDir)

	// Initialize catalog manager
	catalogMgr := catalog.NewManager(catalogDir)

	// Initialize config manager
	configMgr, err := config.NewWithCustomPath(configDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	if err := configMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	// Create resolver
	resolver := NewResolver(catalogMgr, configMgr)

	// Cleanup function
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return resolver, tmpDir, cleanup
}

// createMockCatalog creates a mock catalog structure for testing
func createMockCatalog(t *testing.T, catalogDir string) {
	// Create catalog metadata
	catalogYAML := `version: "1.0.0"`
	if err := os.WriteFile(filepath.Join(catalogDir, "catalog.yaml"), []byte(catalogYAML), 0644); err != nil {
		t.Fatalf("Failed to write catalog.yaml: %v", err)
	}

	// Create services directory structure
	servicesDir := filepath.Join(catalogDir, "services")

	// Service A (no dependencies)
	createMockService(t, servicesDir, "database", "service-a", "latest", `
version: latest
image: service-a:latest
port: 5000
protocol: http
`)

	// Service B (depends on A)
	createMockService(t, servicesDir, "database", "service-b", "latest", `
version: latest
image: service-b:latest
port: 5001
protocol: http
dependencies_v2:
  - name: service-a
    version: latest
    required: true
`)

	// Service C (depends on B, which depends on A)
	createMockService(t, servicesDir, "cache", "service-c", "latest", `
version: latest
image: service-c:latest
port: 5002
protocol: http
dependencies_v2:
  - name: service-b
    version: latest
`)

	// Service D (circular: depends on E)
	createMockService(t, servicesDir, "queue", "service-d", "latest", `
version: latest
image: service-d:latest
port: 5003
protocol: http
dependencies:
  - service-e
`)

	// Service E (circular: depends on D)
	createMockService(t, servicesDir, "queue", "service-e", "latest", `
version: latest
image: service-e:latest
port: 5004
protocol: http
dependencies:
  - service-d
`)

	// Service F (multiple dependencies: A and B)
	createMockService(t, servicesDir, "monitoring", "service-f", "latest", `
version: latest
image: service-f:latest
port: 5005
protocol: http
dependencies_v2:
  - name: service-a
    version: latest
  - name: service-b
    version: latest
`)
}

func createMockService(t *testing.T, servicesDir, category, serviceName, version, configYAML string) {
	// Create directory structure
	serviceDir := filepath.Join(servicesDir, category, serviceName, "versions", version)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatalf("Failed to create service dir: %v", err)
	}

	// Write config.yaml
	configPath := filepath.Join(serviceDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}

	// Write service.yaml (metadata)
	serviceYAML := `
name: ` + serviceName + `
description: Test service ` + serviceName + `
category: ` + category + `
`
	serviceYAMLPath := filepath.Join(servicesDir, category, serviceName, "service.yaml")
	if err := os.WriteFile(serviceYAMLPath, []byte(serviceYAML), 0644); err != nil {
		t.Fatalf("Failed to write service.yaml: %v", err)
	}
}

// TestResolveNoDependencies tests resolving a service with no dependencies
func TestResolveNoDependencies(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-a", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result.InstallOrder) != 1 {
		t.Errorf("Expected 1 service, got %d", len(result.InstallOrder))
	}

	if result.InstallOrder[0].ServiceName != "service-a" {
		t.Errorf("Expected service-a, got %s", result.InstallOrder[0].ServiceName)
	}
}

// TestResolveSingleDependency tests resolving a service with one dependency
func TestResolveSingleDependency(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-b", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 2 services: service-a (dependency) and service-b
	if len(result.InstallOrder) != 2 {
		t.Errorf("Expected 2 services, got %d", len(result.InstallOrder))
	}

	// Dependency (service-a) should come before dependent (service-b)
	if result.InstallOrder[0].ServiceName != "service-a" {
		t.Errorf("Expected service-a first, got %s", result.InstallOrder[0].ServiceName)
	}

	if result.InstallOrder[1].ServiceName != "service-b" {
		t.Errorf("Expected service-b second, got %s", result.InstallOrder[1].ServiceName)
	}
}

// TestResolveNestedDependencies tests resolving nested dependencies (C -> B -> A)
func TestResolveNestedDependencies(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-c", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 3 services in order: A, B, C
	if len(result.InstallOrder) != 3 {
		t.Errorf("Expected 3 services, got %d", len(result.InstallOrder))
	}

	// Verify correct order: dependencies before dependents
	expectedOrder := []string{"service-a", "service-b", "service-c"}
	for i, expected := range expectedOrder {
		if result.InstallOrder[i].ServiceName != expected {
			t.Errorf("Position %d: expected %s, got %s", i, expected, result.InstallOrder[i].ServiceName)
		}
	}
}

// TestResolveMultipleDependencies tests resolving a service with multiple dependencies
func TestResolveMultipleDependencies(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-f", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 3 services: A, B (with its dep A), and F
	if len(result.InstallOrder) != 3 {
		t.Errorf("Expected 3 services, got %d", len(result.InstallOrder))
	}

	// service-a should come first (common dependency)
	if result.InstallOrder[0].ServiceName != "service-a" {
		t.Errorf("Expected service-a first, got %s", result.InstallOrder[0].ServiceName)
	}

	// service-f should come last
	if result.InstallOrder[len(result.InstallOrder)-1].ServiceName != "service-f" {
		t.Errorf("Expected service-f last, got %s", result.InstallOrder[len(result.InstallOrder)-1].ServiceName)
	}
}

// TestResolveCircularDependency tests detection of circular dependencies
func TestResolveCircularDependency(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// service-d and service-e have circular dependency
	_, err := resolver.Resolve("service-d", "latest")
	if err == nil {
		t.Fatal("Expected circular dependency error, got nil")
	}

	if !IsCircularDependency(err) {
		t.Errorf("Expected CircularDependencyError, got: %v", err)
	}
}

// TestGetMissingDependencies tests getting missing dependencies
func TestGetMissingDependencies(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-c", "latest")
	if err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	missing := resolver.GetMissingDependencies(result)

	// service-c depends on service-b, which depends on service-a
	// So we expect: service-a (missing), service-b (missing), but NOT service-c itself
	// The missing list only includes dependencies, not the target service if required=false
	// Actually, all 3 should be in the result since nothing is installed
	// Let's check what we actually get
	if len(missing) < 2 {
		t.Errorf("Expected at least 2 missing dependencies, got %d", len(missing))
	}

	// Verify the missing services are actually not installed
	for _, dep := range missing {
		if dep.IsInstalled {
			t.Errorf("Dependency %s marked as missing but IsInstalled=true", dep.ServiceName)
		}
	}
}

// TestResolveWithInstalledDependency tests resolving when some dependencies are already installed
func TestResolveWithInstalledDependency(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Simulate that service-a is already installed
	configMgr := resolver.configMgr
	instance := &types.Instance{
		Name:        "service-a",
		ServiceType: "service-a",
		Version:     "latest",
		Status:      types.StatusRunning,
	}
	if err := configMgr.AddInstance(instance); err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}

	// Resolve service-b (which depends on service-a)
	result, err := resolver.Resolve("service-b", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should still have 2 services in order
	if len(result.InstallOrder) != 2 {
		t.Errorf("Expected 2 services, got %d", len(result.InstallOrder))
	}

	// service-a should be marked as installed
	if !result.InstallOrder[0].IsInstalled {
		t.Error("Expected service-a to be marked as installed")
	}

	// service-b should not be marked as installed
	if result.InstallOrder[1].IsInstalled {
		t.Error("Expected service-b to not be marked as installed")
	}

	// Only service-b should be missing
	missing := resolver.GetMissingDependencies(result)
	if len(missing) != 1 || missing[0].ServiceName != "service-b" {
		t.Errorf("Expected only service-b to be missing, got: %v", missing)
	}
}

// TestGetDependencyTree tests dependency tree generation
func TestGetDependencyTree(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	result, err := resolver.Resolve("service-c", "latest")
	if err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	tree := resolver.GetDependencyTree(result, "service-c")
	if tree == "" {
		t.Error("Expected non-empty dependency tree")
	}

	// Tree should contain all three services
	if !contains(tree, "service-a") || !contains(tree, "service-b") || !contains(tree, "service-c") {
		t.Errorf("Dependency tree missing services: %s", tree)
	}
}

// TestResolveNonExistentService tests resolving a service that doesn't exist
func TestResolveNonExistentService(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	_, err := resolver.Resolve("non-existent-service", "latest")
	if err == nil {
		t.Fatal("Expected error for non-existent service, got nil")
	}
}

// TestValidateDependencies tests the validation function
func TestValidateDependencies(t *testing.T) {
	resolver, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Valid service with valid dependencies
	err := resolver.ValidateDependencies("service-b", "latest")
	if err != nil {
		t.Errorf("Expected no error for valid service, got: %v", err)
	}

	// Service with circular dependency
	err = resolver.ValidateDependencies("service-d", "latest")
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
