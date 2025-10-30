package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

// ValidateCertificate validates a certificate file
func ValidateCertificate(certPath string) error {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}

	return nil
}

// GetCertificateExpiry returns the expiration date of a certificate
func GetCertificateExpiry(certPath string) (time.Time, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.NotAfter, nil
}

// GetCertificateDaysRemaining returns the number of days until certificate expires
func GetCertificateDaysRemaining(certPath string) (int, error) {
	expiry, err := GetCertificateExpiry(certPath)
	if err != nil {
		return 0, err
	}

	duration := time.Until(expiry)
	days := int(duration.Hours() / 24)

	return days, nil
}

// IsCertificateExpiringSoon checks if certificate expires within the given days
func IsCertificateExpiringSoon(certPath string, days int) (bool, error) {
	remaining, err := GetCertificateDaysRemaining(certPath)
	if err != nil {
		return false, err
	}

	return remaining <= days, nil
}

// GetCertificateDomains returns the domains covered by the certificate
func GetCertificateDomains(certPath string) ([]string, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	domains := []string{cert.Subject.CommonName}
	domains = append(domains, cert.DNSNames...)

	return domains, nil
}

// CopyCertificates copies certificate files to a destination directory
func CopyCertificates(certPath, keyPath, destDir string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy certificate
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	destCertPath := destDir + "/cert.pem"
	if err := os.WriteFile(destCertPath, certData, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Copy key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}

	destKeyPath := destDir + "/key.pem"
	if err := os.WriteFile(destKeyPath, keyData, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return nil
}

// FormatCertificateInfo formats certificate information for display
func FormatCertificateInfo(certPath string) (string, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	info := fmt.Sprintf("Certificate Information:\n")
	info += fmt.Sprintf("  Subject: %s\n", cert.Subject.CommonName)
	info += fmt.Sprintf("  Issuer: %s\n", cert.Issuer.CommonName)
	info += fmt.Sprintf("  Valid From: %s\n", cert.NotBefore.Format("2006-01-02 15:04:05"))
	info += fmt.Sprintf("  Valid Until: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05"))
	info += fmt.Sprintf("  DNS Names: %v\n", cert.DNSNames)

	days, _ := GetCertificateDaysRemaining(certPath)
	info += fmt.Sprintf("  Days Remaining: %d\n", days)

	return info, nil
}
