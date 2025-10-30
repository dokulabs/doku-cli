package docker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

// ResourceConfig holds resource limit configuration
type ResourceConfig struct {
	MemoryLimit   string // e.g., "512m", "1g"
	MemoryReserve string
	CPULimit      string // e.g., "0.5", "1.0", "2"
	CPUReserve    string
}

// ApplyResourceLimits applies resource limits to container host config
func ApplyResourceLimits(hostConfig *container.HostConfig, config ResourceConfig) error {
	if config.MemoryLimit != "" {
		memBytes, err := ParseMemoryString(config.MemoryLimit)
		if err != nil {
			return fmt.Errorf("invalid memory limit: %w", err)
		}
		hostConfig.Resources.Memory = memBytes
	}

	if config.MemoryReserve != "" {
		memBytes, err := ParseMemoryString(config.MemoryReserve)
		if err != nil {
			return fmt.Errorf("invalid memory reservation: %w", err)
		}
		hostConfig.Resources.MemoryReservation = memBytes
	}

	if config.CPULimit != "" {
		cpuQuota, cpuPeriod, err := ParseCPUString(config.CPULimit)
		if err != nil {
			return fmt.Errorf("invalid CPU limit: %w", err)
		}
		hostConfig.Resources.CPUQuota = cpuQuota
		hostConfig.Resources.CPUPeriod = cpuPeriod
	}

	return nil
}

// ParseMemoryString converts memory strings like "512m", "1g" to bytes
func ParseMemoryString(mem string) (int64, error) {
	if mem == "" {
		return 0, fmt.Errorf("empty memory value")
	}

	mem = strings.ToLower(strings.TrimSpace(mem))

	// Extract number and unit
	var value int64
	var unit string

	// Find where the number ends and unit begins
	i := 0
	for i < len(mem) && (mem[i] >= '0' && mem[i] <= '9') {
		i++
	}

	if i == 0 {
		return 0, fmt.Errorf("no numeric value found in: %s", mem)
	}

	numStr := mem[:i]
	if i < len(mem) {
		unit = mem[i:]
	}

	// Parse the numeric part
	var err error
	value, err = strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value: %s", numStr)
	}

	// Apply unit multiplier
	switch unit {
	case "b", "":
		return value, nil
	case "k", "kb":
		return value * 1024, nil
	case "m", "mb":
		return value * 1024 * 1024, nil
	case "g", "gb":
		return value * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}
}

// ParseCPUString converts CPU strings like "0.5", "1", "2" to Docker CPU quota and period
// Docker uses CPUPeriod (default 100000) and CPUQuota to represent CPU limits
// For example, 0.5 CPUs = CPUQuota 50000 with CPUPeriod 100000
func ParseCPUString(cpu string) (quota int64, period int64, err error) {
	if cpu == "" {
		return 0, 0, fmt.Errorf("empty CPU value")
	}

	cpu = strings.TrimSpace(cpu)

	// Parse as float
	cpuFloat, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid CPU value: %s", cpu)
	}

	if cpuFloat <= 0 {
		return 0, 0, fmt.Errorf("CPU value must be greater than 0")
	}

	// Docker's default CPUPeriod
	period = 100000

	// Calculate quota based on CPU cores
	// 1.0 = 100000 quota, 0.5 = 50000 quota, 2.0 = 200000 quota
	quota = int64(cpuFloat * float64(period))

	return quota, period, nil
}

// FormatMemoryBytes converts bytes to human-readable format
func FormatMemoryBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"K", "M", "G", "T", "P", "E"}
	return fmt.Sprintf("%.1f %sB", float64(bytes)/float64(div), units[exp])
}

// FormatCPUQuota converts Docker CPU quota/period to human-readable cores
func FormatCPUQuota(quota, period int64) string {
	if period == 0 {
		return "unlimited"
	}

	cores := float64(quota) / float64(period)
	return fmt.Sprintf("%.2f cores", cores)
}

// CreateRestartPolicy creates a container restart policy
func CreateRestartPolicy(policy string) container.RestartPolicy {
	switch policy {
	case "always":
		return container.RestartPolicy{
			Name: "always",
		}
	case "unless-stopped":
		return container.RestartPolicy{
			Name: "unless-stopped",
		}
	case "on-failure":
		return container.RestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 3,
		}
	case "no":
		return container.RestartPolicy{
			Name: "no",
		}
	default:
		// Default to unless-stopped for services
		return container.RestartPolicy{
			Name: "unless-stopped",
		}
	}
}

// PortBinding creates a port binding configuration
type PortBinding struct {
	ContainerPort int
	HostPort      int
	Protocol      string // "tcp" or "udp"
}

// CreatePortBindings creates Docker port bindings from PortBinding configs
func CreatePortBindings(bindings []PortBinding) nat.PortMap {
	// For Doku, we typically don't expose ports to host (Traefik handles routing)
	// This is mainly for debugging or special cases
	portMap := make(nat.PortMap)

	for _, binding := range bindings {
		protocol := binding.Protocol
		if protocol == "" {
			protocol = "tcp"
		}

		portKey := nat.Port(fmt.Sprintf("%d/%s", binding.ContainerPort, protocol))
		hostBinding := nat.PortBinding{}

		if binding.HostPort > 0 {
			hostBinding.HostPort = fmt.Sprintf("%d", binding.HostPort)
		}

		portMap[portKey] = append(portMap[portKey], hostBinding)
	}

	return portMap
}
