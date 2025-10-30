package docker

import (
	"fmt"
)

// TraefikLabels holds Traefik routing configuration
type TraefikLabels struct {
	Enabled     bool
	RouterName  string
	ServiceName string
	Subdomain   string
	Domain      string
	Port        int
	EntryPoint  string // "web" for HTTP, "websecure" for HTTPS
	TLS         bool
	Priority    int
	PathPrefix  string
	CustomRules []string
}

// GenerateTraefikLabels generates Docker labels for Traefik routing
func GenerateTraefikLabels(config TraefikLabels) map[string]string {
	if !config.Enabled {
		return map[string]string{
			"traefik.enable": "false",
		}
	}

	// Set defaults
	if config.RouterName == "" {
		config.RouterName = config.ServiceName
	}
	if config.EntryPoint == "" {
		config.EntryPoint = "websecure"
	}

	labels := map[string]string{
		"traefik.enable": "true",
	}

	// HTTP Router configuration
	routerPrefix := fmt.Sprintf("traefik.http.routers.%s", config.RouterName)

	// Host rule
	host := fmt.Sprintf("%s.%s", config.Subdomain, config.Domain)
	rule := fmt.Sprintf("Host(`%s`)", host)

	// Add path prefix if specified
	if config.PathPrefix != "" {
		rule = fmt.Sprintf("%s && PathPrefix(`%s`)", rule, config.PathPrefix)
	}

	// Add custom rules if specified
	if len(config.CustomRules) > 0 {
		for _, customRule := range config.CustomRules {
			rule = fmt.Sprintf("%s && %s", rule, customRule)
		}
	}

	labels[fmt.Sprintf("%s.rule", routerPrefix)] = rule
	labels[fmt.Sprintf("%s.entrypoints", routerPrefix)] = config.EntryPoint

	// Priority (higher priority routes are matched first)
	if config.Priority > 0 {
		labels[fmt.Sprintf("%s.priority", routerPrefix)] = fmt.Sprintf("%d", config.Priority)
	}

	// TLS configuration
	if config.TLS {
		labels[fmt.Sprintf("%s.tls", routerPrefix)] = "true"
	}

	// Service configuration (port)
	servicePrefix := fmt.Sprintf("traefik.http.services.%s", config.ServiceName)
	labels[fmt.Sprintf("%s.loadbalancer.server.port", servicePrefix)] = fmt.Sprintf("%d", config.Port)

	// Docker network (Traefik will use this network)
	labels["traefik.docker.network"] = "doku-network"

	return labels
}

// GenerateTraefikLabelsForService generates labels for a standard service
func GenerateTraefikLabelsForService(instanceName, domain string, port int, useTLS bool) map[string]string {
	entryPoint := "web"
	if useTLS {
		entryPoint = "websecure"
	}

	config := TraefikLabels{
		Enabled:     true,
		RouterName:  instanceName,
		ServiceName: instanceName,
		Subdomain:   instanceName,
		Domain:      domain,
		Port:        port,
		EntryPoint:  entryPoint,
		TLS:         useTLS,
	}

	return GenerateTraefikLabels(config)
}

// GenerateTraefikLabelsForDashboard generates labels for Traefik dashboard
func GenerateTraefikLabelsForDashboard(domain string, useTLS bool) map[string]string {
	entryPoint := "web"
	if useTLS {
		entryPoint = "websecure"
	}

	config := TraefikLabels{
		Enabled:     true,
		RouterName:  "traefik-dashboard",
		ServiceName: "api@internal",
		Subdomain:   "traefik",
		Domain:      domain,
		Port:        8080, // Traefik API port
		EntryPoint:  entryPoint,
		TLS:         useTLS,
	}

	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", config.RouterName): fmt.Sprintf("Host(`%s.%s`)", config.Subdomain, config.Domain),
		fmt.Sprintf("traefik.http.routers.%s.service", config.RouterName): "api@internal",
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", config.RouterName): config.EntryPoint,
	}

	if config.TLS {
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", config.RouterName)] = "true"
	}

	return labels
}

// GenerateDokuManagedLabels generates common labels for Doku-managed containers
func GenerateDokuManagedLabels(instanceName, serviceType, version string) map[string]string {
	return map[string]string{
		"managed-by":           "doku",
		"doku.instance":        instanceName,
		"doku.service.type":    serviceType,
		"doku.service.version": version,
	}
}

// MergeLabels merges multiple label maps into one
func MergeLabels(labelMaps ...map[string]string) map[string]string {
	result := make(map[string]string)

	for _, labels := range labelMaps {
		for key, value := range labels {
			result[key] = value
		}
	}

	return result
}

// DisableTraefikLabels returns labels that disable Traefik for a container
func DisableTraefikLabels() map[string]string {
	return map[string]string{
		"traefik.enable": "false",
	}
}

// TCPRouterLabels generates Traefik labels for TCP routing (for databases, etc.)
type TCPRouterLabels struct {
	Enabled     bool
	RouterName  string
	ServiceName string
	EntryPoint  string
	Port        int
	HostSNI     string // For TLS passthrough
}

// GenerateTCPRouterLabels generates Traefik TCP router labels
func GenerateTCPRouterLabels(config TCPRouterLabels) map[string]string {
	if !config.Enabled {
		return DisableTraefikLabels()
	}

	if config.EntryPoint == "" {
		config.EntryPoint = "tcp"
	}

	if config.HostSNI == "" {
		config.HostSNI = "*" // Accept any SNI
	}

	labels := map[string]string{
		"traefik.enable": "true",
	}

	// TCP Router
	routerPrefix := fmt.Sprintf("traefik.tcp.routers.%s", config.RouterName)
	labels[fmt.Sprintf("%s.rule", routerPrefix)] = fmt.Sprintf("HostSNI(`%s`)", config.HostSNI)
	labels[fmt.Sprintf("%s.entrypoints", routerPrefix)] = config.EntryPoint

	// TCP Service
	servicePrefix := fmt.Sprintf("traefik.tcp.services.%s", config.ServiceName)
	labels[fmt.Sprintf("%s.loadbalancer.server.port", servicePrefix)] = fmt.Sprintf("%d", config.Port)

	return labels
}
