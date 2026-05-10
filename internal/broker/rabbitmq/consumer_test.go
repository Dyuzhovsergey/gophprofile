package rabbitmq

import (
	"errors"
	"testing"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
)

func TestNewConsumer_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.RabbitMQConfig
		wantErr error
	}{
		{
			name:    "empty url",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.URL = "" }),
			wantErr: ErrEmptyRabbitMQURL,
		},
		{
			name:    "empty exchange",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.Exchange = "" }),
			wantErr: ErrEmptyRabbitMQExchange,
		},
		{
			name:    "empty upload queue",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.UploadQueue = "" }),
			wantErr: ErrEmptyRabbitMQQueue,
		},
		{
			name:    "empty upload routing key",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.UploadRoutingKey = "" }),
			wantErr: ErrEmptyRabbitMQRoutingKey,
		},
		{
			name:    "empty delete queue",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.DeleteQueue = "" }),
			wantErr: ErrEmptyRabbitMQQueue,
		},
		{
			name:    "empty delete routing key",
			cfg:     consumerValidConfigWith(func(cfg *config.RabbitMQConfig) { cfg.DeleteRoutingKey = "" }),
			wantErr: ErrEmptyRabbitMQRoutingKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if consumer != nil {
				t.Fatal("expected nil consumer")
			}
		})
	}
}

func consumerValidConfigWith(change func(cfg *config.RabbitMQConfig)) config.RabbitMQConfig {
	cfg := config.RabbitMQConfig{
		URL:              "amqp://guest:guest@localhost:5672/",
		Exchange:         "avatars.exchange",
		UploadQueue:      "avatar.uploaded.queue",
		UploadRoutingKey: "avatar.uploaded",
		DeleteQueue:      "avatar.deleted.queue",
		DeleteRoutingKey: "avatar.deleted",
	}

	change(&cfg)

	return cfg
}
