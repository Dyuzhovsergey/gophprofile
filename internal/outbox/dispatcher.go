package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultBatchSize      = 100
	defaultRetryBaseDelay = 5 * time.Second
	defaultRetryMaxDelay  = 5 * time.Minute
	defaultPublishTimeout = 5 * time.Second
)

// Repository описывает методы outbox repository, которые нужны dispatcher-у.
type Repository interface {
	ListPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, lastError string, availableAt time.Time) error
}

// Publisher описывает публикацию событий аватарок в брокер.
type Publisher interface {
	PublishAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error
	PublishAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error
}

// DispatcherConfig хранит настройки outbox dispatcher-а.
type DispatcherConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	PublishTimeout time.Duration
}

// Dispatcher читает pending-события из outbox и публикует их в брокер.
type Dispatcher struct {
	repo      Repository
	publisher Publisher
	log       *slog.Logger
	cfg       DispatcherConfig
	now       func() time.Time
}

// NewDispatcher создаёт dispatcher с настройками по умолчанию.
func NewDispatcher(repo Repository, publisher Publisher, log *slog.Logger) *Dispatcher {
	return NewDispatcherWithConfig(repo, publisher, log, DispatcherConfig{})
}

// NewDispatcherWithConfig создаёт dispatcher с пользовательскими настройками.
func NewDispatcherWithConfig(
	repo Repository,
	publisher Publisher,
	log *slog.Logger,
	cfg DispatcherConfig,
) *Dispatcher {
	if log == nil {
		log = logger.NewNop()
	}

	cfg = normalizeDispatcherConfig(cfg)

	return &Dispatcher{
		repo:      repo,
		publisher: publisher,
		log:       log,
		cfg:       cfg,
		now:       time.Now,
	}
}

// Run запускает периодическую публикацию outbox-событий.
func (d *Dispatcher) Run(ctx context.Context) {
	d.dispatchAndLog(ctx)

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info("outbox dispatcher stopped")
			return

		case <-ticker.C:
			d.dispatchAndLog(ctx)
		}
	}
}

// DispatchOnce выполняет одну итерацию публикации pending outbox-событий.
func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	if d.repo == nil {
		return errors.New("outbox repository is nil")
	}

	if d.publisher == nil {
		return errors.New("outbox publisher is nil")
	}

	events, err := d.repo.ListPending(ctx, d.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("list pending outbox events: %w", err)
	}

	for _, event := range events {
		if err := d.publishEvent(ctx, event); err != nil {
			availableAt := d.now().Add(calculateRetryDelay(
				event.Attempts,
				d.cfg.RetryBaseDelay,
				d.cfg.RetryMaxDelay,
			))

			if markErr := d.repo.MarkFailed(ctx, event.ID, err.Error(), availableAt); markErr != nil {
				return errors.Join(
					fmt.Errorf("publish outbox event %s: %w", event.ID, err),
					fmt.Errorf("mark outbox event failed: %w", markErr),
				)
			}

			d.log.Warn(
				"failed to publish outbox event",
				slog.String("event_id", event.ID),
				slog.String("event_type", string(event.EventType)),
				slog.Int("attempts", event.Attempts),
				slog.Time("next_attempt_at", availableAt),
				logger.Err(err),
			)

			continue
		}

		if err := d.repo.MarkPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark outbox event published: %w", err)
		}

		d.log.Info(
			"outbox event published",
			slog.String("event_id", event.ID),
			slog.String("event_type", string(event.EventType)),
		)
	}

	return nil
}

// dispatchAndLog выполняет одну итерацию dispatcher-а и логирует ошибку.
func (d *Dispatcher) dispatchAndLog(ctx context.Context) {
	if err := d.DispatchOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		d.log.Error("outbox dispatch iteration failed", logger.Err(err))
	}
}

// publishEvent публикует outbox-событие в брокер.
func (d *Dispatcher) publishEvent(ctx context.Context, event domain.OutboxEvent) error {
	publishCtx, cancel := context.WithTimeout(ctx, d.cfg.PublishTimeout)
	defer cancel()

	publishCtx = observabilitytracing.ExtractTextMap(publishCtx, event.Headers)

	switch event.EventType {
	case domain.OutboxEventTypeAvatarUploaded:
		var payload domain.AvatarUploadEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("unmarshal avatar uploaded event: %w", err)
		}

		if err := d.publisher.PublishAvatarUploaded(publishCtx, payload); err != nil {
			return fmt.Errorf("publish avatar uploaded event: %w", err)
		}

		return nil

	case domain.OutboxEventTypeAvatarDeleted:
		var payload domain.AvatarDeletedEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("unmarshal avatar deleted event: %w", err)
		}

		if err := d.publisher.PublishAvatarDeleted(publishCtx, payload); err != nil {
			return fmt.Errorf("publish avatar deleted event: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("%w: %s", domain.ErrInvalidOutboxEventType, event.EventType)
	}
}

// normalizeDispatcherConfig заполняет настройки dispatcher-а значениями по умолчанию.
func normalizeDispatcherConfig(cfg DispatcherConfig) DispatcherConfig {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}

	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = defaultRetryBaseDelay
	}

	if cfg.RetryMaxDelay <= 0 {
		cfg.RetryMaxDelay = defaultRetryMaxDelay
	}

	if cfg.PublishTimeout <= 0 {
		cfg.PublishTimeout = defaultPublishTimeout
	}

	return cfg
}

// calculateRetryDelay рассчитывает задержку перед следующей попыткой публикации.
func calculateRetryDelay(attempts int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempts <= 0 {
		return baseDelay
	}

	delay := baseDelay

	for i := 0; i < attempts; i++ {
		if delay >= maxDelay/2 {
			return maxDelay
		}

		delay *= 2
	}

	if delay > maxDelay {
		return maxDelay
	}

	return delay
}
