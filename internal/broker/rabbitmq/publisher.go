package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
)

// rabbitMQPublishAttrs возвращает общие атрибуты для RabbitMQ publish span.
func rabbitMQPublishAttrs(
	exchange string,
	routingKey string,
	eventType string,
	bodySize int,
	attrs ...attribute.KeyValue,
) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs)+6)

	result = append(result,
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.operation", "publish"),
		attribute.String("messaging.destination.name", strings.TrimSpace(exchange)),
		attribute.String("messaging.rabbitmq.routing_key", strings.TrimSpace(routingKey)),
		attribute.String("messaging.message.type", strings.TrimSpace(eventType)),
		attribute.Int("messaging.message.body.size", bodySize),
	)

	result = append(result, attrs...)

	return result
}

const (
	exchangeTypeTopic = "topic"
	contentTypeJSON   = "application/json"

	eventTypeAvatarUploaded = "avatar.uploaded"
	eventTypeAvatarDeleted  = "avatar.deleted"

	rabbitMQReconnectInterval = 2 * time.Second
)

// Ошибки RabbitMQ publisher-а.
var (
	// ErrEmptyRabbitMQURL означает, что не указан URL подключения к RabbitMQ.
	ErrEmptyRabbitMQURL = errors.New("rabbitmq url is empty")

	// ErrEmptyRabbitMQExchange означает, что не указано имя exchange.
	ErrEmptyRabbitMQExchange = errors.New("rabbitmq exchange is empty")

	// ErrEmptyRabbitMQQueue означает, что не указано имя queue.
	ErrEmptyRabbitMQQueue = errors.New("rabbitmq queue is empty")

	// ErrEmptyRabbitMQRoutingKey означает, что не указан routing key.
	ErrEmptyRabbitMQRoutingKey = errors.New("rabbitmq routing key is empty")

	// ErrRabbitMQClosed означает, что соединение или канал RabbitMQ закрыты.
	ErrRabbitMQClosed = errors.New("rabbitmq connection is closed")
)

// Publisher публикует события аватарок в RabbitMQ.
type Publisher struct {
	cfg config.RabbitMQConfig

	mu          sync.RWMutex
	reconnectMu sync.Mutex
	closeOnce   sync.Once

	conn    *amqp.Connection
	channel *amqp.Channel

	exchange         string
	uploadRoutingKey string
	deleteRoutingKey string

	closed chan struct{}
}

// NewPublisher создаёт RabbitMQ publisher.
func NewPublisher(cfg config.RabbitMQConfig) (*Publisher, error) {
	if err := validatePublisherConfig(cfg); err != nil {
		return nil, err
	}

	publisher := &Publisher{
		cfg:              cfg,
		exchange:         cfg.Exchange,
		uploadRoutingKey: cfg.UploadRoutingKey,
		deleteRoutingKey: cfg.DeleteRoutingKey,
		closed:           make(chan struct{}),
	}

	if err := publisher.reconnect(context.Background()); err != nil {
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

	return p.publish(
		ctx,
		p.uploadRoutingKey,
		eventTypeAvatarUploaded,
		body,
		attribute.String("avatar_id", event.AvatarID),
		attribute.String("user_id", event.UserID),
		attribute.String("s3_key", event.S3Key),
	)
}

// PublishAvatarDeleted публикует событие удаления аватарки.
func (p *Publisher) PublishAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal avatar deleted event: %w", err)
	}

	return p.publish(
		ctx,
		p.deleteRoutingKey,
		eventTypeAvatarDeleted,
		body,
		attribute.String("avatar_id", event.AvatarID),
		attribute.String("user_id", event.UserID),
		attribute.String("s3_key", event.S3Key),
		attribute.Int("thumbnails_count", len(event.ThumbnailS3Keys)),
	)
}

// Ping проверяет, что соединение и канал RabbitMQ открыты.
func (p *Publisher) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !p.isConnectionReady() {
		return ErrRabbitMQClosed
	}

	return nil
}

// Close закрывает RabbitMQ publisher.
func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}

	p.closeOnce.Do(func() {
		close(p.closed)
	})

	p.mu.Lock()
	defer p.mu.Unlock()

	var resultErr error

	if p.channel != nil && !p.channel.IsClosed() {
		if err := p.channel.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close rabbitmq channel: %w", err))
		}
	}

	if p.conn != nil && !p.conn.IsClosed() {
		if err := p.conn.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close rabbitmq connection: %w", err))
		}
	}

	p.channel = nil
	p.conn = nil

	return resultErr
}

