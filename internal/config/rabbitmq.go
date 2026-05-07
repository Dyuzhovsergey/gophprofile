package config

const (
	defaultRabbitMQExchange         = "avatars.exchange"
	defaultRabbitMQUploadQueue      = "avatar.uploaded.queue"
	defaultRabbitMQUploadRoutingKey = "avatar.uploaded"

	envRabbitMQURL              = "GOPHPROFILE_RABBITMQ_URL"
	envRabbitMQExchange         = "GOPHPROFILE_RABBITMQ_EXCHANGE"
	envRabbitMQUploadQueue      = "GOPHPROFILE_RABBITMQ_UPLOAD_QUEUE"
	envRabbitMQUploadRoutingKey = "GOPHPROFILE_RABBITMQ_UPLOAD_ROUTING_KEY"
)

// RabbitMQConfig хранит настройки подключения к RabbitMQ.
type RabbitMQConfig struct {
	URL              string
	Exchange         string
	UploadQueue      string
	UploadRoutingKey string
}

// LoadRabbitMQ загружает настройки RabbitMQ из переменных окружения.
func LoadRabbitMQ() RabbitMQConfig {
	return RabbitMQConfig{
		URL:              getEnv(envRabbitMQURL, ""),
		Exchange:         getEnv(envRabbitMQExchange, defaultRabbitMQExchange),
		UploadQueue:      getEnv(envRabbitMQUploadQueue, defaultRabbitMQUploadQueue),
		UploadRoutingKey: getEnv(envRabbitMQUploadRoutingKey, defaultRabbitMQUploadRoutingKey),
	}
}