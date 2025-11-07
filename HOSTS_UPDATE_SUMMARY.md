# Hosts File Management with Sudo - Implementation Summary

## Overview

Updated doku-cli to properly manage `/etc/hosts` entries with sudo permissions, including duplicate detection and user-friendly password prompts.

## Changes Made

### 1. Updated `internal/dns/hosts.go`

#### Added Import
```go
import (
	"os/exec"  // Added for executing sudo commands
)
```

#### Updated `executeCommand()` Function
**Before:** Placeholder that returned an error suggesting manual intervention

**After:** Actually executes shell commands with proper stdin/stdout/stderr handling
```go
func executeCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	command := exec.Command(parts[0], parts[1:]...)

	// Connect to stdin/stdout/stderr for sudo password prompt
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}
```

#### Updated `copyWithSudo()` Function
Added user-friendly messages and proper error handling:
```go
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

	// Inform user about sudo requirement
	fmt.Println()
	fmt.Println("⚠️  Updating /etc/hosts requires administrator privileges")
	fmt.Println("📝 Please enter your password when prompted...")
	fmt.Println()

	cmd := fmt.Sprintf("sudo cp %s %s", src, dest)

	if err := executeCommand(cmd); err != nil {
		return fmt.Errorf("failed to update hosts file with sudo: %w", err)
	}

	fmt.Println("✓ Hosts file updated successfully")
	return nil
}
```

#### Updated `AddDokuDomain()` Function
Enhanced duplicate detection with user-friendly messages:
```go
func (m *Manager) AddDokuDomain(domain string) error {
	exists, err := m.HasDokuEntries()
	if err != nil {
		return err
	}

	if exists {
		// Check if it's the same domain
		existingDomain, err := m.GetDokuDomain()
		if err == nil && existingDomain == domain {
			fmt.Printf("✓ Hosts file entries for %s already exist\n", domain)
			return nil  // Skip update - no changes needed
		}
		// Update existing entries with new domain
		return m.UpdateDokuDomain(domain)
	}

	// ... rest of the function
}
```

#### Updated `AddSingleEntry()` Function
Improved duplicate detection to check exact hostname matches:
```go
func (m *Manager) AddSingleEntry(ip, hostname string) error {
	content, err := os.ReadFile(m.hostsFile)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Check if entry already exists
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// Check if hostname matches in the line
		for i, field := range fields {
			if i > 0 && field == hostname {
				fmt.Printf("✓ Entry for %s already exists in hosts file\n", hostname)
				return nil
			}
		}
	}

	// Add new entry
	entry := fmt.Sprintf("\n%s %s %s\n", ip, hostname, DokuMarker)
	updatedContent := string(content) + entry

	return m.writeHostsFile(updatedContent)
}
```

## Features

### 1. Sudo Password Prompt
- Automatically prompts for sudo password when needed
- Clear user messaging about privilege requirements
- Proper stdin/stdout/stderr handling for interactive prompts

### 2. Duplicate Detection
- Checks if entries already exist before adding
- Compares existing domain with requested domain
- Shows friendly message when entries are already present
- Skips unnecessary updates

### 3. User-Friendly Messages
- **Sudo required:** "⚠️  Updating /etc/hosts requires administrator privileges"
- **Password prompt:** "📝 Please enter your password when prompted..."
- **Success:** "✓ Hosts file updated successfully"
- **Duplicate:** "✓ Hosts file entries for doku.local already exist"
- **Entry exists:** "✓ Entry for hostname already exists in hosts file"

### 4. Safe Operations
- Tries to write without sudo first
- Only uses sudo when necessary
- Creates temporary file before copying
- Preserves existing entries with doku-managed markers

## Usage

### During Init
```bash
$ doku init

# When DNS setup is selected:
⚠️  Updating /etc/hosts requires administrator privileges
📝 Please enter your password when prompted...

Password: [user enters password]
✓ Hosts file updated successfully
```

### When Entries Already Exist
```bash
$ doku init

✓ Hosts file entries for doku.local already exist
```

## Hosts File Format

Doku uses special markers to manage its entries:

```
# doku-managed-start
127.0.0.1 doku.local # doku-managed - do not edit
127.0.0.1 *.doku.local # doku-managed - do not edit
# doku-managed-end
```

These markers allow doku to:
- Identify its own entries
- Update entries when domain changes
- Remove entries during uninstall
- Detect duplicates

## Security Considerations

1. **Minimal Sudo Usage:** Only uses sudo when necessary
2. **Temporary Files:** Uses temp files to prepare content before copying
3. **Marker-Based Management:** Only modifies entries with doku-managed markers
4. **User Confirmation:** User must explicitly enter password
5. **No Automated Privileges:** Never stores or caches sudo credentials

## Testing

Due to the TTY requirements for sudo password prompts, testing in automated environments is limited. The implementation is designed for interactive use where:

1. User runs `doku init`
2. Selects "Automatic (/etc/hosts modification)"
3. Enters sudo password when prompted
4. Hosts file is updated successfully

### Manual Testing Steps

```bash
# 1. Build the project
make build

# 2. Run init
./bin/doku init

# 3. Select "Automatic" DNS setup

# 4. Enter password when prompted

# 5. Verify entries were added
grep "doku-managed" /etc/hosts

# 6. Run init again to test duplicate detection
./bin/doku init
# Should show: "✓ Hosts file entries for doku.local already exist"
```

## Error Handling

- **Permission denied:** Prompts for sudo with clear message
- **Sudo fails:** Returns error with details
- **File not found:** Returns descriptive error
- **Duplicate entries:** Skips with success message

## Backward Compatibility

- Existing hosts file entries are preserved
- Entries without doku-managed markers are untouched
- Can detect and update old-style entries
- Safe to run multiple times

## Platform Support

- ✅ **macOS:** Full support with sudo prompts
- ✅ **Linux:** Full support with sudo prompts
- ✅ **Windows:** Attempts direct write to C:\Windows\System32\drivers\etc\hosts

## Future Enhancements

Potential improvements:
- Support for alternative sudo mechanisms (pkexec, osascript)
- DNS resolver configuration on macOS (/etc/resolver/)
- Systemd-resolved integration on Linux
- Backup/restore functionality
- Interactive entry management commands
