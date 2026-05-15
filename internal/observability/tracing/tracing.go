// Package tracing содержит общую заготовку для OpenTelemetry tracing.
package tracing

import "strings"

const (
	// InstrumentationName задаёт имя instrumentation library для spans проекта.
	InstrumentationName = "github.com/Dyuzhovsergey/gophprofile"
)

// Config хранит базовые настройки OpenTelemetry tracing.
type Config struct {
	ServiceName      string
	ExporterEndpoint string
	Enabled          bool
}

// NewConfig создаёт конфигурацию tracing для конкретного сервиса.
func NewConfig(serviceName string, exporterEndpoint string, enabled bool) Config {
	return Config{
		ServiceName:      strings.TrimSpace(serviceName),
		ExporterEndpoint: strings.TrimSpace(exporterEndpoint),
		Enabled:          enabled,
	}
}
