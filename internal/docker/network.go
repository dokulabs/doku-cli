package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

const (
	DefaultNetworkName    = "doku-network"
	DefaultNetworkSubnet  = "172.20.0.0/16"
	DefaultNetworkGateway = "172.20.0.1"
)

// NetworkManager manages Docker networks for Doku
type NetworkManager struct {
	client *Client
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(client *Client) *NetworkManager {
	return &NetworkManager{
		client: client,
	}
}

// EnsureDokuNetwork ensures the Doku network exists
func (nm *NetworkManager) EnsureDokuNetwork(networkName, subnet, gateway string) error {
	// Use defaults if not provided
	if networkName == "" {
		networkName = DefaultNetworkName
	}
	if subnet == "" {
		subnet = DefaultNetworkSubnet
	}
	if gateway == "" {
		gateway = DefaultNetworkGateway
	}

	// Check if network already exists
	exists, err := nm.client.NetworkExists(networkName)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if exists {
		return nil
	}

	// Create network
	_, err = nm.CreateBridgeNetwork(networkName, subnet, gateway)
	if err != nil {
		return fmt.Errorf("failed to create doku network: %w", err)
	}

	return nil
}

// CreateBridgeNetwork creates a bridge network with custom subnet and gateway
func (nm *NetworkManager) CreateBridgeNetwork(name, subnet, gateway string) (string, error) {
	ipam := &network.IPAM{
		Config: []network.IPAMConfig{
			{
				Subnet:  subnet,
				Gateway: gateway,
			},
		},
	}

	options := types.NetworkCreate{
		Driver:     "bridge",
		IPAM:       ipam,
		EnableIPv6: false,
		Labels: map[string]string{
			"managed-by": "doku",
			"doku.network": "true",
		},
	}

	networkID, err := nm.client.NetworkCreate(name, options)
	if err != nil {
		return "", err
	}

	return networkID, nil
}

// GetNetworkByName retrieves a network by name
func (nm *NetworkManager) GetNetworkByName(name string) (*types.NetworkResource, error) {
	networks, err := nm.client.NetworkList()
	if err != nil {
		return nil, err
	}

	for _, network := range networks {
		if network.Name == name {
			return &network, nil
		}
	}

	return nil, fmt.Errorf("network not found: %s", name)
}

// ConnectContainer connects a container to a network
func (nm *NetworkManager) ConnectContainer(networkName, containerID string) error {
	network, err := nm.GetNetworkByName(networkName)
	if err != nil {
		return err
	}

	return nm.client.NetworkConnect(network.ID, containerID)
}

// DisconnectContainer disconnects a container from a network
func (nm *NetworkManager) DisconnectContainer(networkName, containerID string, force bool) error {
	network, err := nm.GetNetworkByName(networkName)
	if err != nil {
		return err
	}

	return nm.client.NetworkDisconnect(network.ID, containerID, force)
}

// RemoveDokuNetwork removes the Doku network
func (nm *NetworkManager) RemoveDokuNetwork(networkName string) error {
	if networkName == "" {
		networkName = DefaultNetworkName
	}

	network, err := nm.GetNetworkByName(networkName)
	if err != nil {
		// Network doesn't exist, nothing to remove
		if err.Error() == fmt.Sprintf("network not found: %s", networkName) {
			return nil
		}
		return err
	}

	return nm.client.NetworkRemove(network.ID)
}

// GetNetworkInfo returns information about a network
func (nm *NetworkManager) GetNetworkInfo(networkName string) (types.NetworkResource, error) {
	return nm.client.NetworkInspect(networkName)
}

// ListDokuNetworks lists all networks managed by Doku
func (nm *NetworkManager) ListDokuNetworks() ([]types.NetworkResource, error) {
	allNetworks, err := nm.client.NetworkList()
	if err != nil {
		return nil, err
	}

	dokuNetworks := []types.NetworkResource{}
	for _, network := range allNetworks {
		// Check if network is managed by Doku
		if labels := network.Labels; labels != nil {
			if managedBy, ok := labels["managed-by"]; ok && managedBy == "doku" {
				dokuNetworks = append(dokuNetworks, network)
			}
		}
	}

	return dokuNetworks, nil
}

// GetConnectedContainers returns a list of containers connected to a network
func (nm *NetworkManager) GetConnectedContainers(networkName string) ([]string, error) {
	network, err := nm.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	containerIDs := make([]string, 0, len(network.Containers))
	for containerID := range network.Containers {
		containerIDs = append(containerIDs, containerID)
	}

	return containerIDs, nil
}

// IsNetworkHealthy checks if the network is healthy
func (nm *NetworkManager) IsNetworkHealthy(networkName string) (bool, error) {
	exists, err := nm.client.NetworkExists(networkName)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	network, err := nm.GetNetworkByName(networkName)
	if err != nil {
		return false, err
	}

	// Basic health check: network exists and has proper driver
	return network.Driver == "bridge", nil
}
