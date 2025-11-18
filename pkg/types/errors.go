package types

import "errors"

// Common sentinel errors for better error handling throughout the application
var (
	// Service errors
	ErrServiceNotFound   = errors.New("service not found")
	ErrServiceNotRunning = errors.New("service is not running")
	ErrAlreadyRunning    = errors.New("service is already running")
	ErrAlreadyStopped    = errors.New("service is already stopped")
	ErrInvalidService    = errors.New("invalid service configuration")

	// Configuration errors
	ErrNotInitialized = errors.New("doku is not initialized")
	ErrConfigNotFound = errors.New("configuration not found")
	ErrInvalidConfig  = errors.New("invalid configuration")

	// Catalog errors
	ErrCatalogNotFound = errors.New("catalog not found")
	ErrVersionNotFound = errors.New("version not found")
	ErrInvalidCatalog  = errors.New("invalid catalog format")

	// Project errors
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidProject  = errors.New("invalid project configuration")
	ErrProjectExists   = errors.New("project already exists")

	// Container errors
	ErrContainerNotFound = errors.New("container not found")
	ErrContainerFailed   = errors.New("container failed")

	// Network errors
	ErrNetworkNotFound = errors.New("network not found")
	ErrNetworkFailed   = errors.New("network operation failed")

	// Volume errors
	ErrVolumeNotFound = errors.New("volume not found")
	ErrVolumeFailed   = errors.New("volume operation failed")
)
