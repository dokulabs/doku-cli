package backup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RestoreOptions holds options for restore operation
type RestoreOptions struct {
	BackupPath     string
	InstanceName   string // Target instance name (can differ from backup)
	RestoreVolumes bool   // Restore Docker volumes
	RestoreEnv     bool   // Restore env files
	Overwrite      bool   // Overwrite existing files
}

// RestoreResult contains information about the restore operation
type RestoreResult struct {
	InstanceName    string
	RestoredEnvFiles []string
	RestoredVolumes  []string
	Warnings         []string
}

// Restore restores a backup to the system
func (m *Manager) Restore(opts RestoreOptions) (*RestoreResult, error) {
	// Open backup file
	file, err := os.Open(opts.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var tarReader *tar.Reader

	// Check if gzipped
	if strings.HasSuffix(opts.BackupPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	result := &RestoreResult{
		InstanceName:    opts.InstanceName,
		RestoredEnvFiles: []string{},
		RestoredVolumes:  []string{},
		Warnings:         []string{},
	}

	// Read metadata first to get original instance name if not specified
	metadata, err := m.readMetadataFromTar(opts.BackupPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Could not read metadata: %v", err))
	}

	// Use original instance name if not specified
	if opts.InstanceName == "" && metadata != nil {
		if name, ok := metadata["instance_name"]; ok {
			opts.InstanceName = name
			result.InstanceName = name
		}
	}

	if opts.InstanceName == "" {
		return nil, fmt.Errorf("instance name not specified and not found in backup metadata")
	}

	// Re-open file to read contents
	file.Seek(0, 0)
	if strings.HasSuffix(opts.BackupPath, ".gz") {
		gzReader, _ := gzip.NewReader(file)
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	// Process tar contents
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Handle env files
		if opts.RestoreEnv && strings.HasPrefix(header.Name, "env/") {
			envFileName := filepath.Base(header.Name)

			// Replace original instance name with target instance name if different
			if metadata != nil {
				if origName, ok := metadata["instance_name"]; ok && origName != opts.InstanceName {
					envFileName = strings.Replace(envFileName, origName, opts.InstanceName, 1)
				}
			}

			envPath := filepath.Join(m.configMgr.GetDokuDir(), "services", envFileName)

			// Check if file exists
			if !opts.Overwrite {
				if _, err := os.Stat(envPath); err == nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Skipped %s (already exists)", envFileName))
					continue
				}
			}

			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create directory for %s: %v", envFileName, err))
				continue
			}

			// Write file
			outFile, err := os.Create(envPath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create %s: %v", envFileName, err))
				continue
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to write %s: %v", envFileName, err))
				continue
			}
			outFile.Close()

			result.RestoredEnvFiles = append(result.RestoredEnvFiles, envFileName)
		}

		// Handle volume info (for now just note them)
		if opts.RestoreVolumes && strings.HasPrefix(header.Name, "volumes/") {
			volumeName := strings.TrimSuffix(filepath.Base(header.Name), ".info")
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Volume %s needs to be restored manually or recreated on next install", volumeName))
			result.RestoredVolumes = append(result.RestoredVolumes, volumeName)
		}
	}

	return result, nil
}

// readMetadataFromTar reads the metadata from a backup file
func (m *Manager) readMetadataFromTar(backupPath string) (map[string]string, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tarReader *tar.Reader

	if strings.HasSuffix(backupPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Name == "metadata.txt" {
			return m.parseMetadata(tarReader)
		}
	}

	return nil, fmt.Errorf("metadata not found in backup")
}

// parseMetadata parses the metadata file content
func (m *Manager) parseMetadata(reader io.Reader) (map[string]string, error) {
	metadata := make(map[string]string)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			metadata[key] = value
		}
	}

	return metadata, scanner.Err()
}

// GetBackupInfo returns information about a backup file
func (m *Manager) GetBackupInfo(backupPath string) (*BackupInfo, error) {
	metadata, err := m.readMetadataFromTar(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup metadata: %w", err)
	}

	stat, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	info := &BackupInfo{
		Name:         filepath.Base(backupPath),
		Path:         backupPath,
		InstanceName: metadata["instance_name"],
		ServiceType:  metadata["service_type"],
		Version:      metadata["version"],
		Size:         stat.Size(),
	}

	if volumes, ok := metadata["volumes"]; ok && volumes != "" {
		info.Volumes = strings.Split(volumes, ",")
	}

	if envFiles, ok := metadata["env_files"]; ok && envFiles != "" {
		info.EnvFiles = strings.Split(envFiles, ",")
	}

	return info, nil
}
