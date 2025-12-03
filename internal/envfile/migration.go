package envfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

// MigrationResult contains the result of a migration operation
type MigrationResult struct {
	ServicesMigrated int
	ProjectsMigrated int
	Errors           []error
}

// MigrateServiceEnv migrates environment variables from config to env file
// Returns true if migration was performed, false if env file already exists
func (m *Manager) MigrateServiceEnv(instanceName string, env map[string]string, containerName string) (bool, error) {
	envPath := m.GetServiceEnvPath(instanceName, containerName)

	// Check if env file already exists
	if m.Exists(envPath) {
		return false, nil
	}

	// Only migrate if there are env vars to migrate
	if len(env) == 0 {
		return false, nil
	}

	// Ensure directory exists
	if err := m.EnsureDir(envPath); err != nil {
		return false, fmt.Errorf("failed to create env directory: %w", err)
	}

	// Save env file
	if err := m.Save(envPath, env); err != nil {
		return false, fmt.Errorf("failed to save env file: %w", err)
	}

	return true, nil
}

// MigrateProjectEnv migrates environment variables from project config to env file
// Also handles migration from .env.doku in project directory
func (m *Manager) MigrateProjectEnv(projectName string, env map[string]string, projectPath string) (bool, error) {
	envPath := m.GetProjectEnvPath(projectName)

	// Check if env file already exists
	if m.Exists(envPath) {
		return false, nil
	}

	// Try to load from .env.doku in project directory first
	if projectPath != "" {
		envDokuPath := filepath.Join(projectPath, ".env.doku")
		if fileExists(envDokuPath) {
			projectEnv, err := LoadEnvFile(envDokuPath)
			if err == nil && len(projectEnv) > 0 {
				// Merge with config env (config takes precedence as it may have updates)
				env = MergeEnv(projectEnv, env)
			}
		}
	}

	// Only migrate if there are env vars to migrate
	if len(env) == 0 {
		return false, nil
	}

	// Ensure directory exists
	if err := m.EnsureDir(envPath); err != nil {
		return false, fmt.Errorf("failed to create env directory: %w", err)
	}

	// Save env file
	if err := m.Save(envPath, env); err != nil {
		return false, fmt.Errorf("failed to save env file: %w", err)
	}

	return true, nil
}

// MigrateAllFromConfig migrates all services and projects from config
// This is called during operations that need env vars to ensure migration happens
func (m *Manager) MigrateAllFromConfig(instances map[string]InstanceEnvData, projects map[string]ProjectEnvData) *MigrationResult {
	result := &MigrationResult{}

	// Migrate services
	for name, data := range instances {
		if data.IsMultiContainer {
			// Migrate per-container env files
			for _, container := range data.Containers {
				migrated, err := m.MigrateServiceEnv(name, container.Environment, container.Name)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("service %s container %s: %w", name, container.Name, err))
				} else if migrated {
					result.ServicesMigrated++
				}
			}
			// Also migrate service-level env
			if len(data.Environment) > 0 {
				migrated, err := m.MigrateServiceEnv(name, data.Environment, "")
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("service %s: %w", name, err))
				} else if migrated {
					result.ServicesMigrated++
				}
			}
		} else {
			migrated, err := m.MigrateServiceEnv(name, data.Environment, "")
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("service %s: %w", name, err))
			} else if migrated {
				result.ServicesMigrated++
			}
		}
	}

	// Migrate projects
	for name, data := range projects {
		migrated, err := m.MigrateProjectEnv(name, data.Environment, data.Path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("project %s: %w", name, err))
		} else if migrated {
			result.ProjectsMigrated++
		}
	}

	return result
}

// InstanceEnvData contains data needed for instance migration
type InstanceEnvData struct {
	Environment      map[string]string
	IsMultiContainer bool
	Containers       []ContainerEnvData
}

// ContainerEnvData contains data needed for container migration
type ContainerEnvData struct {
	Name        string
	Environment map[string]string
}

// ProjectEnvData contains data needed for project migration
type ProjectEnvData struct {
	Environment map[string]string
	Path        string
}

// PrintMigrationResult prints the migration result to stdout
func PrintMigrationResult(result *MigrationResult) {
	if result.ServicesMigrated > 0 || result.ProjectsMigrated > 0 {
		color.Green("✓ Migrated environment variables to .env files:")
		if result.ServicesMigrated > 0 {
			fmt.Printf("  - %d service(s)\n", result.ServicesMigrated)
		}
		if result.ProjectsMigrated > 0 {
			fmt.Printf("  - %d project(s)\n", result.ProjectsMigrated)
		}
	}

	if len(result.Errors) > 0 {
		color.Yellow("⚠️  Some migrations failed:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}
}

// CleanupEnvFiles removes env files for a service or project
func (m *Manager) CleanupEnvFiles(instanceName string, isMultiContainer bool, containerNames []string) error {
	var errs []error

	// Remove main env file
	mainEnvPath := m.GetServiceEnvPath(instanceName, "")
	if err := m.Delete(mainEnvPath); err != nil {
		errs = append(errs, err)
	}

	// Remove container-specific env files for multi-container services
	if isMultiContainer {
		for _, containerName := range containerNames {
			containerEnvPath := m.GetServiceEnvPath(instanceName, containerName)
			if err := m.Delete(containerEnvPath); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to cleanup some env files: %v", errs)
	}
	return nil
}

// CleanupProjectEnvFile removes the env file for a project
func (m *Manager) CleanupProjectEnvFile(projectName string) error {
	envPath := m.GetProjectEnvPath(projectName)
	return m.Delete(envPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
