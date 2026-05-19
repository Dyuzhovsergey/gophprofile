package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// StartSpan создаёт новый span с общим instrumentation name проекта.
func StartSpan(
	ctx context.Context,
	name string,
	attrs ...attribute.KeyValue,
) (context.Context, oteltrace.Span) {
	return otel.Tracer(InstrumentationName).Start(
		ctx,
		name,
		oteltrace.WithAttributes(attrs...),
	)
}

// RecordError записывает ошибку в span и помечает span как ошибочный.
func RecordError(span oteltrace.Span, err error) {
	if span == nil || err == nil {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
