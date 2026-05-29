package tracing

import (
	"context"
	"errors"
	"testing"
)

func TestStartSpan(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "test.span")
	defer span.End()

	if ctx == nil {
		t.Fatal("expected context")
	}

	if span == nil {
		t.Fatal("expected span")
	}
}

func TestRecordError_NilError(t *testing.T) {
	_, span := StartSpan(context.Background(), "test.span")
	defer span.End()

	RecordError(span, nil)
}

func TestRecordError_WithError(t *testing.T) {
	_, span := StartSpan(context.Background(), "test.span")
	defer span.End()

	RecordError(span, errors.New("test error"))
}
