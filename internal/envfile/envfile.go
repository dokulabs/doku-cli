package envfile

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ServiceEnvDir is the subdirectory for service env files
	ServiceEnvDir = "services"
	// ProjectEnvDir is the subdirectory for project env files
	ProjectEnvDir = "projects"
)

// Manager handles environment file operations
type Manager struct {
	dokuDir string
}

// NewManager creates a new environment file manager
func NewManager(dokuDir string) *Manager {
	return &Manager{
		dokuDir: dokuDir,
	}
}

// GetServiceEnvPath returns the path to a service's env file
// For single-container services: ~/.doku/services/<instance>.env
// For multi-container services with container name: ~/.doku/services/<instance>-<container>.env
func (m *Manager) GetServiceEnvPath(instanceName string, containerName string) string {
	if containerName != "" {
		return filepath.Join(m.dokuDir, ServiceEnvDir, fmt.Sprintf("%s-%s.env", instanceName, containerName))
	}
	return filepath.Join(m.dokuDir, ServiceEnvDir, fmt.Sprintf("%s.env", instanceName))
}

// GetProjectEnvPath returns the path to a project's env file
func (m *Manager) GetProjectEnvPath(projectName string) string {
	return filepath.Join(m.dokuDir, ProjectEnvDir, fmt.Sprintf("%s.env", projectName))
}

// GetInitContainerEnvPath returns the path to an init container's env file
func (m *Manager) GetInitContainerEnvPath(instanceName string, initContainerName string) string {
	return filepath.Join(m.dokuDir, ServiceEnvDir, fmt.Sprintf("%s-init-%s.env", instanceName, initContainerName))
}

// Load reads environment variables from an env file
func (m *Manager) Load(envPath string) (map[string]string, error) {
	return LoadEnvFile(envPath)
}

// Save writes environment variables to an env file
func (m *Manager) Save(envPath string, env map[string]string) error {
	return SaveEnvFile(envPath, env)
}

// Exists checks if an env file exists
func (m *Manager) Exists(envPath string) bool {
	_, err := os.Stat(envPath)
	return err == nil
}

// Delete removes an env file
func (m *Manager) Delete(envPath string) error {
	if !m.Exists(envPath) {
		return nil
	}
	return os.Remove(envPath)
}

// EnsureDir ensures the env file directory exists
func (m *Manager) EnsureDir(envPath string) error {
	dir := filepath.Dir(envPath)
	return os.MkdirAll(dir, 0755)
}

// LoadEnvFile loads environment variables from an env file
func LoadEnvFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return env, nil
}

// SaveEnvFile writes environment variables to an env file
func SaveEnvFile(filePath string, env map[string]string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create or truncate file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}
	defer file.Close()

	// Sort keys for consistent output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write header comment
	fmt.Fprintf(file, "# Environment variables for this service\n")
	fmt.Fprintf(file, "# Managed by doku - edit with 'doku env edit <service>'\n")
	fmt.Fprintf(file, "# Changes are applied on restart\n\n")

	// Write each variable
	for _, key := range keys {
		value := env[key]
		// Quote values that contain special characters
		if needsQuoting(value) {
			fmt.Fprintf(file, "%s=\"%s\"\n", key, escapeValue(value))
		} else {
			fmt.Fprintf(file, "%s=%s\n", key, value)
		}
	}

	return nil
}

// UpdateEnvFile updates specific keys in an env file, preserving comments and order
func UpdateEnvFile(filePath string, updates map[string]string) error {
	// Load existing env
	existing, err := LoadEnvFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if existing == nil {
		existing = make(map[string]string)
	}

	// Apply updates
	for k, v := range updates {
		existing[k] = v
	}

	// Save back
	return SaveEnvFile(filePath, existing)
}

// DeleteFromEnvFile removes keys from an env file
func DeleteFromEnvFile(filePath string, keys []string) error {
	// Load existing env
	existing, err := LoadEnvFile(filePath)
	if err != nil {
		return err
	}

	// Remove keys
	for _, k := range keys {
		delete(existing, k)
	}

	// Save back
	return SaveEnvFile(filePath, existing)
}

// MergeEnv merges multiple environment maps, later maps override earlier ones
func MergeEnv(envMaps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, env := range envMaps {
		for k, v := range env {
			result[k] = v
		}
	}
	return result
}

// EnvMapToSlice converts environment map to Docker-compatible slice
func EnvMapToSlice(env map[string]string) []string {
	slice := make([]string, 0, len(env))
	for k, v := range env {
		slice = append(slice, fmt.Sprintf("%s=%s", k, v))
	}
	return slice
}

// EnvSliceToMap converts Docker-compatible slice to environment map
func EnvSliceToMap(envSlice []string) map[string]string {
	env := make(map[string]string)
	for _, e := range envSlice {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// OpenInEditor opens the env file in the user's preferred editor
func OpenInEditor(filePath string) error {
	editor := getEditor()
	if editor == "" {
		return fmt.Errorf("no editor found. Set $EDITOR or $VISUAL environment variable")
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getEditor returns the user's preferred editor
func getEditor() string {
	// Check environment variables
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	// Try common editors
	editors := []string{"vim", "vi", "nano", "code", "subl"}
	for _, editor := range editors {
		if path, err := exec.LookPath(editor); err == nil {
			return path
		}
	}

	return ""
}

// needsQuoting returns true if the value needs to be quoted
func needsQuoting(value string) bool {
	// Quote if contains spaces, quotes, special characters, or is empty
	if value == "" {
		return true
	}
	specialChars := " \t\n\r\"'`$\\#"
	for _, c := range specialChars {
		if strings.ContainsRune(value, c) {
			return true
		}
	}
	return false
}

// escapeValue escapes special characters in a value for env file
func escapeValue(value string) string {
	// Escape backslashes first, then quotes
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return value
}

// ListServiceEnvFiles returns all env files for services
func (m *Manager) ListServiceEnvFiles() ([]string, error) {
	pattern := filepath.Join(m.dokuDir, ServiceEnvDir, "*.env")
	return filepath.Glob(pattern)
}

// ListProjectEnvFiles returns all env files for projects
func (m *Manager) ListProjectEnvFiles() ([]string, error) {
	pattern := filepath.Join(m.dokuDir, ProjectEnvDir, "*.env")
	return filepath.Glob(pattern)
}

// GetInstanceNameFromPath extracts the instance name from an env file path
func GetInstanceNameFromPath(envPath string) string {
	base := filepath.Base(envPath)
	return strings.TrimSuffix(base, ".env")
}
