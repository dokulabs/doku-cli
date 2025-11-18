// Package constants defines common constants used throughout the Doku CLI application
package constants

const (
	// Container names
	TraefikContainerName = "doku-traefik"

	// Network names
	DokuNetworkName = "doku-network"

	// Default values
	DefaultDomain   = "doku.local"
	DefaultProtocol = "https"

	// Timeouts (in seconds)
	DefaultContainerTimeout = 10

	// Buffer sizes
	DefaultLogBufferSize = 4096
)
