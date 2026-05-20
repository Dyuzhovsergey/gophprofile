// Package metrics содержит HTTP endpoint и будущие метрики Prometheus для GophProfile.
package metrics

import (
	"net/http"
	"strings"

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

// NewConfig создаёт конфигурацию метрик для конкретного сервиса.
func NewConfig(serviceName string) Config {
	return Config{
		ServiceName: strings.TrimSpace(serviceName),
	}
}

// Handler возвращает HTTP handler для Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
