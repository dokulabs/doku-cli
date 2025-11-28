package service

import (
	"testing"
)

// TestContainsAny tests the containsAny helper function
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{
			name:     "contains user",
			s:        "POSTGRES_USER",
			substrs:  []string{"user", "username", "password", "database", "db"},
			expected: true,
		},
		{
			name:     "contains password",
			s:        "MYSQL_ROOT_PASSWORD",
			substrs:  []string{"user", "username", "password", "database", "db"},
			expected: true,
		},
		{
			name:     "contains database",
			s:        "POSTGRES_DATABASE",
			substrs:  []string{"user", "username", "password", "database", "db"},
			expected: true,
		},
		{
			name:     "contains db",
			s:        "POSTGRES_DB",
			substrs:  []string{"user", "username", "password", "database", "db"},
			expected: true,
		},
		{
			name:     "does not contain any",
			s:        "SOME_OTHER_VAR",
			substrs:  []string{"user", "username", "password", "database", "db"},
			expected: false,
		},
		{
			name:     "case insensitive match",
			s:        "postgres_USER",
			substrs:  []string{"user"},
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substrs:  []string{"user"},
			expected: false,
		},
		{
			name:     "empty substrs",
			s:        "POSTGRES_USER",
			substrs:  []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrs)
			if result != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, expected %v", tt.s, tt.substrs, result, tt.expected)
			}
		})
	}
}

// TestNewManager tests creating a new service manager
func TestNewManager(t *testing.T) {
	// NewManager with nil arguments should still create a manager
	manager := NewManager(nil, nil)
	if manager == nil {
		t.Error("NewManager should not return nil")
	}
	if manager.dockerClient != nil {
		t.Error("dockerClient should be nil when passed nil")
	}
	if manager.configMgr != nil {
		t.Error("configMgr should be nil when passed nil")
	}
}
