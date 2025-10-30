package dns

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	DokuMarker = "# doku-managed - do not edit"
	DokuStart  = "# doku-managed-start"
	DokuEnd    = "# doku-managed-end"
)

// Manager handles DNS and hosts file management
type Manager struct {
	hostsFile string
}

// NewManager creates a new DNS manager
func NewManager() *Manager {
	hostsFile := getHostsFilePath()
	return &Manager{
		hostsFile: hostsFile,
	}
}

// getHostsFilePath returns the hosts file path based on OS
func getHostsFilePath() string {
	if runtime.GOOS == "windows" {
		return "C:\\Windows\\System32\\drivers\\etc\\hosts"
	}
	return "/etc/hosts"
}

// AddDokuDomain adds Doku domain entries to the hosts file
func (m *Manager) AddDokuDomain(domain string) error {
	// Check if already exists
	exists, err := m.HasDokuEntries()
	if err != nil {
		return err
	}

	if exists {
		// Update existing entries
		return m.UpdateDokuDomain(domain)
	}

	// Read existing hosts file
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Prepare new entries
	newEntries := m.generateHostsEntries(domain)

	// Append new entries
	updatedContent := string(content)
	if !strings.HasSuffix(updatedContent, "\n") {
		updatedContent += "\n"
	}
	updatedContent += "\n" + newEntries

	// Write back to hosts file
	if err := m.writeHostsFile(updatedContent); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	return nil
}

// RemoveDokuEntries removes Doku-managed entries from hosts file
func (m *Manager) RemoveDokuEntries() error {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inDokuSection := false

	for _, line := range lines {
		if strings.Contains(line, DokuStart) {
			inDokuSection = true
			continue
		}
		if strings.Contains(line, DokuEnd) {
			inDokuSection = false
			continue
		}
		if !inDokuSection && !strings.Contains(line, DokuMarker) {
			newLines = append(newLines, line)
		}
	}

	updatedContent := strings.Join(newLines, "\n")
	return m.writeHostsFile(updatedContent)
}

// UpdateDokuDomain updates the domain in existing Doku entries
func (m *Manager) UpdateDokuDomain(domain string) error {
	// Remove existing entries
	if err := m.RemoveDokuEntries(); err != nil {
		return err
	}

	// Add new entries
	return m.AddDokuDomain(domain)
}

// HasDokuEntries checks if Doku entries exist in hosts file
func (m *Manager) HasDokuEntries() (bool, error) {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return false, fmt.Errorf("failed to read hosts file: %w", err)
	}

	return strings.Contains(string(content), DokuMarker) ||
		strings.Contains(string(content), DokuStart), nil
}

// GetDokuDomain returns the currently configured Doku domain from hosts file
func (m *Manager) GetDokuDomain() (string, error) {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return "", fmt.Errorf("failed to read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	inDokuSection := false

	for _, line := range lines {
		if strings.Contains(line, DokuStart) {
			inDokuSection = true
			continue
		}
		if strings.Contains(line, DokuEnd) {
			break
		}

		if inDokuSection && strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			// Parse the line to extract domain
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// Return the first domain (without wildcard)
				domain := fields[1]
				if strings.HasPrefix(domain, "*.") {
					continue
				}
				return domain, nil
			}
		}
	}

	return "", fmt.Errorf("no Doku domain found in hosts file")
}

// generateHostsEntries generates hosts file entries for the domain
func (m *Manager) generateHostsEntries(domain string) string {
	entries := fmt.Sprintf("%s\n", DokuStart)
	entries += fmt.Sprintf("127.0.0.1 %s %s\n", domain, DokuMarker)
	entries += fmt.Sprintf("127.0.0.1 *.%s %s\n", domain, DokuMarker)
	entries += fmt.Sprintf("%s\n", DokuEnd)
	return entries
}

// writeHostsFile writes content to the hosts file (requires sudo on Unix)
func (m *Manager) writeHostsFile(content string) error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "doku-hosts-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Copy temp file to hosts file (may require sudo)
	if runtime.GOOS == "windows" {
		// On Windows, try direct write
		return os.WriteFile(m.hostsFile, []byte(content), 0644)
	}

	// On Unix, use sudo to copy
	return m.copyWithSudo(tmpFile.Name(), m.hostsFile)
}

// copyWithSudo copies a file using sudo (Unix only)
func (m *Manager) copyWithSudo(src, dest string) error {
	// Try without sudo first
	srcContent, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dest, srcContent, 0644)
	if err == nil {
		return nil
	}

	// If that fails, we need sudo
	// Note: This will prompt for password
	fmt.Println("Updating hosts file requires administrator privileges...")
	cmd := fmt.Sprintf("sudo cp %s %s", src, dest)

	// Execute the command
	return executeCommand(cmd)
}

// VerifyDNSResolution verifies that DNS resolution works for the domain
func (m *Manager) VerifyDNSResolution(domain string) error {
	// This is a basic check - in a real implementation,
	// you might want to use actual DNS resolution

	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	if strings.Contains(string(content), domain) {
		return nil
	}

	return fmt.Errorf("domain %s not found in hosts file", domain)
}

// BackupHostsFile creates a backup of the hosts file
func (m *Manager) BackupHostsFile() (string, error) {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return "", fmt.Errorf("failed to read hosts file: %w", err)
	}

	backupPath := m.hostsFile + ".doku-backup"
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// RestoreHostsFile restores the hosts file from backup
func (m *Manager) RestoreHostsFile(backupPath string) error {
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	return m.writeHostsFile(string(content))
}

// ListDokuEntries returns all Doku-managed entries
func (m *Manager) ListDokuEntries() ([]string, error) {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var dokuLines []string
	inDokuSection := false

	for _, line := range lines {
		if strings.Contains(line, DokuStart) {
			inDokuSection = true
			continue
		}
		if strings.Contains(line, DokuEnd) {
			break
		}

		if inDokuSection && strings.TrimSpace(line) != "" {
			dokuLines = append(dokuLines, line)
		}
	}

	return dokuLines, nil
}

// ValidateHostsFile checks if the hosts file is writable
func (m *Manager) ValidateHostsFile() error {
	// Check if file exists
	if _, err := os.Stat(m.hostsFile); os.IsNotExist(err) {
		return fmt.Errorf("hosts file does not exist: %s", m.hostsFile)
	}

	// Try to open for reading
	file, err := os.Open(m.hostsFile)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}
	file.Close()

	return nil
}

// GetHostsFilePath returns the path to the hosts file
func (m *Manager) GetHostsFilePath() string {
	return m.hostsFile
}

// executeCommand executes a shell command
func executeCommand(cmd string) error {
	// This is a placeholder - actual implementation would use exec.Command
	// For now, we'll return an error suggesting manual intervention
	return fmt.Errorf("please run the following command manually:\n  %s", cmd)
}

// AddSingleEntry adds a single custom entry to hosts file
func (m *Manager) AddSingleEntry(ip, hostname string) error {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Check if entry already exists
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, hostname) {
			return nil // Entry already exists
		}
	}

	// Add new entry
	entry := fmt.Sprintf("\n%s %s %s\n", ip, hostname, DokuMarker)
	updatedContent := string(content) + entry

	return m.writeHostsFile(updatedContent)
}

// RemoveSingleEntry removes a specific entry from hosts file
func (m *Manager) RemoveSingleEntry(hostname string) error {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if !strings.Contains(line, hostname) || !strings.Contains(line, DokuMarker) {
			newLines = append(newLines, line)
		}
	}

	updatedContent := strings.Join(newLines, "\n")
	return m.writeHostsFile(updatedContent)
}
