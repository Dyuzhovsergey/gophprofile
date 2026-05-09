package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	// ErrEmptyRabbitMQURL означает, что не указан URL подключения к RabbitMQ.
	ErrEmptyRabbitMQURL = errors.New("rabbitmq url is empty")

	// ErrEmptyRabbitMQExchange означает, что не указано имя exchange.
	ErrEmptyRabbitMQExchange = errors.New("rabbitmq exchange is empty")

	// ErrEmptyRabbitMQQueue означает, что не указано имя queue.
	ErrEmptyRabbitMQQueue = errors.New("rabbitmq queue is empty")

	// ErrEmptyRabbitMQRoutingKey означает, что не указан routing key.
	ErrEmptyRabbitMQRoutingKey = errors.New("rabbitmq routing key is empty")
)

const (
	exchangeTypeTopic       = "topic"
	contentTypeJSON         = "application/json"
	eventTypeAvatarUploaded = "avatar.uploaded"
	eventTypeAvatarDeleted  = "avatar.deleted"
)

// Publisher публикует события аватарок в RabbitMQ.
type Publisher struct {
	conn             *amqp.Connection
	channel          *amqp.Channel
	exchange         string
	uploadRoutingKey string
	deleteRoutingKey string
}

// NewPublisher создаёт RabbitMQ publisher и объявляет exchange, queue и binding.
func NewPublisher(cfg config.RabbitMQConfig) (*Publisher, error) {
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

	if strings.TrimSpace(cfg.DeleteQueue) == "" {
		return nil, ErrEmptyRabbitMQQueue
	}

	if strings.TrimSpace(cfg.DeleteRoutingKey) == "" {
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

	publisher := &Publisher{
		conn:             conn,
		channel:          channel,
		exchange:         cfg.Exchange,
		uploadRoutingKey: cfg.UploadRoutingKey,
		deleteRoutingKey: cfg.DeleteRoutingKey,
	}

	if err := publisher.declareUploadTopology(cfg); err != nil {
		_ = publisher.Close()
		return nil, err
	}

	if err := publisher.declareDeleteTopology(cfg); err != nil {
		_ = publisher.Close()
		return nil, err
	}

	return publisher, nil
}

// PublishAvatarUploaded публикует событие загрузки аватарки.
func (p *Publisher) PublishAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal avatar upload event: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		p.exchange,
		p.uploadRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  contentTypeJSON,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    uuid.NewString(),
			Type:         eventTypeAvatarUploaded,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish avatar upload event: %w", err)
	}

	return nil
}

// Close закрывает канал и соединение с RabbitMQ.
func (p *Publisher) Close() error {
	var resultErr error

	if p.channel != nil {
		resultErr = errors.Join(resultErr, p.channel.Close())
	}

	if p.conn != nil {
		resultErr = errors.Join(resultErr, p.conn.Close())
	}

	return resultErr
}

// declareUploadTopology объявляет exchange, queue и binding для avatar.uploaded.
func (p *Publisher) declareUploadTopology(cfg config.RabbitMQConfig) error {
	if err := p.channel.ExchangeDeclare(
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

	if _, err := p.channel.QueueDeclare(
		cfg.UploadQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq upload queue: %w", err)
	}

	if err := p.channel.QueueBind(
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

// PublishAvatarDeleted публикует событие удаления аватарки.
func (p *Publisher) PublishAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal avatar deleted event: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		p.exchange,
		p.deleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  contentTypeJSON,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    uuid.NewString(),
			Type:         eventTypeAvatarDeleted,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish avatar deleted event: %w", err)
	}

	return nil
}

// declareDeleteTopology объявляет exchange, queue и binding для avatar.deleted.
func (p *Publisher) declareDeleteTopology(cfg config.RabbitMQConfig) error {
	if err := p.channel.ExchangeDeclare(
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

	if _, err := p.channel.QueueDeclare(
		cfg.DeleteQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq delete queue: %w", err)
	}

	if err := p.channel.QueueBind(
		cfg.DeleteQueue,
		cfg.DeleteRoutingKey,
		cfg.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq delete queue: %w", err)
	}

	return nil
}
