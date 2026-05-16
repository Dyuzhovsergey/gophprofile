// Package logging содержит общие настройки observability-логирования.
package logging

import "strings"

const (
	// ServiceNameServer используется в логах HTTP-сервера.
	ServiceNameServer = "gophprofile-server"

	// ServiceNameWorker используется в логах worker-приложения.
	ServiceNameWorker = "gophprofile-worker"

	// DefaultEnvironment задаёт окружение по умолчанию для локальной разработки.
	DefaultEnvironment = "local"
)

// Config хранит базовые настройки observability-логирования.
type Config struct {
	ServiceName string
	Environment string
}

// NewConfig создаёт конфигурацию логирования с безопасными значениями по умолчанию.
func NewConfig(serviceName string, environment string) Config {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = ServiceNameServer
	}

	environment = strings.TrimSpace(environment)
	if environment == "" {
		environment = DefaultEnvironment
	}

	return Config{
		ServiceName: serviceName,
		Environment: environment,
	}
}
