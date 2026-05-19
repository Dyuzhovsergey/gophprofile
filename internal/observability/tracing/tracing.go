// Package tracing содержит инициализацию OpenTelemetry tracing.
package tracing

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	// InstrumentationName задаёт имя instrumentation library для spans проекта.
	InstrumentationName = "github.com/Dyuzhovsergey/gophprofile"
)

var (
	// ErrEmptyServiceName возвращается, когда tracing включён без имени сервиса.
	ErrEmptyServiceName = errors.New("tracing service name is empty")

	// ErrEmptyExporterEndpoint возвращается, когда tracing включён без OTLP endpoint.
	ErrEmptyExporterEndpoint = errors.New("tracing exporter endpoint is empty")
)

// Config хранит базовые настройки OpenTelemetry tracing.
type Config struct {
	ServiceName      string
	ExporterEndpoint string
	Enabled          bool
}

// ShutdownFunc завершает работу tracer provider и отправляет накопленные spans.
type ShutdownFunc func(ctx context.Context) error

// NewConfig создаёт конфигурацию tracing для конкретного сервиса.
func NewConfig(serviceName string, exporterEndpoint string, enabled bool) Config {
	return Config{
		ServiceName:      strings.TrimSpace(serviceName),
		ExporterEndpoint: strings.TrimSpace(exporterEndpoint),
		Enabled:          enabled,
	}
}

// InitProvider инициализирует OpenTelemetry tracer provider.
//
// Если tracing выключен, возвращается no-op shutdown-функция.
func InitProvider(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	if !cfg.Enabled {
		return noopShutdown, nil
	}

	if cfg.ServiceName == "" {
		return nil, ErrEmptyServiceName
	}

	if cfg.ExporterEndpoint == "" {
		return nil, ErrEmptyExporterEndpoint
	}

	exporter, err := otlptracehttp.New(ctx, exporterOptions(cfg.ExporterEndpoint)...)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)

	return provider.Shutdown, nil
}

// exporterOptions возвращает настройки OTLP HTTP exporter.
func exporterOptions(endpoint string) []otlptracehttp.Option {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return []otlptracehttp.Option{
			otlptracehttp.WithEndpointURL(endpoint),
		}
	}

	return []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	}
}

// noopShutdown ничего не делает, когда tracing выключен.
func noopShutdown(context.Context) error {
	return nil
}
