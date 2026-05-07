package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

// AvatarUploadHandler обрабатывает событие загрузки аватарки.
type AvatarUploadHandler func(ctx context.Context, event domain.AvatarUploadEvent) error

// Consumer читает события аватарок из RabbitMQ.
type Consumer struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	uploadQueue string
}

// NewConsumer создаёт RabbitMQ consumer и объявляет exchange, queue и binding.
func NewConsumer(cfg config.RabbitMQConfig) (*Consumer, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, ErrEmptyRabbitMQURL
	}

	if strings.TrimSpace(cfg.Exchange) == "" {
		return nil, ErrEmptyRabbitMQExchange
	}

	if strings.TrimSpace(cfg.UploadQueue) == "" {
		return nil, ErrEmptyRabbitMQQueue
	}

	if strings.TrimSpace(cfg.UploadRoutingKey) == "" {
		return nil, ErrEmptyRabbitMQRoutingKey
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	consumer := &Consumer{
		conn:        conn,
		channel:     channel,
		uploadQueue: cfg.UploadQueue,
	}

	if err := consumer.declareUploadTopology(cfg); err != nil {
		_ = consumer.Close()
		return nil, err
	}

	return consumer, nil
}

// ConsumeAvatarUploaded читает события avatar.uploaded и передаёт их handler-у.
func (c *Consumer) ConsumeAvatarUploaded(ctx context.Context, handler AvatarUploadHandler) error {
	deliveries, err := c.channel.ConsumeWithContext(
		ctx,
		c.uploadQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume avatar uploaded queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}

			var event domain.AvatarUploadEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(ctx, event); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("ack avatar uploaded event: %w", err)
			}
		}
	}
}

// Close закрывает канал и соединение с RabbitMQ.
func (c *Consumer) Close() error {
	var resultErr error

	if c.channel != nil {
		resultErr = errors.Join(resultErr, c.channel.Close())
	}

	if c.conn != nil {
		resultErr = errors.Join(resultErr, c.conn.Close())
	}

	return resultErr
}

// declareUploadTopology объявляет exchange, queue и binding для avatar.uploaded.
func (c *Consumer) declareUploadTopology(cfg config.RabbitMQConfig) error {
	if err := c.channel.ExchangeDeclare(
		cfg.Exchange,
		exchangeTypeTopic,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq exchange: %w", err)
	}

	if _, err := c.channel.QueueDeclare(
		cfg.UploadQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq upload queue: %w", err)
	}

	if err := c.channel.QueueBind(
		cfg.UploadQueue,
		cfg.UploadRoutingKey,
		cfg.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq upload queue: %w", err)
	}

	return nil
}
