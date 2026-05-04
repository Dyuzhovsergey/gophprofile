// Package config содержит загрузку и хранение конфигурации приложения GophProfile.
package config

import "os"

const (
	defaultServerAddress = ":8080"
	defaultLogLevel      = "info"

	envServerAddress = "GOPHPROFILE_SERVER_ADDRESS"
	envLogLevel      = "GOPHPROFILE_LOG_LEVEL"
	envDatabaseDSN   = "GOPHPROFILE_DATABASE_DSN"
)

// ServerConfig хранит настройки HTTP-сервера.
type ServerConfig struct {
	Address     string
	LogLevel    string
	DatabaseDSN string
	S3          S3Config
}

// LoadServer загружает конфигурацию HTTP-сервера из переменных окружения.
func LoadServer() ServerConfig {
	return ServerConfig{
		Address:     getEnv(envServerAddress, defaultServerAddress),
		LogLevel:    getEnv(envLogLevel, defaultLogLevel),
		DatabaseDSN: os.Getenv(envDatabaseDSN),
		S3:          LoadS3(),
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
