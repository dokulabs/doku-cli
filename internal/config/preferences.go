package config

import (
	"fmt"

	"github.com/dokulabs/doku-cli/pkg/types"
)

// Preferences provides convenient access to preference settings
type Preferences struct {
	manager *Manager
}

// NewPreferences creates a new Preferences helper
func NewPreferences(manager *Manager) *Preferences {
	return &Preferences{
		manager: manager,
	}
}

// Domain returns the configured domain
func (p *Preferences) Domain() (string, error) {
	return p.manager.GetDomain()
}

// Protocol returns the configured protocol (http or https)
func (p *Preferences) Protocol() (string, error) {
	return p.manager.GetProtocol()
}

// SetDomain sets a new domain
func (p *Preferences) SetDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	return p.manager.SetDomain(domain)
}

// SetProtocol sets the protocol
func (p *Preferences) SetProtocol(protocol string) error {
	return p.manager.SetProtocol(protocol)
}

// CatalogVersion returns the current catalog version
func (p *Preferences) CatalogVersion() (string, error) {
	config, err := p.manager.Get()
	if err != nil {
		return "", err
	}
	return config.Preferences.CatalogVersion, nil
}

// DNSSetup returns the DNS setup method
func (p *Preferences) DNSSetup() (string, error) {
	config, err := p.manager.Get()
	if err != nil {
		return "", err
	}
	return config.Preferences.DNSSetup, nil
}

// SetDNSSetup sets the DNS setup method
func (p *Preferences) SetDNSSetup(method string) error {
	return p.manager.Update(func(c *types.Config) error {
		c.Preferences.DNSSetup = method
		return nil
	})
}

// GetAll returns all preferences
func (p *Preferences) GetAll() (*types.PreferencesConfig, error) {
	config, err := p.manager.Get()
	if err != nil {
		return nil, err
	}
	return &config.Preferences, nil
}

// GetDefaultMemoryLimit returns the default memory limit for services
func (p *Preferences) GetDefaultMemoryLimit() string {
	// Default to 512MB if not configured
	return "512m"
}

// GetDefaultCPULimit returns the default CPU limit for services
func (p *Preferences) GetDefaultCPULimit() string {
	// Default to 1.0 (1 core) if not configured
	return "1.0"
}

// BuildServiceURL constructs a URL for a service instance
func (p *Preferences) BuildServiceURL(instanceName string, useHTTPS bool) (string, error) {
	domain, err := p.Domain()
	if err != nil {
		return "", err
	}

	protocol := "http"
	if useHTTPS {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s.%s", protocol, instanceName, domain), nil
}

// BuildInternalHostname constructs the internal Docker hostname for a service
func (p *Preferences) BuildInternalHostname(instanceName string) (string, error) {
	domain, err := p.Domain()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", instanceName, domain), nil
}
