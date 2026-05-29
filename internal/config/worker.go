package config

import "os"

const (
	defaultWorkerMetricsAddress = ":9091"

	envWorkerMetricsAddress = "GOPHPROFILE_WORKER_METRICS_ADDRESS"
)

// WorkerConfig хранит настройки worker-приложения.
type WorkerConfig struct {
	LogLevel       string
	DatabaseDSN    string
	MetricsAddress string
	S3             S3Config
	RabbitMQ       RabbitMQConfig
	Tracing        TracingConfig
}

// LoadWorker загружает конфигурацию worker-а из переменных окружения.
func LoadWorker() WorkerConfig {
	return WorkerConfig{
		LogLevel:       getEnv(envLogLevel, defaultLogLevel),
		DatabaseDSN:    os.Getenv(envDatabaseDSN),
		MetricsAddress: getEnv(envWorkerMetricsAddress, defaultWorkerMetricsAddress),
		S3:             LoadS3(),
		RabbitMQ:       LoadRabbitMQ(),
		Tracing:        LoadTracing(defaultWorkerServiceName),
	}
}
