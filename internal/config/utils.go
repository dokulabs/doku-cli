package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ValidateDomain validates a domain name
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Basic domain validation regex
	// Allows: example.local, doku.dev, my-domain.local
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format: %s", domain)
	}

	// Domain should not start or end with hyphen
	if strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		return fmt.Errorf("domain cannot start or end with hyphen: %s", domain)
	}

	// Check length
	if len(domain) > 253 {
		return fmt.Errorf("domain is too long (max 253 characters): %s", domain)
	}

	return nil
}

// ValidateInstanceName validates an instance name
func ValidateInstanceName(name string) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}

	// Instance names should be DNS-safe (must start with letter)
	nameRegex := regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid instance name: %s (must start with letter, contain only lowercase letters, numbers, and hyphens)", name)
	}

	// Check length
	if len(name) > 63 {
		return fmt.Errorf("instance name is too long (max 63 characters): %s", name)
	}

	// Reserved names
	reserved := []string{"traefik", "doku", "localhost", "docker"}
	for _, r := range reserved {
		if name == r {
			return fmt.Errorf("instance name '%s' is reserved", name)
		}
	}

	return nil
}

// NormalizeInstanceName converts a string to a valid instance name
func NormalizeInstanceName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Replace multiple consecutive hyphens with a single hyphen
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")

	// Truncate if too long
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}

	return name
}

// SanitizeMemoryLimit validates and normalizes memory limit strings
func SanitizeMemoryLimit(limit string) (string, error) {
	if limit == "" {
		return "", fmt.Errorf("memory limit cannot be empty")
	}

	// Valid formats: 100m, 512m, 1g, 2G, 256M
	memRegex := regexp.MustCompile(`^(\d+)(m|M|g|G|k|K)$`)

	if !memRegex.MatchString(limit) {
		return "", fmt.Errorf("invalid memory limit format: %s (use format like 512m, 1g)", limit)
	}

	// Normalize to lowercase unit
	matches := memRegex.FindStringSubmatch(limit)
	return matches[1] + strings.ToLower(matches[2]), nil
}

// SanitizeCPULimit validates and normalizes CPU limit strings
func SanitizeCPULimit(limit string) (string, error) {
	if limit == "" {
		return "", fmt.Errorf("CPU limit cannot be empty")
	}

	// Valid formats: 0.5, 1, 1.0, 2, 4
	cpuRegex := regexp.MustCompile(`^(\d+\.?\d*)$`)

	if !cpuRegex.MatchString(limit) {
		return "", fmt.Errorf("invalid CPU limit format: %s (use format like 0.5, 1, 2)", limit)
	}

	return limit, nil
}

// GenerateInstanceName generates a unique instance name
func GenerateInstanceName(serviceType, version string, existingNames []string) string {
	// Try: service-version
	baseName := fmt.Sprintf("%s-%s", serviceType, version)
	name := NormalizeInstanceName(baseName)

	if !containsName(existingNames, name) {
		return name
	}

	// If exists, try: service-version-1, service-version-2, etc.
	counter := 1
	for {
		candidate := fmt.Sprintf("%s-%d", name, counter)
		if !containsName(existingNames, candidate) {
			return candidate
		}
		counter++

		// Prevent infinite loop
		if counter > 100 {
			// Fall back to timestamp-based name
			return fmt.Sprintf("%s-%d", name, time.Now().Unix())
		}
	}
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// ParseMemoryToBytes converts memory string to bytes (for comparison)
func ParseMemoryToBytes(mem string) (int64, error) {
	if mem == "" {
		return 0, fmt.Errorf("empty memory value")
	}

	memRegex := regexp.MustCompile(`^(\d+)(m|M|g|G|k|K|b|B)?$`)
	matches := memRegex.FindStringSubmatch(mem)

	if matches == nil {
		return 0, fmt.Errorf("invalid memory format: %s", mem)
	}

	var value int64
	fmt.Sscanf(matches[1], "%d", &value)

	unit := strings.ToLower(matches[2])
	switch unit {
	case "k":
		return value * 1024, nil
	case "m", "":
		return value * 1024 * 1024, nil
	case "g":
		return value * 1024 * 1024 * 1024, nil
	case "b":
		return value, nil
	default:
		return value * 1024 * 1024, nil // Default to MB
	}
}
