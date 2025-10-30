package certs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Manager handles certificate generation with mkcert
type Manager struct {
	certsDir string
	domain   string
}

// NewManager creates a new certificate manager
func NewManager(certsDir, domain string) *Manager {
	return &Manager{
		certsDir: certsDir,
		domain:   domain,
	}
}

// IsMkcertInstalled checks if mkcert is installed on the system
func (m *Manager) IsMkcertInstalled() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}

// InstallMkcert attempts to install mkcert
func (m *Manager) InstallMkcert() error {
	if m.IsMkcertInstalled() {
		return nil
	}

	goos := runtime.GOOS

	switch goos {
	case "darwin":
		return m.installMkcertMacOS()
	case "linux":
		return m.installMkcertLinux()
	case "windows":
		return m.installMkcertWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", goos)
	}
}

// installMkcertMacOS installs mkcert on macOS using Homebrew
func (m *Manager) installMkcertMacOS() error {
	// Check if Homebrew is installed
	_, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("Homebrew not found. Please install Homebrew first: https://brew.sh")
	}

	fmt.Println("Installing mkcert via Homebrew...")
	cmd := exec.Command("brew", "install", "mkcert")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}

	return nil
}

// installMkcertLinux installs mkcert on Linux
func (m *Manager) installMkcertLinux() error {
	// Try to detect package manager
	if _, err := exec.LookPath("apt-get"); err == nil {
		return m.installWithApt()
	} else if _, err := exec.LookPath("yum"); err == nil {
		return m.installWithYum()
	} else if _, err := exec.LookPath("pacman"); err == nil {
		return m.installWithPacman()
	}

	// Fall back to manual installation
	return m.installMkcertManual()
}

// installWithApt installs mkcert using apt (Debian/Ubuntu)
func (m *Manager) installWithApt() error {
	fmt.Println("Installing mkcert via apt...")

	// Install certutil dependency
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "libnss3-tools")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install libnss3-tools: %w", err)
	}

	// Install mkcert from binary
	return m.installMkcertManual()
}

// installWithYum installs mkcert using yum (RHEL/CentOS)
func (m *Manager) installWithYum() error {
	fmt.Println("Installing mkcert via yum...")

	// Install certutil dependency
	cmd := exec.Command("sudo", "yum", "install", "-y", "nss-tools")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install nss-tools: %w", err)
	}

	return m.installMkcertManual()
}

// installWithPacman installs mkcert using pacman (Arch Linux)
func (m *Manager) installWithPacman() error {
	fmt.Println("Installing mkcert via pacman...")
	cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "mkcert")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}

	return nil
}

// installMkcertManual installs mkcert from GitHub releases
func (m *Manager) installMkcertManual() error {
	fmt.Println("Installing mkcert from GitHub releases...")

	arch := runtime.GOARCH
	goos := runtime.GOOS

	// Determine download URL based on OS and architecture
	var url string
	switch goos {
	case "linux":
		if arch == "amd64" {
			url = "https://github.com/FiloSottile/mkcert/releases/latest/download/mkcert-v1.4.4-linux-amd64"
		} else if arch == "arm64" {
			url = "https://github.com/FiloSottile/mkcert/releases/latest/download/mkcert-v1.4.4-linux-arm64"
		} else {
			return fmt.Errorf("unsupported architecture: %s", arch)
		}
	default:
		return fmt.Errorf("manual installation not supported for %s", goos)
	}

	// Download mkcert binary
	cmd := exec.Command("curl", "-L", "-o", "/usr/local/bin/mkcert", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download mkcert: %w", err)
	}

	// Make it executable
	if err := os.Chmod("/usr/local/bin/mkcert", 0755); err != nil {
		return fmt.Errorf("failed to make mkcert executable: %w", err)
	}

	return nil
}

// installMkcertWindows installs mkcert on Windows
func (m *Manager) installMkcertWindows() error {
	// Check if Chocolatey is installed
	_, err := exec.LookPath("choco")
	if err != nil {
		return fmt.Errorf("Chocolatey not found. Please install mkcert manually: https://github.com/FiloSottile/mkcert#windows")
	}

	fmt.Println("Installing mkcert via Chocolatey...")
	cmd := exec.Command("choco", "install", "mkcert", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}

	return nil
}

