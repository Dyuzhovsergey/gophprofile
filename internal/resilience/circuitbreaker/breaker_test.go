package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreaker_OpensAfterFailureThreshold(
	t *testing.T,
) {
	currentTime := time.Unix(1_700_000_000, 0)

	breaker := newBreaker(
		"s3",
		Config{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      time.Minute,
		},
		nil,
		func() time.Time {
			return currentTime
		},
	)

	dependencyErr := errors.New(
		"dependency unavailable",
	)

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		err := breaker.Execute(
			context.Background(),
			func() error {
				return dependencyErr
			},
		)

		if !errors.Is(err, dependencyErr) {
			t.Fatalf(
				"request %d: unexpected error: %v",
				requestNumber,
				err,
			)
		}
	}

	if breaker.State() != StateOpen {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			StateOpen,
		)
	}

	called := false

	err := breaker.Execute(
		context.Background(),
		func() error {
			called = true
			return nil
		},
	)

	if !errors.Is(err, ErrOpen) {
		t.Fatalf(
			"got error %v, want %v",
			err,
			ErrOpen,
		)
	}

	if called {
		t.Fatal(
			"operation must not be called while breaker is open",
		)
	}
}

func TestBreaker_HalfOpenSuccessClosesBreaker(
	t *testing.T,
) {
	currentTime := time.Unix(1_700_000_000, 0)

	breaker := newBreaker(
		"s3",
		Config{
			Enabled:          true,
			FailureThreshold: 1,
			OpenTimeout:      time.Minute,
		},
		nil,
		func() time.Time {
			return currentTime
		},
	)

	_ = breaker.Execute(
		context.Background(),
		func() error {
			return errors.New("failed")
		},
	)

	currentTime = currentTime.Add(time.Minute)

	err := breaker.Execute(
		context.Background(),
		func() error {
			if breaker.State() != StateHalfOpen {
				t.Fatalf(
					"got state %q during probe, want %q",
					breaker.State(),
					StateHalfOpen,
				)
			}

			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if breaker.State() != StateClosed {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			StateClosed,
		)
	}
}

func TestBreaker_HalfOpenFailureReopensBreaker(
	t *testing.T,
) {
	currentTime := time.Unix(1_700_000_000, 0)

	breaker := newBreaker(
		"rabbitmq",
		Config{
			Enabled:          true,
			FailureThreshold: 1,
			OpenTimeout:      time.Minute,
		},
		nil,
		func() time.Time {
			return currentTime
		},
	)

	_ = breaker.Execute(
		context.Background(),
		func() error {
			return errors.New("failed")
		},
	)

	currentTime = currentTime.Add(time.Minute)

	_ = breaker.Execute(
		context.Background(),
		func() error {
			return errors.New("failed again")
		},
	)

	if breaker.State() != StateOpen {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			StateOpen,
		)
	}
}

func TestBreaker_DoesNotCountContextCanceled(
	t *testing.T,
) {
	breaker := New(
		"s3",
		Config{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	_ = breaker.Execute(
		context.Background(),
		func() error {
			return context.Canceled
		},
	)

	_ = breaker.Execute(
		context.Background(),
		func() error {
			return errors.New("failed")
		},
	)

	if breaker.State() != StateClosed {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			StateClosed,
		)
	}
}

func TestBreaker_Disabled(t *testing.T) {
	breaker := New(
		"s3",
		Config{
			Enabled:          false,
			FailureThreshold: 1,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	called := 0

	for requestNumber := 0; requestNumber < 3; requestNumber++ {
		err := breaker.Execute(
			context.Background(),
			func() error {
				called++
				return errors.New("failed")
			},
		)

		if err == nil {
			t.Fatal("expected operation error")
		}
	}

	if called != 3 {
		t.Fatalf(
			"operation called %d times, want 3",
			called,
		)
	}

	if breaker.State() != StateClosed {
		t.Fatalf(
			"got state %q, want %q",
			breaker.State(),
			StateClosed,
		)
	}
}
