package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dokulabs/doku-cli/pkg/types"
)

func TestNewManager(t *testing.T) {
	mgr, err := New()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Manager is nil")
	}

	if mgr.dokuDir == "" {
		t.Fatal("Doku directory is empty")
	}

	if mgr.configPath == "" {
		t.Fatal("Config path is empty")
	}
}

func TestInitialize(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Check if directories were created
	subdirs := []string{"catalog", "traefik", "certs", "services", "projects"}
	for _, subdir := range subdirs {
		path := filepath.Join(mgr.dokuDir, subdir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", subdir)
		}
	}

	// Check if config file was created
	if !mgr.Exists() {
		t.Error("Config file was not created")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	// Create test config
	testConfig := &types.Config{
		Preferences: types.PreferencesConfig{
			Protocol:       "https",
			Domain:         "test.local",
			CatalogVersion: "v1.0.0",
			LastUpdate:     time.Now(),
			DNSSetup:       "hosts",
		},
		Network: types.NetworkGlobalConfig{
			Name:    "test-network",
			Subnet:  "172.20.0.0/16",
			Gateway: "172.20.0.1",
		},
		Instances: make(map[string]*types.Instance),
		Projects:  make(map[string]*types.Project),
	}

	// Save
	err := mgr.Save(testConfig)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load
	loadedConfig, err := mgr.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify
	if loadedConfig.Preferences.Domain != "test.local" {
		t.Errorf("Expected domain 'test.local', got '%s'", loadedConfig.Preferences.Domain)
	}

	if loadedConfig.Preferences.Protocol != "https" {
		t.Errorf("Expected protocol 'https', got '%s'", loadedConfig.Preferences.Protocol)
	}

	if loadedConfig.Network.Name != "test-network" {
		t.Errorf("Expected network name 'test-network', got '%s'", loadedConfig.Network.Name)
	}
}

func TestAddAndGetInstance(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add instance
	instance := &types.Instance{
		Name:          "postgres-14",
		ServiceType:   "postgres",
		Version:       "14",
		Status:        types.StatusRunning,
		ContainerName: "doku-postgres-14",
		CreatedAt:     time.Now(),
	}

	err = mgr.AddInstance(instance)
	if err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}

	// Get instance
	retrieved, err := mgr.GetInstance("postgres-14")
	if err != nil {
		t.Fatalf("Failed to get instance: %v", err)
	}

	if retrieved.Name != "postgres-14" {
		t.Errorf("Expected name 'postgres-14', got '%s'", retrieved.Name)
	}

	if retrieved.ServiceType != "postgres" {
		t.Errorf("Expected service type 'postgres', got '%s'", retrieved.ServiceType)
	}
}

func TestListInstances(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add multiple instances
	instances := []*types.Instance{
		{Name: "postgres-14", ServiceType: "postgres", Version: "14", Status: types.StatusRunning},
		{Name: "redis", ServiceType: "redis", Version: "7", Status: types.StatusRunning},
		{Name: "mysql", ServiceType: "mysql", Version: "8", Status: types.StatusStopped},
	}

	for _, instance := range instances {
		err = mgr.AddInstance(instance)
		if err != nil {
			t.Fatalf("Failed to add instance %s: %v", instance.Name, err)
		}
	}

	// List instances
	list, err := mgr.ListInstances()
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(list))
	}
}

func TestRemoveInstance(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add instance
	instance := &types.Instance{
		Name:        "postgres-14",
		ServiceType: "postgres",
		Version:     "14",
	}

	err = mgr.AddInstance(instance)
	if err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}

	// Remove instance
	err = mgr.RemoveInstance("postgres-14")
	if err != nil {
		t.Fatalf("Failed to remove instance: %v", err)
	}

	// Try to get removed instance
	_, err = mgr.GetInstance("postgres-14")
	if err == nil {
		t.Error("Expected error when getting removed instance, got nil")
	}
}

func TestSetDomain(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Set domain
	err = mgr.SetDomain("myapp.local")
	if err != nil {
		t.Fatalf("Failed to set domain: %v", err)
	}

	// Get domain
	domain, err := mgr.GetDomain()
	if err != nil {
		t.Fatalf("Failed to get domain: %v", err)
	}

	if domain != "myapp.local" {
		t.Errorf("Expected domain 'myapp.local', got '%s'", domain)
	}
}

func TestSetProtocol(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := &Manager{
		dokuDir:    filepath.Join(tmpDir, ".doku"),
		configPath: filepath.Join(tmpDir, ".doku", "config.toml"),
	}

	err := mgr.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Set protocol to http
	err = mgr.SetProtocol("http")
	if err != nil {
		t.Fatalf("Failed to set protocol: %v", err)
	}

	// Get protocol
	protocol, err := mgr.GetProtocol()
	if err != nil {
		t.Fatalf("Failed to get protocol: %v", err)
	}

	if protocol != "http" {
		t.Errorf("Expected protocol 'http', got '%s'", protocol)
	}

	// Try invalid protocol
	err = mgr.SetProtocol("ftp")
	if err == nil {
		t.Error("Expected error for invalid protocol, got nil")
	}
}
