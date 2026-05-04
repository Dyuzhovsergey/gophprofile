package config

import "os"

// WorkerConfig хранит настройки worker-приложения.
type WorkerConfig struct {
	LogLevel    string
	DatabaseDSN string
	S3          S3Config
}

// LoadWorker загружает конфигурацию worker-а из переменных окружения.
func LoadWorker() WorkerConfig {
	return WorkerConfig{
		LogLevel:    getEnv(envLogLevel, defaultLogLevel),
		DatabaseDSN: os.Getenv(envDatabaseDSN),
		S3:          LoadS3(),
	}
}
