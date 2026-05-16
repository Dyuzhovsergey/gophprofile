// Package logging содержит helpers для observability-логирования.
package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceAttrs возвращает slog-атрибуты trace_id и span_id из context.Context.
func TraceAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}

	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return nil
	}

	return []slog.Attr{
		slog.String("trace_id", spanContext.TraceID().String()),
		slog.String("span_id", spanContext.SpanID().String()),
	}
}

// AppendTraceAttrs добавляет trace_id/span_id к переданным slog-атрибутам.
func AppendTraceAttrs(ctx context.Context, attrs ...slog.Attr) []slog.Attr {
	traceAttrs := TraceAttrs(ctx)
	if len(traceAttrs) == 0 {
		return attrs
	}

	result := make([]slog.Attr, 0, len(traceAttrs)+len(attrs))
	result = append(result, traceAttrs...)
	result = append(result, attrs...)

	return result
}
