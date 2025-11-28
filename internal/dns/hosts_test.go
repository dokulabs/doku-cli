package dns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewManager tests the NewManager function
func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.hostsFile == "" {
		t.Error("hostsFile should not be empty")
	}
}

// TestGetHostsFilePath tests the getHostsFilePath function
func TestGetHostsFilePath(t *testing.T) {
	path := getHostsFilePath()
	if path == "" {
		t.Error("getHostsFilePath returned empty string")
	}
	// Path should be either /etc/hosts or Windows path
	if !strings.Contains(path, "hosts") {
		t.Errorf("getHostsFilePath returned unexpected path: %s", path)
	}
}

// TestManagerWithTempFile creates a Manager with a temp file for testing
func createTestManager(t *testing.T, initialContent string) (*Manager, string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "doku-dns-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create temp hosts file
	tmpHostsFile := filepath.Join(tmpDir, "hosts")
	if err := os.WriteFile(tmpHostsFile, []byte(initialContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create temp hosts file: %v", err)
	}

	manager := &Manager{hostsFile: tmpHostsFile}
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return manager, tmpHostsFile, cleanup
}

// TestHasDokuEntries tests the HasDokuEntries function
func TestHasDokuEntries(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no doku entries",
			content:  "127.0.0.1 localhost\n::1 localhost\n",
			expected: false,
		},
		{
			name:     "has doku marker",
			content:  "127.0.0.1 localhost\n127.0.0.1 doku.local # doku-managed - do not edit\n",
			expected: true,
		},
		{
			name:     "has doku start marker",
			content:  "127.0.0.1 localhost\n# doku-managed-start\n127.0.0.1 doku.local\n# doku-managed-end\n",
			expected: true,
		},
		{
			name:     "empty file",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, _, cleanup := createTestManager(t, tt.content)
			defer cleanup()

			result, err := manager.HasDokuEntries()
			if err != nil {
				t.Fatalf("HasDokuEntries returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("HasDokuEntries() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestGenerateHostsEntries tests the generateHostsEntries function
func TestGenerateHostsEntries(t *testing.T) {
	manager := &Manager{}
	entries := manager.generateHostsEntries("doku.local")

	if !strings.Contains(entries, DokuStart) {
		t.Error("Generated entries should contain DokuStart marker")
	}
	if !strings.Contains(entries, DokuEnd) {
		t.Error("Generated entries should contain DokuEnd marker")
	}
	if !strings.Contains(entries, "127.0.0.1 doku.local") {
		t.Error("Generated entries should contain domain entry")
	}
	if !strings.Contains(entries, DokuMarker) {
		t.Error("Generated entries should contain DokuMarker")
	}
}

// TestAddDokuDomain tests adding a Doku domain
func TestAddDokuDomain(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n::1 localhost\n"
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.AddDokuDomain("doku.local")
	if err != nil {
		t.Fatalf("AddDokuDomain failed: %v", err)
	}

	// Read the file and verify
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		t.Fatalf("Failed to read hosts file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "doku.local") {
		t.Error("Hosts file should contain doku.local")
	}
	if !strings.Contains(contentStr, DokuStart) {
		t.Error("Hosts file should contain DokuStart marker")
	}
	if !strings.Contains(contentStr, DokuEnd) {
		t.Error("Hosts file should contain DokuEnd marker")
	}
}

// TestAddDokuDomainIdempotent tests that adding same domain twice is idempotent
func TestAddDokuDomainIdempotent(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n"
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	// Add domain first time
	err := manager.AddDokuDomain("doku.local")
	if err != nil {
		t.Fatalf("First AddDokuDomain failed: %v", err)
	}

	// Read content after first add
	content1, _ := os.ReadFile(hostsFile)

	// Add domain second time
	err = manager.AddDokuDomain("doku.local")
	if err != nil {
		t.Fatalf("Second AddDokuDomain failed: %v", err)
	}

	// Read content after second add
	content2, _ := os.ReadFile(hostsFile)

	// Content should be the same (idempotent)
	if string(content1) != string(content2) {
		t.Error("AddDokuDomain should be idempotent")
	}
}

// TestRemoveDokuEntries tests removing Doku entries
func TestRemoveDokuEntries(t *testing.T) {
	initialContent := `127.0.0.1 localhost
::1 localhost
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
127.0.0.1 rabbitmq.doku.local # doku-managed - do not edit
# doku-managed-end
`
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.RemoveDokuEntries()
	if err != nil {
		t.Fatalf("RemoveDokuEntries failed: %v", err)
	}

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		t.Fatalf("Failed to read hosts file: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, DokuStart) {
		t.Error("Hosts file should not contain DokuStart marker after removal")
	}
	if strings.Contains(contentStr, DokuEnd) {
		t.Error("Hosts file should not contain DokuEnd marker after removal")
	}
	if strings.Contains(contentStr, "doku.local") {
		t.Error("Hosts file should not contain doku.local after removal")
	}
	if !strings.Contains(contentStr, "127.0.0.1 localhost") {
		t.Error("Hosts file should still contain localhost entry")
	}
}

// TestGetDokuDomain tests retrieving the Doku domain
func TestGetDokuDomain(t *testing.T) {
	initialContent := `127.0.0.1 localhost
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
127.0.0.1 rabbitmq.doku.local # doku-managed - do not edit
# doku-managed-end
`
	manager, _, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	domain, err := manager.GetDokuDomain()
	if err != nil {
		t.Fatalf("GetDokuDomain failed: %v", err)
	}

	if domain != "doku.local" {
		t.Errorf("GetDokuDomain() = %s, expected doku.local", domain)
	}
}

// TestGetDokuDomainNotFound tests GetDokuDomain when no domain exists
func TestGetDokuDomainNotFound(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n"
	manager, _, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	_, err := manager.GetDokuDomain()
	if err == nil {
		t.Error("GetDokuDomain should return error when no domain exists")
	}
}

// TestAddServiceDomain tests adding a service domain
func TestAddServiceDomain(t *testing.T) {
	initialContent := `127.0.0.1 localhost
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
# doku-managed-end
`
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.AddServiceDomain("rabbitmq", "doku.local")
	if err != nil {
		t.Fatalf("AddServiceDomain failed: %v", err)
	}

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		t.Fatalf("Failed to read hosts file: %v", err)
	}

	if !strings.Contains(string(content), "rabbitmq.doku.local") {
		t.Error("Hosts file should contain rabbitmq.doku.local")
	}
}

// TestAddServiceDomainIdempotent tests that adding same service twice is idempotent
func TestAddServiceDomainIdempotent(t *testing.T) {
	initialContent := `127.0.0.1 localhost
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
# doku-managed-end
`
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	// Add service first time
	err := manager.AddServiceDomain("redis", "doku.local")
	if err != nil {
		t.Fatalf("First AddServiceDomain failed: %v", err)
	}

	content1, _ := os.ReadFile(hostsFile)
	count1 := strings.Count(string(content1), "redis.doku.local")

	// Add service second time
	err = manager.AddServiceDomain("redis", "doku.local")
	if err != nil {
		t.Fatalf("Second AddServiceDomain failed: %v", err)
	}

	content2, _ := os.ReadFile(hostsFile)
	count2 := strings.Count(string(content2), "redis.doku.local")

	if count1 != count2 {
		t.Errorf("AddServiceDomain should be idempotent, got %d entries first, %d entries second", count1, count2)
	}
}

// TestListDokuEntries tests listing Doku entries
func TestListDokuEntries(t *testing.T) {
	initialContent := `127.0.0.1 localhost
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
127.0.0.1 rabbitmq.doku.local # doku-managed - do not edit
127.0.0.1 redis.doku.local # doku-managed - do not edit
# doku-managed-end
`
	manager, _, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	entries, err := manager.ListDokuEntries()
	if err != nil {
		t.Fatalf("ListDokuEntries failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

// TestVerifyDNSResolution tests DNS verification
func TestVerifyDNSResolution(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n127.0.0.1 doku.local\n"
	manager, _, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	// Should succeed for existing domain
	err := manager.VerifyDNSResolution("doku.local")
	if err != nil {
		t.Errorf("VerifyDNSResolution should succeed for existing domain: %v", err)
	}

	// Should fail for non-existing domain
	err = manager.VerifyDNSResolution("nonexistent.local")
	if err == nil {
		t.Error("VerifyDNSResolution should fail for non-existing domain")
	}
}

// TestValidateHostsFile tests hosts file validation
func TestValidateHostsFile(t *testing.T) {
	manager, _, cleanup := createTestManager(t, "127.0.0.1 localhost\n")
	defer cleanup()

	err := manager.ValidateHostsFile()
	if err != nil {
		t.Errorf("ValidateHostsFile should succeed for valid file: %v", err)
	}
}

// TestValidateHostsFileNotExists tests validation of non-existent file
func TestValidateHostsFileNotExists(t *testing.T) {
	manager := &Manager{hostsFile: "/nonexistent/path/hosts"}

	err := manager.ValidateHostsFile()
	if err == nil {
		t.Error("ValidateHostsFile should fail for non-existent file")
	}
}

// TestGetHostsFilePath_Method tests the GetHostsFilePath method
func TestGetHostsFilePath_Method(t *testing.T) {
	manager, expectedPath, cleanup := createTestManager(t, "")
	defer cleanup()

	path := manager.GetHostsFilePath()
	if path != expectedPath {
		t.Errorf("GetHostsFilePath() = %s, expected %s", path, expectedPath)
	}
}

// TestBackupAndRestore tests backup and restore functionality
func TestBackupAndRestore(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n"
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	// Create backup
	backupPath, err := manager.BackupHostsFile()
	if err != nil {
		t.Fatalf("BackupHostsFile failed: %v", err)
	}
	defer os.Remove(backupPath)

	// Modify the hosts file
	modifiedContent := "127.0.0.1 localhost\n127.0.0.1 modified.local\n"
	if err := os.WriteFile(hostsFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify hosts file: %v", err)
	}

	// Verify modification
	content, _ := os.ReadFile(hostsFile)
	if !strings.Contains(string(content), "modified.local") {
		t.Error("Hosts file should be modified")
	}

	// Restore from backup
	err = manager.RestoreHostsFile(backupPath)
	if err != nil {
		t.Fatalf("RestoreHostsFile failed: %v", err)
	}

	// Verify restoration
	content, _ = os.ReadFile(hostsFile)
	if strings.Contains(string(content), "modified.local") {
		t.Error("Hosts file should be restored to original")
	}
	if !strings.Contains(string(content), "localhost") {
		t.Error("Hosts file should contain original localhost entry")
	}
}

// TestAddSingleEntry tests adding a single entry
func TestAddSingleEntry(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n"
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.AddSingleEntry("192.168.1.100", "myservice.local")
	if err != nil {
		t.Fatalf("AddSingleEntry failed: %v", err)
	}

	content, _ := os.ReadFile(hostsFile)
	if !strings.Contains(string(content), "192.168.1.100 myservice.local") {
		t.Error("Hosts file should contain the new entry")
	}
}

// TestAddSingleEntryIdempotent tests that adding same entry twice is idempotent
func TestAddSingleEntryIdempotent(t *testing.T) {
	initialContent := "127.0.0.1 localhost\n"
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	// Add entry first time
	err := manager.AddSingleEntry("192.168.1.100", "myservice.local")
	if err != nil {
		t.Fatalf("First AddSingleEntry failed: %v", err)
	}

	content1, _ := os.ReadFile(hostsFile)

	// Add entry second time
	err = manager.AddSingleEntry("192.168.1.100", "myservice.local")
	if err != nil {
		t.Fatalf("Second AddSingleEntry failed: %v", err)
	}

	content2, _ := os.ReadFile(hostsFile)

	count1 := strings.Count(string(content1), "myservice.local")
	count2 := strings.Count(string(content2), "myservice.local")

	if count1 != count2 {
		t.Error("AddSingleEntry should be idempotent")
	}
}

// TestRemoveSingleEntry tests removing a single entry
func TestRemoveSingleEntry(t *testing.T) {
	initialContent := `127.0.0.1 localhost
192.168.1.100 myservice.local # doku-managed - do not edit
`
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.RemoveSingleEntry("myservice.local")
	if err != nil {
		t.Fatalf("RemoveSingleEntry failed: %v", err)
	}

	content, _ := os.ReadFile(hostsFile)
	if strings.Contains(string(content), "myservice.local") {
		t.Error("Hosts file should not contain myservice.local after removal")
	}
	if !strings.Contains(string(content), "localhost") {
		t.Error("Hosts file should still contain localhost")
	}
}

// TestExecuteCommandArgs tests the executeCommandArgs function
func TestExecuteCommandArgs(t *testing.T) {
	// Test with empty command name
	err := executeCommandArgs("")
	if err == nil {
		t.Error("executeCommandArgs should fail with empty command name")
	}

	// Test with valid command (echo is available on all platforms)
	err = executeCommandArgs("echo", "test")
	if err != nil {
		t.Errorf("executeCommandArgs should succeed with valid command: %v", err)
	}
}

// TestExecuteCommand tests the deprecated executeCommand function
func TestExecuteCommand(t *testing.T) {
	// Test with empty command
	err := executeCommand("")
	if err == nil {
		t.Error("executeCommand should fail with empty command")
	}

	// Test with valid command
	err = executeCommand("echo test")
	if err != nil {
		t.Errorf("executeCommand should succeed with valid command: %v", err)
	}
}

// TestUpdateDokuDomain tests updating the Doku domain
func TestUpdateDokuDomain(t *testing.T) {
	initialContent := `127.0.0.1 localhost
# doku-managed-start
127.0.0.1 old.local # doku-managed - do not edit
# doku-managed-end
`
	manager, hostsFile, cleanup := createTestManager(t, initialContent)
	defer cleanup()

	err := manager.UpdateDokuDomain("new.local")
	if err != nil {
		t.Fatalf("UpdateDokuDomain failed: %v", err)
	}

	content, _ := os.ReadFile(hostsFile)
	contentStr := string(content)

	if strings.Contains(contentStr, "old.local") {
		t.Error("Hosts file should not contain old domain")
	}
	if !strings.Contains(contentStr, "new.local") {
		t.Error("Hosts file should contain new domain")
	}
}

// TestConstants verifies the constants are properly defined
func TestConstants(t *testing.T) {
	if DokuMarker == "" {
		t.Error("DokuMarker should not be empty")
	}
	if DokuStart == "" {
		t.Error("DokuStart should not be empty")
	}
	if DokuEnd == "" {
		t.Error("DokuEnd should not be empty")
	}
}
