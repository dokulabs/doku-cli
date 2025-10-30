package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ResolverManager handles macOS /etc/resolver configuration
type ResolverManager struct {
	resolverDir string
}

// NewResolverManager creates a new resolver manager
func NewResolverManager() *ResolverManager {
	return &ResolverManager{
		resolverDir: "/etc/resolver",
	}
}

// IsMacOS checks if the system is macOS
func (rm *ResolverManager) IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// SetupResolver creates a resolver configuration for the domain (macOS only)
func (rm *ResolverManager) SetupResolver(domain string) error {
	if !rm.IsMacOS() {
		return fmt.Errorf("resolver setup is only supported on macOS")
	}

	// Ensure resolver directory exists
	if err := os.MkdirAll(rm.resolverDir, 0755); err != nil {
		return fmt.Errorf("failed to create resolver directory: %w", err)
	}

	// Create resolver file
	resolverFile := filepath.Join(rm.resolverDir, domain)
	content := "nameserver 127.0.0.1\n"

	// Write resolver configuration
	if err := os.WriteFile(resolverFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write resolver file: %w", err)
	}

	return nil
}

// RemoveResolver removes the resolver configuration for the domain
func (rm *ResolverManager) RemoveResolver(domain string) error {
	if !rm.IsMacOS() {
		return nil
	}

	resolverFile := filepath.Join(rm.resolverDir, domain)

	if err := os.Remove(resolverFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove resolver file: %w", err)
	}

	return nil
}

// HasResolver checks if a resolver configuration exists for the domain
func (rm *ResolverManager) HasResolver(domain string) bool {
	if !rm.IsMacOS() {
		return false
	}

	resolverFile := filepath.Join(rm.resolverDir, domain)
	_, err := os.Stat(resolverFile)
	return err == nil
}

// ListResolvers lists all resolver configurations
func (rm *ResolverManager) ListResolvers() ([]string, error) {
	if !rm.IsMacOS() {
		return []string{}, nil
	}

	entries, err := os.ReadDir(rm.resolverDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read resolver directory: %w", err)
	}

	var resolvers []string
	for _, entry := range entries {
		if !entry.IsDir() {
			resolvers = append(resolvers, entry.Name())
		}
	}

	return resolvers, nil
}

// GetResolverPath returns the path to the resolver file for a domain
func (rm *ResolverManager) GetResolverPath(domain string) string {
	return filepath.Join(rm.resolverDir, domain)
}
