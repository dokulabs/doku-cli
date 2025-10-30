package docker

import (
	"fmt"
	"strings"
)

// GenerateContainerName generates a Docker container name with doku prefix
func GenerateContainerName(instanceName string) string {
	return fmt.Sprintf("doku-%s", instanceName)
}

// GenerateVolumeName generates a Docker volume name
func GenerateVolumeName(instanceName, volumeType string) string {
	if volumeType == "" {
		return fmt.Sprintf("doku-%s-data", instanceName)
	}
	return fmt.Sprintf("doku-%s-%s", instanceName, volumeType)
}

// ParseContainerName extracts instance name from container name
// e.g., "doku-postgres-14" -> "postgres-14"
func ParseContainerName(containerName string) string {
	// Remove "doku-" prefix
	if strings.HasPrefix(containerName, "doku-") {
		return strings.TrimPrefix(containerName, "doku-")
	}
	return containerName
}

// IsHealthy checks if a container is in healthy state
func IsHealthy(status string) bool {
	status = strings.ToLower(status)
	return status == "healthy" || status == "running"
}

// GetContainerState returns a simplified container state
func GetContainerState(state string) string {
	state = strings.ToLower(state)

	switch state {
	case "running":
		return "running"
	case "exited", "dead", "removing":
		return "stopped"
	case "paused":
		return "paused"
	case "restarting":
		return "restarting"
	case "created":
		return "created"
	default:
		return "unknown"
	}
}

// BuildImageName constructs a full image name with tag
func BuildImageName(repository, tag string) string {
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s:%s", repository, tag)
}

// SanitizeContainerName ensures container name is valid
func SanitizeContainerName(name string) string {
	// Docker container names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		"/", "-",
		"\\", "-",
		":", "-",
	)

	name = replacer.Replace(name)

	// Remove any remaining invalid characters
	var result strings.Builder
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			result.WriteRune(r)
		} else if i == 0 && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') {
			// First character must be alphanumeric
			result.WriteRune('a')
		}
	}

	return result.String()
}

// FormatContainerID shortens a container ID for display
func FormatContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// ContainerHasLabel checks if a container has a specific label
func ContainerHasLabel(labels map[string]string, key, value string) bool {
	if labels == nil {
		return false
	}

	val, exists := labels[key]
	if !exists {
		return false
	}

	if value == "" {
		return true // Just check if label exists
	}

	return val == value
}

// IsDokuContainer checks if a container is managed by Doku
func IsDokuContainer(labels map[string]string) bool {
	return ContainerHasLabel(labels, "managed-by", "doku")
}

// ExtractInstanceName extracts the Doku instance name from container labels
func ExtractInstanceName(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	return labels["doku.instance"]
}

// CreateEnvVars converts a map to Docker environment variable format
func CreateEnvVars(envMap map[string]string) []string {
	envVars := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}
	return envVars
}

// ParseEnvVars converts Docker environment variable format to map
func ParseEnvVars(envVars []string) map[string]string {
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}

// CreateVolumeMounts creates volume mount configuration
func CreateVolumeMounts(volumes map[string]string) []string {
	mounts := make([]string, 0, len(volumes))
	for source, target := range volumes {
		mounts = append(mounts, fmt.Sprintf("%s:%s", source, target))
	}
	return mounts
}

// NormalizeImageName normalizes an image name
// e.g., "postgres" -> "postgres:latest", "postgres:14" -> "postgres:14"
func NormalizeImageName(image string) string {
	if !strings.Contains(image, ":") {
		return image + ":latest"
	}
	return image
}

// GetImageRepository extracts repository from image name
// e.g., "postgres:14" -> "postgres"
func GetImageRepository(image string) string {
	parts := strings.Split(image, ":")
	return parts[0]
}

// GetImageTag extracts tag from image name
// e.g., "postgres:14" -> "14"
func GetImageTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return "latest"
}
