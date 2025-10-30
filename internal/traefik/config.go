package traefik

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config represents Traefik configuration
type Config struct {
	Domain             string
	Protocol           string
	HTTPPort           int
	HTTPSPort          int
	DashboardEnabled   bool
	CertificatePath    string
	CertificateKeyPath string
}

// GenerateConfig generates Traefik configuration file
func (m *Manager) GenerateConfig() error {
	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	config := Config{
		Domain:           m.domain,
		Protocol:         m.protocol,
		HTTPPort:         80,
		HTTPSPort:        443,
		DashboardEnabled: true,
	}

	// Set certificate paths if using HTTPS
	if m.protocol == "https" {
		config.CertificatePath = filepath.Join("/certs", fmt.Sprintf("%s.pem", m.domain))
		config.CertificateKeyPath = filepath.Join("/certs", fmt.Sprintf("%s-key.pem", m.domain))
	}

	// Generate configuration content
	content := m.generateConfigContent(config)

	// Write configuration file
	configPath := filepath.Join(m.configDir, "traefik.yml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Traefik configuration generated: %s\n", configPath)
	return nil
}

// generateConfigContent generates the YAML content for Traefik configuration
func (m *Manager) generateConfigContent(config Config) string {
	var content string

	// API and Dashboard configuration
	content += "# Traefik Configuration for Doku\n\n"
	content += "api:\n"
	if config.DashboardEnabled {
		content += "  dashboard: true\n"
		content += "  insecure: false\n"
	} else {
		content += "  dashboard: false\n"
	}
	content += "\n"

	// Entry Points configuration
	content += "entryPoints:\n"

	// HTTP entry point
	content += "  web:\n"
	content += fmt.Sprintf("    address: \":%d\"\n", config.HTTPPort)

	if config.Protocol == "https" {
		// Redirect HTTP to HTTPS
		content += "    http:\n"
		content += "      redirections:\n"
		content += "        entryPoint:\n"
		content += "          to: websecure\n"
		content += "          scheme: https\n"
		content += "\n"

		// HTTPS entry point
		content += "  websecure:\n"
		content += fmt.Sprintf("    address: \":%d\"\n", config.HTTPSPort)
		content += "    http:\n"
		content += "      tls:\n"
		content += "        certificates:\n"
		content += fmt.Sprintf("          - certFile: %s\n", config.CertificatePath)
		content += fmt.Sprintf("            keyFile: %s\n", config.CertificateKeyPath)
		content += "\n"
	} else {
		content += "\n"
	}

	// Docker provider configuration
	content += "providers:\n"
	content += "  docker:\n"
	content += "    endpoint: \"unix:///var/run/docker.sock\"\n"
	content += "    exposedByDefault: false\n"
	content += "    network: \"doku-network\"\n"
	content += "    watch: true\n"
	content += "\n"

	// Logging
	content += "log:\n"
	content += "  level: INFO\n"
	content += "\n"

	// Access logs
	content += "accessLog:\n"
	content += "  bufferingSize: 100\n"
	content += "\n"

	return content
}

// GenerateDynamicConfig generates dynamic configuration for Traefik
func (m *Manager) GenerateDynamicConfig() error {
	dynamicConfigPath := filepath.Join(m.configDir, "dynamic.yml")

	content := "# Traefik Dynamic Configuration\n\n"
	content += "http:\n"

	// Dashboard router
	if m.protocol == "https" {
		content += "  routers:\n"
		content += "    dashboard:\n"
		content += fmt.Sprintf("      rule: \"Host(`traefik.%s`)\"\n", m.domain)
		content += "      service: api@internal\n"
		content += "      entryPoints:\n"
		content += "        - websecure\n"
		content += "      tls: {}\n"
	} else {
		content += "  routers:\n"
		content += "    dashboard:\n"
		content += fmt.Sprintf("      rule: \"Host(`traefik.%s`)\"\n", m.domain)
		content += "      service: api@internal\n"
		content += "      entryPoints:\n"
		content += "        - web\n"
	}

	if err := os.WriteFile(dynamicConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write dynamic config: %w", err)
	}

	return nil
}

// ValidateConfig validates the Traefik configuration
func (m *Manager) ValidateConfig() error {
	configPath := filepath.Join(m.configDir, "traefik.yml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Basic validation - check if file is readable
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if len(content) == 0 {
		return fmt.Errorf("config file is empty")
	}

	// Check for required sections
	contentStr := string(content)
	required := []string{"entryPoints", "providers", "docker"}

	for _, section := range required {
		if !contains(contentStr, section) {
			return fmt.Errorf("config missing required section: %s", section)
		}
	}

	return nil
}

// GetConfig reads and returns the current Traefik configuration
func (m *Manager) GetConfig() (string, error) {
	configPath := filepath.Join(m.configDir, "traefik.yml")

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	return string(content), nil
}

// BackupConfig creates a backup of the current configuration
func (m *Manager) BackupConfig() (string, error) {
	configPath := filepath.Join(m.configDir, "traefik.yml")
	backupPath := configPath + ".backup"

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupPath, nil
}

// RestoreConfig restores configuration from backup
func (m *Manager) RestoreConfig(backupPath string) error {
	configPath := filepath.Join(m.configDir, "traefik.yml")

	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
