package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultServerServiceName    = "gophprofile-server"
	defaultWorkerServiceName    = "gophprofile-worker"
	defaultOTelEnabled          = false
	defaultOTelExporterEndpoint = "localhost:4318"

	envOTelEnabled          = "GOPHPROFILE_OTEL_ENABLED"
	envOTelExporterEndpoint = "GOPHPROFILE_OTEL_EXPORTER_ENDPOINT"
	envServiceName          = "GOPHPROFILE_SERVICE_NAME"
)

// TracingConfig хранит настройки OpenTelemetry tracing.
type TracingConfig struct {
	Enabled          bool
	ExporterEndpoint string
	ServiceName      string
}

// LoadTracing загружает настройки tracing из переменных окружения.
func LoadTracing(defaultServiceName string) TracingConfig {
	return TracingConfig{
		Enabled:          getBoolEnv(envOTelEnabled, defaultOTelEnabled),
		ExporterEndpoint: getEnv(envOTelExporterEndpoint, defaultOTelExporterEndpoint),
		ServiceName:      getEnv(envServiceName, defaultServiceName),
	}
}

// getBoolEnv возвращает bool-значение переменной окружения или значение по умолчанию.
func getBoolEnv(key string, defaultValue bool) bool {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return defaultValue
	}

	return value
}
