package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectTextMap сохраняет текущий trace context в map, для того чтобы хранить ее в outbox_events.headers, а затем перекладывать в RabbitMQ headers.
func InjectTextMap(ctx context.Context) map[string]string {
	headers := make(map[string]string)

	otel.GetTextMapPropagator().Inject(
		ctx,
		propagation.MapCarrier(headers),
	)

	return headers
}

// ExtractTextMap восстанавливает trace context из map.
func ExtractTextMap(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(
		ctx,
		propagation.MapCarrier(headers),
	)
}
