package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

// amqpTableCarrier адаптирует RabbitMQ headers под OpenTelemetry TextMapCarrier.
type amqpTableCarrier struct {
	headers amqp.Table
}

// Get возвращает значение header-а.
func (c amqpTableCarrier) Get(key string) string {
	value, ok := c.headers[key]
	if !ok {
		return ""
	}

	switch typedValue := value.(type) {
	case string:
		return typedValue
	case []byte:
		return string(typedValue)
	default:
		return fmt.Sprint(typedValue)
	}
}

// Set записывает header.
func (c amqpTableCarrier) Set(key string, value string) {
	c.headers[key] = value
}

// Keys возвращает список header-ключей.
func (c amqpTableCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for key := range c.headers {
		keys = append(keys, key)
	}

	return keys
}

// injectTraceHeaders сохраняет текущий trace context в RabbitMQ headers.
func injectTraceHeaders(ctx context.Context) amqp.Table {
	headers := amqp.Table{}

	otel.GetTextMapPropagator().Inject(
		ctx,
		amqpTableCarrier{headers: headers},
	)

	return headers
}

// extractTraceHeaders восстанавливает trace context из RabbitMQ headers.
func extractTraceHeaders(ctx context.Context, headers amqp.Table) context.Context {
	if len(headers) == 0 {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(
		ctx,
		amqpTableCarrier{headers: headers},
	)
}
