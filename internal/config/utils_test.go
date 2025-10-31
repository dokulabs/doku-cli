package config

import (
	"testing"
)

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		domain  string
		wantErr bool
	}{
		{"doku.local", false},
		{"my-domain.dev", false},
		{"example.com", false},
		{"sub.domain.local", false},
		{"", true},                         // Empty
		{"-invalid.local", true},           // Starts with hyphen
		{"invalid-.local", true},           // Ends with hyphen
		{"inv@lid.local", true},            // Invalid character
		{"spaces not allowed.local", true}, // Spaces
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"postgres-14", false},
		{"redis", false},
		{"my-app", false},
		{"app123", false},
		{"a", false},
		{"", true},          // Empty
		{"Uppercase", true}, // Uppercase
		{"-invalid", true},  // Starts with hyphen
		{"invalid-", true},  // Ends with hyphen
		{"inv@lid", true},   // Invalid character
		{"traefik", true},   // Reserved name
		{"123-app", true},   // Starts with number but ends with valid
		{"app_name", true},  // Underscore not allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInstanceName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInstanceName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeInstanceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PostgreSQL 14", "postgresql-14"},
		{"My_App_Name", "my-app-name"},
		{"UPPERCASE", "uppercase"},
		{"app@#$name", "app-name"},
		{"--multiple--hyphens--", "multiple-hyphens"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeInstanceName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeInstanceName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeMemoryLimit(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"512m", "512m", false},
		{"512M", "512m", false},
		{"1g", "1g", false},
		{"1G", "1g", false},
		{"256k", "256k", false},
		{"100", "", true},     // Missing unit
		{"", "", true},        // Empty
		{"invalid", "", true}, // Invalid format
		{"1.5g", "", true},    // Decimal not supported
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := SanitizeMemoryLimit(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeMemoryLimit(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SanitizeMemoryLimit(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeCPULimit(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"1", "1", false},
		{"0.5", "0.5", false},
		{"2.0", "2.0", false},
		{"4", "4", false},
		{"", "", true},        // Empty
		{"invalid", "", true}, // Invalid format
		{"1.5.2", "", true},   // Multiple decimals
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := SanitizeCPULimit(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeCPULimit(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SanitizeCPULimit(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateInstanceName(t *testing.T) {
	tests := []struct {
		serviceType   string
		version       string
		existingNames []string
		want          string
	}{
		{"postgres", "14", []string{}, "postgres-14"},
		{"postgres", "14", []string{"postgres-14"}, "postgres-14-1"},
		{"postgres", "14", []string{"postgres-14", "postgres-14-1"}, "postgres-14-2"},
		{"redis", "7", []string{}, "redis-7"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GenerateInstanceName(tt.serviceType, tt.version, tt.existingNames)
			if got != tt.want {
				t.Errorf("GenerateInstanceName(%q, %q, %v) = %q, want %q",
					tt.serviceType, tt.version, tt.existingNames, got, tt.want)
			}
		})
	}
}

func TestParseMemoryToBytes(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"100m", 100 * 1024 * 1024, false},
		{"512M", 512 * 1024 * 1024, false},
		{"1g", 1 * 1024 * 1024 * 1024, false},
		{"2G", 2 * 1024 * 1024 * 1024, false},
		{"256k", 256 * 1024, false},
		{"1024", 1024 * 1024 * 1024, false}, // Defaults to MB
		{"", 0, true},                       // Empty
		{"invalid", 0, true},                // Invalid format
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseMemoryToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMemoryToBytes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseMemoryToBytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
