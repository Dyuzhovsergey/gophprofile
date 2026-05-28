// Package metrics содержит Prometheus-метрики GophProfile.
package metrics

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace используется как общий префикс Prometheus-метрик проекта.
	Namespace = "gophprofile"
)

// Config хранит базовые настройки метрик приложения.
type Config struct {
	ServiceName string
}

// AppMetrics объединяет все Prometheus-метрики приложения.
type AppMetrics struct {
	registry *prometheus.Registry

	HTTP   *HTTPMetrics
	Avatar *AvatarMetrics
	Worker *WorkerMetrics
}

// New создаёт метрики приложения с отдельным Prometheus registry.
func New() *AppMetrics {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return NewWithRegistry(registry)
}

// NewWithRegistry создаёт метрики приложения с переданным registry.
//
// Функция полезна для тестов: каждый тест может создать свой registry
// и избежать повторной регистрации одних и тех же метрик.
func NewWithRegistry(registry *prometheus.Registry) *AppMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	return &AppMetrics{
		registry: registry,
		HTTP:     NewHTTPMetrics(registry),
		Avatar:   NewAvatarMetrics(registry),
		Worker:   NewWorkerMetrics(registry),
	}
}

// NewConfig создаёт конфигурацию метрик для конкретного сервиса.
func NewConfig(serviceName string) Config {
	return Config{
		ServiceName: strings.TrimSpace(serviceName),
	}
}

// Handler возвращает HTTP handler для Prometheus metrics.
func (m *AppMetrics) Handler() http.Handler {
	if m == nil || m.registry == nil {
		return promhttp.HandlerFor(prometheus.NewRegistry(), promhttp.HandlerOpts{})
	}

	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
