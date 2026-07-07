package s3

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Dyuzhovsergey/gophprofile/internal/resilience/circuitbreaker"
)

// ErrNilResilientStorage означает, что исходный S3-клиент
// для resilient-обёртки не настроен.
var ErrNilResilientStorage = errors.New(
	"s3 resilient storage is nil",
)

// resilientStorage описывает операции S3-клиента,
// которые защищаются Circuit Breaker-ом.
type resilientStorage interface {
	Upload(
		ctx context.Context,
		key string,
		body io.Reader,
		contentType string,
	) error

	Download(
		ctx context.Context,
		key string,
	) ([]byte, string, error)

	Delete(
		ctx context.Context,
		key string,
	) error

	Exists(
		ctx context.Context,
		key string,
	) (bool, error)

	Ping(ctx context.Context) error
}

// ResilientClient добавляет Circuit Breaker
// поверх клиента S3/MinIO.
type ResilientClient struct {
	next    resilientStorage
	breaker *circuitbreaker.Breaker
}

// downloadResult хранит результат скачивания объекта
// для передачи через generic-функцию ExecuteValue.
type downloadResult struct {
	data        []byte
	contentType string
}

// NewResilientClient создаёт S3-клиент,
// защищённый Circuit Breaker-ом.
func NewResilientClient(
	next resilientStorage,
	breaker *circuitbreaker.Breaker,
) *ResilientClient {
	return &ResilientClient{
		next:    next,
		breaker: breaker,
	}
}

// Upload загружает объект в S3 через Circuit Breaker.
func (c *ResilientClient) Upload(
	ctx context.Context,
	key string,
	body io.Reader,
	contentType string,
) error {
	if err := c.validate(); err != nil {
		return err
	}

	err := c.breaker.Execute(ctx, func() error {
		return c.next.Upload(
			ctx,
			key,
			body,
			contentType,
		)
	})
	if err != nil {
		return fmt.Errorf(
			"s3 upload through circuit breaker: %w",
			err,
		)
	}

	return nil
}

// Download скачивает объект из S3 через Circuit Breaker.
func (c *ResilientClient) Download(
	ctx context.Context,
	key string,
) ([]byte, string, error) {
	if err := c.validate(); err != nil {
		return nil, "", err
	}

	result, err := circuitbreaker.ExecuteValue(
		ctx,
		c.breaker,
		func() (downloadResult, error) {
			data, contentType, downloadErr := c.next.Download(ctx, key)
			if downloadErr != nil {
				return downloadResult{}, downloadErr
			}

			return downloadResult{
				data:        data,
				contentType: contentType,
			}, nil
		},
	)
	if err != nil {
		return nil, "", fmt.Errorf(
			"s3 download through circuit breaker: %w",
			err,
		)
	}

	return result.data, result.contentType, nil
}

// Delete удаляет объект из S3 через Circuit Breaker.
func (c *ResilientClient) Delete(
	ctx context.Context,
	key string,
) error {
	if err := c.validate(); err != nil {
		return err
	}

	err := c.breaker.Execute(ctx, func() error {
		return c.next.Delete(ctx, key)
	})
	if err != nil {
		return fmt.Errorf(
			"s3 delete through circuit breaker: %w",
			err,
		)
	}

	return nil
}

// Exists проверяет существование объекта
// в S3 через Circuit Breaker.
func (c *ResilientClient) Exists(
	ctx context.Context,
	key string,
) (bool, error) {
	if err := c.validate(); err != nil {
		return false, err
	}

	exists, err := circuitbreaker.ExecuteValue(
		ctx,
		c.breaker,
		func() (bool, error) {
			return c.next.Exists(ctx, key)
		},
	)
	if err != nil {
		return false, fmt.Errorf(
			"s3 exists through circuit breaker: %w",
			err,
		)
	}

	return exists, nil
}

// Ping проверяет доступность S3 через Circuit Breaker.
func (c *ResilientClient) Ping(
	ctx context.Context,
) error {
	if err := c.validate(); err != nil {
		return err
	}

	err := c.breaker.Execute(ctx, func() error {
		return c.next.Ping(ctx)
	})
	if err != nil {
		return fmt.Errorf(
			"s3 ping through circuit breaker: %w",
			err,
		)
	}

	return nil
}

// validate проверяет наличие исходного S3-клиента.
func (c *ResilientClient) validate() error {
	if c == nil || c.next == nil {
		return ErrNilResilientStorage
	}

	return nil
}
