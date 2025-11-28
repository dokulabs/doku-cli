package docker

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/api/types/network"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) *Client {
	t.Helper()

	client, err := NewClient()
	if err != nil {
		t.Skipf("Skipping test: Docker client creation failed: %v", err)
		return nil
	}

	if err := client.Ping(); err != nil {
		client.Close()
		t.Skipf("Skipping test: Docker daemon not available: %v", err)
		return nil
	}

	return client
}

// TestNewClient tests creating a new Docker client
func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		// It's okay if Docker is not available
		t.Logf("NewClient failed (Docker may not be available): %v", err)
		return
	}
	defer client.Close()

	if client.cli == nil {
		t.Error("client.cli should not be nil")
	}
	if client.ctx == nil {
		t.Error("client.ctx should not be nil")
	}
}

// TestNewClientSetsBuildKit tests that NewClient sets DOCKER_BUILDKIT
func TestNewClientSetsBuildKit(t *testing.T) {
	// Unset the variable first
	os.Unsetenv("DOCKER_BUILDKIT")

	_, err := NewClient()
	if err != nil {
		t.Logf("NewClient failed (Docker may not be available): %v", err)
		return
	}

	// Check if DOCKER_BUILDKIT was set
	if os.Getenv("DOCKER_BUILDKIT") != "1" {
		t.Error("NewClient should set DOCKER_BUILDKIT=1")
	}
}

// TestClientClose tests closing the client
func TestClientClose(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Logf("NewClient failed (Docker may not be available): %v", err)
		return
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Closing a nil client should also not error
	nilClient := &Client{}
	err = nilClient.Close()
	if err != nil {
		t.Errorf("Close on nil client should not return error: %v", err)
	}
}

// TestClientPing tests pinging the Docker daemon
func TestClientPing(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	err := client.Ping()
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

// TestClientVersion tests getting Docker version
func TestClientVersion(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	version, err := client.Version()
	if err != nil {
		t.Errorf("Version failed: %v", err)
	}

	if version.Version == "" {
		t.Error("Version should not be empty")
	}
	if version.APIVersion == "" {
		t.Error("APIVersion should not be empty")
	}
}

// TestClientIsDockerAvailable tests the IsDockerAvailable method
func TestClientIsDockerAvailable(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Logf("NewClient failed (Docker may not be available): %v", err)
		return
	}
	defer client.Close()

	// This should return true if Docker is running, false otherwise
	available := client.IsDockerAvailable()
	t.Logf("Docker available: %v", available)
}

// TestContainerList tests listing containers
func TestContainerList(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	// List all containers
	containers, err := client.ContainerList(true)
	if err != nil {
		t.Errorf("ContainerList failed: %v", err)
	}

	// Should return a slice (may be empty)
	if containers == nil {
		t.Error("ContainerList should return non-nil slice")
	}
}

// TestContainerExists tests checking if a container exists
func TestContainerExists(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Check for a container that definitely doesn't exist
	exists, err := client.ContainerExists("doku-test-nonexistent-container-12345")
	if err != nil {
		t.Errorf("ContainerExists failed: %v", err)
	}
	if exists {
		t.Error("Non-existent container should not exist")
	}
}

// TestImageList tests listing images
func TestImageList(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	images, err := client.ImageList()
	if err != nil {
		t.Errorf("ImageList failed: %v", err)
	}

	// Should return a slice (may be empty)
	if images == nil {
		t.Error("ImageList should return non-nil slice")
	}
}

// TestImageExists tests checking if an image exists
func TestImageExists(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Check for an image that definitely doesn't exist
	exists, err := client.ImageExists("doku-test-nonexistent-image:12345")
	if err != nil {
		t.Errorf("ImageExists failed: %v", err)
	}
	if exists {
		t.Error("Non-existent image should not exist")
	}
}

// TestVolumeList tests listing volumes
func TestVolumeList(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	volumes, err := client.VolumeList()
	if err != nil {
		t.Errorf("VolumeList failed: %v", err)
	}

	// Should return a slice (may be empty)
	if volumes == nil {
		t.Error("VolumeList should return non-nil slice")
	}
}

// TestVolumeExists tests checking if a volume exists
func TestVolumeExists(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Check for a volume that definitely doesn't exist
	exists, err := client.VolumeExists("doku-test-nonexistent-volume-12345")
	if err != nil {
		t.Errorf("VolumeExists failed: %v", err)
	}
	if exists {
		t.Error("Non-existent volume should not exist")
	}
}

// TestNetworkList tests listing networks
func TestNetworkList(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	networks, err := client.NetworkList()
	if err != nil {
		t.Errorf("NetworkList failed: %v", err)
	}

	// Should return a slice with at least the default networks
	if networks == nil {
		t.Error("NetworkList should return non-nil slice")
	}
}

