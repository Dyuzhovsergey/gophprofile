package rabbitmq

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/resilience/circuitbreaker"
)

// ErrNilResilientPublisher означает, что исходный
// RabbitMQ publisher для resilient-обёртки не настроен.
var ErrNilResilientPublisher = errors.New(
	"rabbitmq resilient publisher is nil",
)

// resilientPublisher описывает операции RabbitMQ publisher-а,
// которые использует resilient-обёртка.
type resilientPublisher interface {
	PublishAvatarUploaded(
		ctx context.Context,
		event domain.AvatarUploadEvent,
	) error

	PublishAvatarDeleted(
		ctx context.Context,
		event domain.AvatarDeletedEvent,
	) error

	Ping(ctx context.Context) error
	Close() error
}

// ResilientPublisher добавляет Circuit Breaker
// поверх RabbitMQ publisher-а.
type ResilientPublisher struct {
	next    resilientPublisher
	breaker *circuitbreaker.Breaker
}

// NewResilientPublisher создаёт RabbitMQ publisher,
// защищённый Circuit Breaker-ом.
func NewResilientPublisher(
	next resilientPublisher,
	breaker *circuitbreaker.Breaker,
) *ResilientPublisher {
	return &ResilientPublisher{
		next:    next,
		breaker: breaker,
	}
}

// PublishAvatarUploaded публикует событие загрузки
// аватарки через Circuit Breaker.
func (p *ResilientPublisher) PublishAvatarUploaded(
	ctx context.Context,
	event domain.AvatarUploadEvent,
) error {
	return p.execute(
		ctx,
		"publish avatar uploaded",
		func() error {
			return p.next.PublishAvatarUploaded(
				ctx,
				event,
			)
		},
	)
}

// PublishAvatarDeleted публикует событие удаления
// аватарки через Circuit Breaker.
func (p *ResilientPublisher) PublishAvatarDeleted(
	ctx context.Context,
	event domain.AvatarDeletedEvent,
) error {
	return p.execute(
		ctx,
		"publish avatar deleted",
		func() error {
			return p.next.PublishAvatarDeleted(
				ctx,
				event,
			)
		},
	)
}

// Ping проверяет RabbitMQ publisher через Circuit Breaker.
func (p *ResilientPublisher) Ping(
	ctx context.Context,
) error {
	return p.execute(
		ctx,
		"ping rabbitmq publisher",
		func() error {
			return p.next.Ping(ctx)
		},
	)
}

// Close закрывает исходный publisher.
//
// Close не проходит через Circuit Breaker,
// потому что освобождение ресурсов должно выполняться
// независимо от состояния внешней зависимости.
func (p *ResilientPublisher) Close() error {
	if err := p.validate(); err != nil {
		return err
	}

	if err := p.next.Close(); err != nil {
		return fmt.Errorf(
			"close rabbitmq publisher: %w",
			err,
		)
	}

	return nil
}

// execute выполняет операцию RabbitMQ
// через Circuit Breaker.
func (p *ResilientPublisher) execute(
	ctx context.Context,
	operationName string,
	operation func() error,
) error {
	if err := p.validate(); err != nil {
		return err
	}

	err := p.breaker.Execute(ctx, operation)
	if err != nil {
		return fmt.Errorf(
			"%s through circuit breaker: %w",
			operationName,
			err,
		)
	}

	return nil
}

// validate проверяет наличие исходного publisher-а.
func (p *ResilientPublisher) validate() error {
	if p == nil || p.next == nil {
		return ErrNilResilientPublisher
	}

	return nil
}
