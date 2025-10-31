package cmd

import "strings"

// isSensitiveKey checks if a key contains sensitive information
func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"PASSWORD", "PASSWD", "SECRET", "TOKEN", "KEY", "API_KEY",
		"PRIVATE", "CREDENTIAL", "AUTH", "CERT",
	}

	upperKey := strings.ToUpper(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(upperKey, sensitive) {
			return true
		}
	}
	return false
}

// maskValue masks a sensitive value for display
func maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}
