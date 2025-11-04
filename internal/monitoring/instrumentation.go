package monitoring

import (
	"fmt"

	dockerTypes "github.com/docker/docker/api/types/container"
	"github.com/dokulabs/doku-cli/pkg/types"
)

// GetInstrumentationEnv returns environment variables for monitoring instrumentation
// These will be automatically injected into all services
func GetInstrumentationEnv(serviceName string, monitoringConfig *types.MonitoringConfig) map[string]string {
	if monitoringConfig == nil || !monitoringConfig.Enabled || monitoringConfig.Tool == "none" {
		return map[string]string{}
	}

	switch monitoringConfig.Tool {
	case "signoz":
		return getSignozEnv(serviceName, monitoringConfig)
	case "sentry":
		return getSentryEnv(serviceName, monitoringConfig)
	default:
		return map[string]string{}
	}
}

// getSignozEnv returns SignOz-specific OpenTelemetry environment variables
// These variables configure automatic instrumentation for OpenTelemetry-compatible services
func getSignozEnv(serviceName string, config *types.MonitoringConfig) map[string]string {
	env := map[string]string{
		// OpenTelemetry Protocol (OTLP) configuration
		"OTEL_EXPORTER_OTLP_ENDPOINT": config.DSN,
		"OTEL_SERVICE_NAME":           serviceName,

		// Resource attributes for better service identification
		"OTEL_RESOURCE_ATTRIBUTES": fmt.Sprintf(
			"service.name=%s,deployment.environment=local,service.namespace=doku",
			serviceName,
		),

		// Enable all telemetry types
		"OTEL_TRACES_EXPORTER":  "otlp",
		"OTEL_METRICS_EXPORTER": "otlp",
		"OTEL_LOGS_EXPORTER":    "otlp",

		// Sampling configuration (always sample in local dev)
		"OTEL_TRACES_SAMPLER":     "always_on",
		"OTEL_TRACES_SAMPLER_ARG": "1.0",

		// Protocol configuration
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",

		// Specific endpoint configurations
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  config.DSN + "/v1/traces",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": config.DSN + "/v1/metrics",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT":    config.DSN + "/v1/logs",
	}

	return env
}

// getSentryEnv returns Sentry-specific environment variables
func getSentryEnv(serviceName string, config *types.MonitoringConfig) map[string]string {
	if config.DSN == "" {
		// DSN not configured yet, return empty
		return map[string]string{}
	}

	env := map[string]string{
		"SENTRY_DSN":         config.DSN,
		"SENTRY_ENVIRONMENT": "local",
		"SENTRY_RELEASE":     serviceName,

		// Performance monitoring
		"SENTRY_TRACES_SAMPLE_RATE":   "1.0",
		"SENTRY_PROFILES_SAMPLE_RATE": "1.0",

		// Additional context
		"SENTRY_SERVER_NAME": serviceName,
		"SENTRY_TAGS":        "deployment:local,managed_by:doku",
	}

	return env
}

// GetDockerLoggingConfig returns Docker logging driver configuration for monitoring
func GetDockerLoggingConfig(monitoringConfig *types.MonitoringConfig) *dockerTypes.LogConfig {
	if monitoringConfig == nil || !monitoringConfig.Enabled || monitoringConfig.Tool == "none" {
		return &dockerTypes.LogConfig{
			Type: "json-file",
			Config: map[string]string{
				"max-size": "10m",
				"max-file": "3",
			},
		}
	}

	// For both SignOz and Sentry, use json-file with appropriate labels
	// This allows log collectors to pick up the logs
	return &dockerTypes.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": "10m",
			"max-file": "3",
			"labels":   "service,monitoring,managed-by",
			"tag":      "{{.Name}}",
		},
	}
}

// GetServiceLabels returns Docker labels for monitoring
func GetServiceLabels(serviceName string, monitoringConfig *types.MonitoringConfig) map[string]string {
	labels := make(map[string]string)

	if monitoringConfig == nil || !monitoringConfig.Enabled || monitoringConfig.Tool == "none" {
		return labels
	}

	// Add monitoring-specific labels
	labels["doku.monitoring.enabled"] = "true"
	labels["doku.monitoring.tool"] = monitoringConfig.Tool
	labels["doku.monitoring.service"] = serviceName

	switch monitoringConfig.Tool {
	case "signoz":
		labels["doku.monitoring.type"] = "opentelemetry"
		labels["doku.monitoring.otlp-endpoint"] = monitoringConfig.DSN

	case "sentry":
		labels["doku.monitoring.type"] = "sentry"
		if monitoringConfig.DSN != "" {
			labels["doku.monitoring.dsn-configured"] = "true"
		} else {
			labels["doku.monitoring.dsn-configured"] = "false"
		}
	}

	return labels
}

// IsMonitoringEnabled checks if monitoring is enabled in config
func IsMonitoringEnabled(monitoringConfig *types.MonitoringConfig) bool {
	return monitoringConfig != nil && monitoringConfig.Enabled && monitoringConfig.Tool != "none"
}

// GetMonitoringInfo returns a human-readable string about monitoring status
func GetMonitoringInfo(monitoringConfig *types.MonitoringConfig) string {
	if !IsMonitoringEnabled(monitoringConfig) {
		return "Monitoring: Disabled"
	}

	toolName := "Unknown"
	switch monitoringConfig.Tool {
	case "signoz":
		toolName = "SignOz"
	case "sentry":
		toolName = "Sentry"
	}

	return fmt.Sprintf("Monitoring: %s (%s)", toolName, monitoringConfig.URL)
}
