package config

const (
	defaultRabbitMQExchange         = "avatars.exchange"
	defaultRabbitMQUploadQueue      = "avatar.uploaded.queue"
	defaultRabbitMQUploadRoutingKey = "avatar.uploaded"
	defaultRabbitMQDeleteQueue      = "avatar.deleted.queue"
	defaultRabbitMQDeleteRoutingKey = "avatar.deleted"

	envRabbitMQURL              = "GOPHPROFILE_RABBITMQ_URL"
	envRabbitMQExchange         = "GOPHPROFILE_RABBITMQ_EXCHANGE"
	envRabbitMQUploadQueue      = "GOPHPROFILE_RABBITMQ_UPLOAD_QUEUE"
	envRabbitMQUploadRoutingKey = "GOPHPROFILE_RABBITMQ_UPLOAD_ROUTING_KEY"
	envRabbitMQDeleteQueue      = "GOPHPROFILE_RABBITMQ_DELETE_QUEUE"
	envRabbitMQDeleteRoutingKey = "GOPHPROFILE_RABBITMQ_DELETE_ROUTING_KEY"
)

// RabbitMQConfig хранит настройки подключения к RabbitMQ.
type RabbitMQConfig struct {
	URL              string
	Exchange         string
	UploadQueue      string
	UploadRoutingKey string
	DeleteQueue      string
	DeleteRoutingKey string
}

// LoadRabbitMQ загружает настройки RabbitMQ из переменных окружения.
func LoadRabbitMQ() RabbitMQConfig {
	return RabbitMQConfig{
		URL:              getEnv(envRabbitMQURL, ""),
		Exchange:         getEnv(envRabbitMQExchange, defaultRabbitMQExchange),
		UploadQueue:      getEnv(envRabbitMQUploadQueue, defaultRabbitMQUploadQueue),
		UploadRoutingKey: getEnv(envRabbitMQUploadRoutingKey, defaultRabbitMQUploadRoutingKey),
		DeleteQueue:      getEnv(envRabbitMQDeleteQueue, defaultRabbitMQDeleteQueue),
		DeleteRoutingKey: getEnv(envRabbitMQDeleteRoutingKey, defaultRabbitMQDeleteRoutingKey),
	}
}
