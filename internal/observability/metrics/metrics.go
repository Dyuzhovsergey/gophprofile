// Package metrics содержит общую заготовку для Prometheus-метрик GophProfile.
package metrics

import "strings"

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
