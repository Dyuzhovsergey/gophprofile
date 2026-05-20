package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
)

// AvatarUploadHandler обрабатывает событие загрузки аватарки.
type AvatarUploadHandler func(ctx context.Context, event domain.AvatarUploadEvent) error

// AvatarDeletedHandler обрабатывает событие удаления аватарки.
type AvatarDeletedHandler func(ctx context.Context, event domain.AvatarDeletedEvent) error

// Consumer читает события аватарок из RabbitMQ.
type Consumer struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	uploadQueue string
	deleteQueue string
}

// rabbitMQConsumeAttrs возвращает общие атрибуты для RabbitMQ consume span.
func rabbitMQConsumeAttrs(delivery amqp.Delivery, queue string, eventType string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.operation", "consume"),
		attribute.String("messaging.destination.name", strings.TrimSpace(queue)),
		attribute.String("messaging.rabbitmq.exchange", strings.TrimSpace(delivery.Exchange)),
		attribute.String("messaging.rabbitmq.routing_key", strings.TrimSpace(delivery.RoutingKey)),
		attribute.String("messaging.message.id", strings.TrimSpace(delivery.MessageId)),
		attribute.String("messaging.message.type", strings.TrimSpace(eventType)),
		attribute.Int("messaging.message.body.size", len(delivery.Body)),
	}
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

	consumer := &Consumer{
		conn:        conn,
		channel:     channel,
		uploadQueue: cfg.UploadQueue,
		deleteQueue: cfg.DeleteQueue,
	}

	if err := consumer.declareUploadTopology(cfg); err != nil {
		_ = consumer.Close()
		return nil, err
	}

	return consumer, nil
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

	if _, err := c.channel.QueueDeclare(
		cfg.DeleteQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq delete queue: %w", err)
	}

	if err := c.channel.QueueBind(
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

// ConsumeAvatarEvents читает события avatar.uploaded и avatar.deleted.
func (c *Consumer) ConsumeAvatarEvents(
	ctx context.Context,
	uploadHandler AvatarUploadHandler,
	deleteHandler AvatarDeletedHandler,
) error {
	uploadDeliveries, err := c.channel.ConsumeWithContext(
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

	deleteDeliveries, err := c.channel.ConsumeWithContext(
		ctx,
		c.deleteQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume avatar deleted queue: %w", err)
	}

	for uploadDeliveries != nil || deleteDeliveries != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case delivery, ok := <-uploadDeliveries:
			if !ok {
				uploadDeliveries = nil
				continue
			}

			if err := handleAvatarUploadedDelivery(ctx, c.uploadQueue, delivery, uploadHandler); err != nil {
				return err
			}

		case delivery, ok := <-deleteDeliveries:
			if !ok {
				deleteDeliveries = nil
				continue
			}

			if err := handleAvatarDeletedDelivery(ctx, c.deleteQueue, delivery, deleteHandler); err != nil {
				return err
			}
		}
	}

	return nil
}

// handleAvatarUploadedDelivery обрабатывает одно сообщение avatar.uploaded.
func handleAvatarUploadedDelivery(
	ctx context.Context,
	queue string,
	delivery amqp.Delivery,
	handler AvatarUploadHandler,
) error {
	deliveryCtx := extractTraceHeaders(ctx, delivery.Headers)

	deliveryCtx, span := observabilitytracing.StartSpan(
		deliveryCtx,
		"rabbitmq.consume",
		rabbitMQConsumeAttrs(delivery, queue, eventTypeAvatarUploaded)...,
	)

	var spanErr error
	defer func() {
		observabilitytracing.RecordError(span, spanErr)
		span.End()
	}()

	var event domain.AvatarUploadEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		spanErr = err

		_ = delivery.Nack(false, false)

		return nil
	}

	span.SetAttributes(
		attribute.String("avatar_id", event.AvatarID),
		attribute.String("user_id", event.UserID),
		attribute.String("s3_key", event.S3Key),
	)

	if err := handler(deliveryCtx, event); err != nil {
		spanErr = err

		_ = delivery.Nack(false, false)

		return nil
	}

	if err := delivery.Ack(false); err != nil {
		spanErr = err

		return fmt.Errorf("ack avatar uploaded event: %w", err)
	}

	return nil
}

// handleAvatarDeletedDelivery обрабатывает одно сообщение avatar.deleted.
func handleAvatarDeletedDelivery(
	ctx context.Context,
	queue string,
	delivery amqp.Delivery,
	handler AvatarDeletedHandler,
) error {
	deliveryCtx := extractTraceHeaders(ctx, delivery.Headers)

	deliveryCtx, span := observabilitytracing.StartSpan(
		deliveryCtx,
		"rabbitmq.consume",
		rabbitMQConsumeAttrs(delivery, queue, eventTypeAvatarDeleted)...,
	)

	var spanErr error
	defer func() {
		observabilitytracing.RecordError(span, spanErr)
		span.End()
	}()

	var event domain.AvatarDeletedEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		spanErr = err

		_ = delivery.Nack(false, false)

		return nil
	}

	span.SetAttributes(
		attribute.String("avatar_id", event.AvatarID),
		attribute.String("user_id", event.UserID),
		attribute.String("s3_key", event.S3Key),
		attribute.Int("thumbnails_count", len(event.ThumbnailS3Keys)),
	)

	if err := handler(deliveryCtx, event); err != nil {
		spanErr = err

		_ = delivery.Nack(false, false)

		return nil
	}

	if err := delivery.Ack(false); err != nil {
		spanErr = err

		return fmt.Errorf("ack avatar deleted event: %w", err)
	}

	return nil
}
