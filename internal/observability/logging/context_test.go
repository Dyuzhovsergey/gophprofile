package logging

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceAttrs_WithoutSpanContext(t *testing.T) {
	attrs := TraceAttrs(context.Background())

	if len(attrs) != 0 {
		t.Fatalf("expected empty attrs, got %d", len(attrs))
	}
}

func TestTraceAttrs_WithSpanContext(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("failed to create trace id: %v", err)
	}

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("failed to create span id: %v", err)
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	attrs := TraceAttrs(ctx)

	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}

	if attrs[0].Key != "trace_id" {
		t.Fatalf("expected first attr key trace_id, got %s", attrs[0].Key)
	}

	if attrs[1].Key != "span_id" {
		t.Fatalf("expected second attr key span_id, got %s", attrs[1].Key)
	}
}

func TestAppendTraceAttrs(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("failed to create trace id: %v", err)
	}

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("failed to create span id: %v", err)
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	attrs := AppendTraceAttrs(ctx)

	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
}