// publish публикует сообщение в RabbitMQ.
func (p *Publisher) publish(
	ctx context.Context,
	routingKey string,
	eventType string,
	body []byte,
	attrs ...attribute.KeyValue,
) (err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"rabbitmq.publish",
		rabbitMQPublishAttrs(
			p.exchange,
			routingKey,
			eventType,
			len(body),
			attrs...,
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if !p.isConnectionReady() {
		if err := p.reconnect(ctx); err != nil {
			return fmt.Errorf("reconnect rabbitmq before publish: %w", err)
		}
	}

	p.mu.RLock()
	channel := p.channel
	exchange := p.exchange
	p.mu.RUnlock()

	if channel == nil || channel.IsClosed() {
		go p.reconnectLoop()
		return ErrRabbitMQClosed
	}

	messageID := uuid.NewString()

	span.SetAttributes(
		attribute.String("messaging.message.id", messageID),
		attribute.String("messaging.destination.name", exchange),
	)

	err = channel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  contentTypeJSON,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    messageID,
			Type:         eventType,
			Headers:      injectTraceHeaders(ctx),
			Body:         body,
		},
	)
	if err != nil {
		go p.reconnectLoop()

		return fmt.Errorf("publish rabbitmq event: %w", err)
	}

	return nil
}

// reconnect переподключается к RabbitMQ и заново объявляет topology.
func (p *Publisher) reconnect(ctx context.Context) error {
	p.reconnectMu.Lock()
	defer p.reconnectMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if p.isClosed() {
		return ErrRabbitMQClosed
	}

	conn, err := amqp.Dial(p.cfg.URL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()

		return fmt.Errorf("create rabbitmq channel: %w", err)
	}

	if err := declarePublisherTopology(channel, p.cfg); err != nil {
		_ = channel.Close()
		_ = conn.Close()

		return err
	}

	p.mu.Lock()
	oldConn := p.conn
	oldChannel := p.channel

	p.conn = conn
	p.channel = channel
	p.mu.Unlock()

	if oldChannel != nil && !oldChannel.IsClosed() {
		_ = oldChannel.Close()
	}

	if oldConn != nil && !oldConn.IsClosed() {
		_ = oldConn.Close()
	}

	p.watchChannelClose(channel)

	return nil
}

// reconnectLoop пытается переподключиться к RabbitMQ до успеха или закрытия publisher-а.
func (p *Publisher) reconnectLoop() {
	if !p.reconnectMu.TryLock() {
		return
	}
	p.reconnectMu.Unlock()

	for {
		if p.isClosed() {
			return
		}

		if err := p.reconnect(context.Background()); err == nil {
			return
		}

		select {
		case <-p.closed:
			return
		case <-time.After(rabbitMQReconnectInterval):
		}
	}
}

// watchChannelClose следит за закрытием RabbitMQ channel.
func (p *Publisher) watchChannelClose(channel *amqp.Channel) {
	notifyClose := channel.NotifyClose(make(chan *amqp.Error, 1))

	go func() {
		select {
		case <-p.closed:
			return

		case _, ok := <-notifyClose:
			if !ok || p.isClosed() {
				return
			}

			go p.reconnectLoop()
		}
	}()
}

// isConnectionReady проверяет, что соединение и канал готовы к публикации.
func (p *Publisher) isConnectionReady() bool {
	if p == nil {
		return false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.conn == nil || p.channel == nil {
		return false
	}

	if p.conn.IsClosed() || p.channel.IsClosed() {
		return false
	}

	return true
}

// isClosed проверяет, закрыт ли publisher.
func (p *Publisher) isClosed() bool {
	if p == nil {
		return true
	}

	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}

// validatePublisherConfig проверяет настройки RabbitMQ publisher-а.
func validatePublisherConfig(cfg config.RabbitMQConfig) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return ErrEmptyRabbitMQURL
	}

	if strings.TrimSpace(cfg.Exchange) == "" {
		return ErrEmptyRabbitMQExchange
	}

	if strings.TrimSpace(cfg.UploadQueue) == "" {
		return ErrEmptyRabbitMQQueue
	}

	if strings.TrimSpace(cfg.UploadRoutingKey) == "" {
		return ErrEmptyRabbitMQRoutingKey
	}

	if strings.TrimSpace(cfg.DeleteQueue) == "" {
		return ErrEmptyRabbitMQQueue
	}

	if strings.TrimSpace(cfg.DeleteRoutingKey) == "" {
		return ErrEmptyRabbitMQRoutingKey
	}

	return nil
}

// declarePublisherTopology объявляет exchange, queue и binding для событий аватарок.
func declarePublisherTopology(channel *amqp.Channel, cfg config.RabbitMQConfig) error {
	if err := channel.ExchangeDeclare(
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

	if err := declareQueueBinding(
		channel,
		cfg.UploadQueue,
		cfg.UploadRoutingKey,
		cfg.Exchange,
	); err != nil {
		return fmt.Errorf("declare upload queue binding: %w", err)
	}

	if err := declareQueueBinding(
		channel,
		cfg.DeleteQueue,
		cfg.DeleteRoutingKey,
		cfg.Exchange,
	); err != nil {
		return fmt.Errorf("declare delete queue binding: %w", err)
	}

	return nil
}

// declareQueueBinding объявляет durable queue и binding к exchange.
func declareQueueBinding(
	channel *amqp.Channel,
	queue string,
	routingKey string,
	exchange string,
) error {
	if _, err := channel.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare rabbitmq queue: %w", err)
	}

	if err := channel.QueueBind(
		queue,
		routingKey,
		exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind rabbitmq queue: %w", err)
	}

	return nil
}