// TestNetworkExists tests checking if a network exists
func TestNetworkExists(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Bridge network should exist
	exists, err := client.NetworkExists("bridge")
	if err != nil {
		t.Errorf("NetworkExists failed: %v", err)
	}
	if !exists {
		t.Error("Bridge network should exist")
	}

	// Check for a network that definitely doesn't exist
	exists, err = client.NetworkExists("doku-test-nonexistent-network-12345")
	if err != nil {
		t.Errorf("NetworkExists failed: %v", err)
	}
	if exists {
		t.Error("Non-existent network should not exist")
	}
}

// TestListContainersWithContext tests listing containers with context
func TestListContainersWithContext(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	containers, err := client.ListContainers(ctx)
	if err != nil {
		t.Errorf("ListContainers failed: %v", err)
	}

	if containers == nil {
		t.Error("ListContainers should return non-nil slice")
	}
}

// TestListContainersByLabel tests listing containers by label
func TestListContainersByLabel(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	containers, err := client.ListContainersByLabel(ctx, "com.doku.test", "true")
	if err != nil {
		t.Errorf("ListContainersByLabel failed: %v", err)
	}

	// Should return empty slice since no containers have this label
	if containers == nil {
		t.Error("ListContainersByLabel should return non-nil slice")
	}
}

// TestListVolumesWithContext tests listing volumes with context
func TestListVolumesWithContext(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	volumes, err := client.ListVolumes(ctx)
	if err != nil {
		t.Errorf("ListVolumes failed: %v", err)
	}

	if volumes == nil {
		t.Error("ListVolumes should return non-nil slice")
	}
}

// TestListVolumesByLabel tests listing volumes by label
func TestListVolumesByLabel(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	ctx := context.Background()
	volumes, err := client.ListVolumesByLabel(ctx, "com.doku.test", "true")
	if err != nil {
		t.Errorf("ListVolumesByLabel failed: %v", err)
	}

	// Should return empty slice since no volumes have this label
	if volumes == nil {
		t.Error("ListVolumesByLabel should return non-nil slice")
	}
}

// Integration tests that create/modify Docker resources
// These tests create actual Docker resources and clean them up

// TestVolumeCreateAndRemove tests creating and removing a volume
func TestVolumeCreateAndRemove(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	volumeName := "doku-test-volume-create"

	// Clean up in case previous test failed
	client.VolumeRemove(volumeName, true)

	// Create volume
	vol, err := client.VolumeCreate(volumeName, map[string]string{
		"com.doku.test": "true",
	})
	if err != nil {
		t.Fatalf("VolumeCreate failed: %v", err)
	}

	if vol.Name != volumeName {
		t.Errorf("Volume name mismatch: got %s, want %s", vol.Name, volumeName)
	}

	// Verify it exists
	exists, err := client.VolumeExists(volumeName)
	if err != nil {
		t.Errorf("VolumeExists failed: %v", err)
	}
	if !exists {
		t.Error("Created volume should exist")
	}

	// Inspect volume
	inspected, err := client.VolumeInspect(volumeName)
	if err != nil {
		t.Errorf("VolumeInspect failed: %v", err)
	}
	if inspected.Name != volumeName {
		t.Errorf("Inspected volume name mismatch: got %s, want %s", inspected.Name, volumeName)
	}

	// Remove volume
	err = client.VolumeRemove(volumeName, true)
	if err != nil {
		t.Errorf("VolumeRemove failed: %v", err)
	}

	// Verify it's gone
	exists, err = client.VolumeExists(volumeName)
	if err != nil {
		t.Errorf("VolumeExists failed: %v", err)
	}
	if exists {
		t.Error("Removed volume should not exist")
	}
}

// TestNetworkCreateAndRemove tests creating and removing a network
func TestNetworkCreateAndRemove(t *testing.T) {
	client := skipIfNoDocker(t)
	if client == nil {
		return
	}
	defer client.Close()

	networkName := "doku-test-network-create"

	// Clean up in case previous test failed
	client.NetworkRemove(networkName)

	// Create network
	_, err := client.NetworkCreate(networkName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"com.doku.test": "true",
		},
	})
	if err != nil {
		t.Fatalf("NetworkCreate failed: %v", err)
	}

	// Verify it exists
	exists, err := client.NetworkExists(networkName)
	if err != nil {
		t.Errorf("NetworkExists failed: %v", err)
	}
	if !exists {
		t.Error("Created network should exist")
	}

	// Inspect network
	inspected, err := client.NetworkInspect(networkName)
	if err != nil {
		t.Errorf("NetworkInspect failed: %v", err)
	}
	if inspected.Name != networkName {
		t.Errorf("Inspected network name mismatch: got %s, want %s", inspected.Name, networkName)
	}

	// Remove network
	err = client.NetworkRemove(networkName)
	if err != nil {
		t.Errorf("NetworkRemove failed: %v", err)
	}

	// Verify it's gone
	exists, err = client.NetworkExists(networkName)
	if err != nil {
		t.Errorf("NetworkExists failed: %v", err)
	}
	if exists {
		t.Error("Removed network should not exist")
	}
}
