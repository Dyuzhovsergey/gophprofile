package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/resilience/circuitbreaker"
)

type fakeResilientPublisher struct {
	uploadedCalls int
	deletedCalls  int
	pingCalls     int
	closeCalls    int

	uploadedErr error
	deletedErr  error
	pingErr     error
	closeErr    error
}

func (f *fakeResilientPublisher) PublishAvatarUploaded(
	_ context.Context,
	_ domain.AvatarUploadEvent,
) error {
	f.uploadedCalls++

	return f.uploadedErr
}

func (f *fakeResilientPublisher) PublishAvatarDeleted(
	_ context.Context,
	_ domain.AvatarDeletedEvent,
) error {
	f.deletedCalls++

	return f.deletedErr
}

func (f *fakeResilientPublisher) Ping(
	_ context.Context,
) error {
	f.pingCalls++

	return f.pingErr
}

func (f *fakeResilientPublisher) Close() error {
	f.closeCalls++

	return f.closeErr
}

func TestResilientPublisher_OpensBreakerAfterFailures(
	t *testing.T,
) {
	dependencyErr := errors.New(
		"rabbitmq unavailable",
	)

	publisher := &fakeResilientPublisher{
		uploadedErr: dependencyErr,
	}

	breaker := circuitbreaker.New(
		"rabbitmq_publisher",
		circuitbreaker.Config{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	resilientPublisher := NewResilientPublisher(
		publisher,
		breaker,
	)

	event := domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar.jpg",
	}

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		err := resilientPublisher.PublishAvatarUploaded(
			context.Background(),
			event,
		)

		if !errors.Is(err, dependencyErr) {
			t.Fatalf(
				"request %d: got error %v, want %v",
				requestNumber,
				err,
				dependencyErr,
			)
		}
	}

	if breaker.State() != circuitbreaker.StateOpen {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			circuitbreaker.StateOpen,
		)
	}

	err := resilientPublisher.PublishAvatarUploaded(
		context.Background(),
		event,
	)

	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf(
			"got error %v, want %v",
			err,
			circuitbreaker.ErrOpen,
		)
	}

	if publisher.uploadedCalls != 2 {
		t.Fatalf(
			"publisher called %d times, want 2",
			publisher.uploadedCalls,
		)
	}
}

func TestResilientPublisher_CloseBypassesBreaker(
	t *testing.T,
) {
	publisher := &fakeResilientPublisher{
		uploadedErr: errors.New(
			"rabbitmq unavailable",
		),
	}

	breaker := circuitbreaker.New(
		"rabbitmq_publisher",
		circuitbreaker.Config{
			Enabled:          true,
			FailureThreshold: 1,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	resilientPublisher := NewResilientPublisher(
		publisher,
		breaker,
	)

	_ = resilientPublisher.PublishAvatarUploaded(
		context.Background(),
		domain.AvatarUploadEvent{},
	)

	if breaker.State() != circuitbreaker.StateOpen {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			circuitbreaker.StateOpen,
		)
	}

	if err := resilientPublisher.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if publisher.closeCalls != 1 {
		t.Fatalf(
			"close called %d times, want 1",
			publisher.closeCalls,
		)
	}
}

func TestResilientPublisher_NilPublisher(
	t *testing.T,
) {
	publisher := NewResilientPublisher(nil, nil)

	err := publisher.Ping(context.Background())

	if !errors.Is(
		err,
		ErrNilResilientPublisher,
	) {
		t.Fatalf(
			"got error %v, want %v",
			err,
			ErrNilResilientPublisher,
		)
	}
}
