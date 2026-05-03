// Package config contains configuration for the server.
package config

import "os"

const (
	defaultServerAddress = ":8080"
	defaultLogLevel      = "info"

	envServerAddress = "GOPHPROFILE_SERVER_ADDRESS"
	envLogLevel      = "GOPHPROFILE_LOG_LEVEL"
)

// ServerConfig хранит настройки HTTP-сервера.
type ServerConfig struct {
	Address  string
	LogLevel string
}

// LoadServer загружает конфигурацию HTTP-сервера из переменных окружения.
func LoadServer() ServerConfig {
	return ServerConfig{
		Address:  getEnv(envServerAddress, defaultServerAddress),
		LogLevel: getEnv(envLogLevel, defaultLogLevel),
	}
}

// getEnv возвращает значение переменной окружения или значение по умолчанию.
func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
