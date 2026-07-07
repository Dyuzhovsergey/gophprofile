package s3

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/resilience/circuitbreaker"
)

type fakeResilientStorage struct {
	uploadCalls   int
	downloadCalls int
	deleteCalls   int
	existsCalls   int
	pingCalls     int

	uploadErr error

	downloadData        []byte
	downloadContentType string
	downloadErr         error

	deleteErr error

	exists    bool
	existsErr error

	pingErr error
}

func (f *fakeResilientStorage) Upload(
	_ context.Context,
	_ string,
	_ io.Reader,
	_ string,
) error {
	f.uploadCalls++

	return f.uploadErr
}

func (f *fakeResilientStorage) Download(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	f.downloadCalls++

	return f.downloadData,
		f.downloadContentType,
		f.downloadErr
}

func (f *fakeResilientStorage) Delete(
	_ context.Context,
	_ string,
) error {
	f.deleteCalls++

	return f.deleteErr
}

func (f *fakeResilientStorage) Exists(
	_ context.Context,
	_ string,
) (bool, error) {
	f.existsCalls++

	return f.exists, f.existsErr
}

func (f *fakeResilientStorage) Ping(
	_ context.Context,
) error {
	f.pingCalls++

	return f.pingErr
}

func TestResilientClient_OpensBreakerAfterFailures(
	t *testing.T,
) {
	dependencyErr := errors.New("s3 unavailable")

	storage := &fakeResilientStorage{
		uploadErr: dependencyErr,
	}

	breaker := circuitbreaker.New(
		"s3",
		circuitbreaker.Config{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	client := NewResilientClient(
		storage,
		breaker,
	)

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		err := client.Upload(
			context.Background(),
			"originals/avatar.jpg",
			strings.NewReader("image"),
			"image/jpeg",
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

	err := client.Upload(
		context.Background(),
		"originals/avatar.jpg",
		strings.NewReader("image"),
		"image/jpeg",
	)

	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf(
			"got error %v, want %v",
			err,
			circuitbreaker.ErrOpen,
		)
	}

	if storage.uploadCalls != 2 {
		t.Fatalf(
			"storage called %d times, want 2",
			storage.uploadCalls,
		)
	}
}

func TestResilientClient_DownloadReturnsResult(
	t *testing.T,
) {
	storage := &fakeResilientStorage{
		downloadData:        []byte("image"),
		downloadContentType: "image/jpeg",
	}

	breaker := circuitbreaker.New(
		"s3",
		circuitbreaker.Config{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      time.Minute,
		},
		nil,
	)

	client := NewResilientClient(
		storage,
		breaker,
	)

	data, contentType, err := client.Download(
		context.Background(),
		"originals/avatar.jpg",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(data) != "image" {
		t.Fatalf(
			"got data %q, want %q",
			string(data),
			"image",
		)
	}

	if contentType != "image/jpeg" {
		t.Fatalf(
			"got content type %q, want %q",
			contentType,
			"image/jpeg",
		)
	}

	if storage.downloadCalls != 1 {
		t.Fatalf(
			"storage called %d times, want 1",
			storage.downloadCalls,
		)
	}
}

func TestResilientClient_NilStorage(
	t *testing.T,
) {
	client := NewResilientClient(nil, nil)

	err := client.Ping(context.Background())

	if !errors.Is(err, ErrNilResilientStorage) {
		t.Fatalf(
			"got error %v, want %v",
			err,
			ErrNilResilientStorage,
		)
	}
}