// InstallCA installs the mkcert root CA into system trust stores
func (m *Manager) InstallCA() error {
	if !m.IsMkcertInstalled() {
		return fmt.Errorf("mkcert is not installed")
	}

	fmt.Println("Installing mkcert CA into system trust store...")
	cmd := exec.Command("mkcert", "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install CA: %w", err)
	}

	return nil
}

// GenerateCertificates generates SSL certificates for the domain
func (m *Manager) GenerateCertificates() error {
	if !m.IsMkcertInstalled() {
		return fmt.Errorf("mkcert is not installed")
	}

	// Ensure certs directory exists
	if err := os.MkdirAll(m.certsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	certFile := filepath.Join(m.certsDir, fmt.Sprintf("%s.pem", m.domain))
	keyFile := filepath.Join(m.certsDir, fmt.Sprintf("%s-key.pem", m.domain))

	// Generate certificate for domain and wildcard
	fmt.Printf("Generating SSL certificates for %s and *.%s...\n", m.domain, m.domain)

	cmd := exec.Command("mkcert",
		"-cert-file", certFile,
		"-key-file", keyFile,
		m.domain,
		fmt.Sprintf("*.%s", m.domain),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	fmt.Printf("✓ Certificates generated:\n")
	fmt.Printf("  - Certificate: %s\n", certFile)
	fmt.Printf("  - Key: %s\n", keyFile)

	return nil
}

// GetCALocation returns the path to the mkcert CA root certificate
func (m *Manager) GetCALocation() (string, error) {
	if !m.IsMkcertInstalled() {
		return "", fmt.Errorf("mkcert is not installed")
	}

	cmd := exec.Command("mkcert", "-CAROOT")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CA location: %w", err)
	}

	caRoot := strings.TrimSpace(string(output))
	return caRoot, nil
}

// GetCertificatePath returns the path to the generated certificate
func (m *Manager) GetCertificatePath() string {
	return filepath.Join(m.certsDir, fmt.Sprintf("%s.pem", m.domain))
}

// GetKeyPath returns the path to the generated private key
func (m *Manager) GetKeyPath() string {
	return filepath.Join(m.certsDir, fmt.Sprintf("%s-key.pem", m.domain))
}

// CertificatesExist checks if certificates have been generated
func (m *Manager) CertificatesExist() bool {
	certPath := m.GetCertificatePath()
	keyPath := m.GetKeyPath()

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil
}

// RegenerateCertificates removes old certificates and generates new ones
func (m *Manager) RegenerateCertificates() error {
	// Remove old certificates if they exist
	if m.CertificatesExist() {
		certPath := m.GetCertificatePath()
		keyPath := m.GetKeyPath()

		if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old certificate: %w", err)
		}

		if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old key: %w", err)
		}
	}

	return m.GenerateCertificates()
}

// UninstallCA removes the mkcert CA from system trust stores
func (m *Manager) UninstallCA() error {
	if !m.IsMkcertInstalled() {
		return fmt.Errorf("mkcert is not installed")
	}

	fmt.Println("Uninstalling mkcert CA from system trust store...")
	cmd := exec.Command("mkcert", "-uninstall")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall CA: %w", err)
	}

	return nil
}

// IsCAInstalled checks if the mkcert CA is installed
func (m *Manager) IsCAInstalled() (bool, error) {
	if !m.IsMkcertInstalled() {
		return false, nil
	}

	caRoot, err := m.GetCALocation()
	if err != nil {
		return false, err
	}

	// Check if CA root directory exists and has certificates
	caCertPath := filepath.Join(caRoot, "rootCA.pem")
	_, err = os.Stat(caCertPath)

	return err == nil, nil
}

// GetCertInfo returns information about the generated certificates
func (m *Manager) GetCertInfo() (map[string]interface{}, error) {
	if !m.CertificatesExist() {
		return nil, fmt.Errorf("certificates do not exist")
	}

	certPath := m.GetCertificatePath()
	keyPath := m.GetKeyPath()

	certInfo, err := os.Stat(certPath)
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"certificate_path": certPath,
		"key_path":         keyPath,
		"domain":           m.domain,
		"wildcard":         fmt.Sprintf("*.%s", m.domain),
		"created_at":       certInfo.ModTime(),
	}

	return info, nil
}
