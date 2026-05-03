package config

// WorkerConfig хранит настройки worker-приложения.
type WorkerConfig struct {
	LogLevel string
}

// LoadWorker загружает конфигурацию worker-а из переменных окружения.
func LoadWorker() WorkerConfig {
	return WorkerConfig{
		LogLevel: getEnv(envLogLevel, defaultLogLevel),
	}
}
