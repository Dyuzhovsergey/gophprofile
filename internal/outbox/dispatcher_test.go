package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
)

type fakeRepository struct {
	events  []domain.OutboxEvent
	listErr error

	markPublishedIDs []string
	markPublishedErr error

	markFailedIDs        []string
	markFailedLastErrors []string
	markFailedAvailable  []time.Time
	markFailedErr        error
}

func (r *fakeRepository) ListPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	if limit > 0 && len(r.events) > limit {
		return r.events[:limit], nil
	}

	return r.events, nil
}

func (r *fakeRepository) MarkPublished(ctx context.Context, id string) error {
	r.markPublishedIDs = append(r.markPublishedIDs, id)

	return r.markPublishedErr
}

func (r *fakeRepository) MarkFailed(
	ctx context.Context,
	id string,
	lastError string,
	availableAt time.Time,
) error {
	r.markFailedIDs = append(r.markFailedIDs, id)
	r.markFailedLastErrors = append(r.markFailedLastErrors, lastError)
	r.markFailedAvailable = append(r.markFailedAvailable, availableAt)

	return r.markFailedErr
}

type fakePublisher struct {
	uploadedEvents []domain.AvatarUploadEvent
	uploadedErr    error

	deletedEvents []domain.AvatarDeletedEvent
	deletedErr    error
}

func (p *fakePublisher) PublishAvatarUploaded(
	ctx context.Context,
	event domain.AvatarUploadEvent,
) error {
	if p.uploadedErr != nil {
		return p.uploadedErr
	}

	p.uploadedEvents = append(p.uploadedEvents, event)

	return nil
}

func (p *fakePublisher) PublishAvatarDeleted(
	ctx context.Context,
	event domain.AvatarDeletedEvent,
) error {
	if p.deletedErr != nil {
		return p.deletedErr
	}

	p.deletedEvents = append(p.deletedEvents, event)

	return nil
}

func TestDispatcher_DispatchOnce_PublishesUploadedEvent(t *testing.T) {
	payload := mustMarshalOutboxPayload(t, domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
	})

	repo := &fakeRepository{
		events: []domain.OutboxEvent{
			{
				ID:        "event-id",
				EventType: domain.OutboxEventTypeAvatarUploaded,
				Payload:   payload,
				Status:    domain.OutboxEventStatusPending,
			},
		},
	}

	publisher := &fakePublisher{}

	dispatcher := NewDispatcherWithConfig(
		repo,
		publisher,
		logger.NewNop(),
		DispatcherConfig{
			BatchSize:      10,
			RetryBaseDelay: time.Second,
			RetryMaxDelay:  time.Minute,
			PublishTimeout: time.Second,
		},
	)

	err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.uploadedEvents) != 1 {
		t.Fatalf("unexpected uploaded events count: got %d, want %d", len(publisher.uploadedEvents), 1)
	}

	if publisher.uploadedEvents[0].AvatarID != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q", publisher.uploadedEvents[0].AvatarID)
	}

	if len(repo.markPublishedIDs) != 1 {
		t.Fatalf("unexpected published count: got %d, want %d", len(repo.markPublishedIDs), 1)
	}

	if repo.markPublishedIDs[0] != "event-id" {
		t.Fatalf("unexpected published event id: got %q", repo.markPublishedIDs[0])
	}

	if len(repo.markFailedIDs) != 0 {
		t.Fatalf("did not expect failed events, got %d", len(repo.markFailedIDs))
	}
}

func TestDispatcher_DispatchOnce_PublishesDeletedEvent(t *testing.T) {
	payload := mustMarshalOutboxPayload(t, domain.AvatarDeletedEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
		ThumbnailS3Keys: map[domain.ThumbnailSize]string{
			domain.ThumbnailSize100: "thumbnails/avatar-id/100x100.jpg",
		},
	})

	repo := &fakeRepository{
		events: []domain.OutboxEvent{
			{
				ID:        "event-id",
				EventType: domain.OutboxEventTypeAvatarDeleted,
				Payload:   payload,
				Status:    domain.OutboxEventStatusPending,
			},
		},
	}

	publisher := &fakePublisher{}

	dispatcher := NewDispatcherWithConfig(
		repo,
		publisher,
		logger.NewNop(),
		DispatcherConfig{
			BatchSize:      10,
			RetryBaseDelay: time.Second,
			RetryMaxDelay:  time.Minute,
			PublishTimeout: time.Second,
		},
	)

	err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.deletedEvents) != 1 {
		t.Fatalf("unexpected deleted events count: got %d, want %d", len(publisher.deletedEvents), 1)
	}

	if publisher.deletedEvents[0].AvatarID != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q", publisher.deletedEvents[0].AvatarID)
	}

	if len(repo.markPublishedIDs) != 1 {
		t.Fatalf("unexpected published count: got %d, want %d", len(repo.markPublishedIDs), 1)
	}
}

func TestDispatcher_DispatchOnce_PublishErrorMarksFailed(t *testing.T) {
	payload := mustMarshalOutboxPayload(t, domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
	})

	repo := &fakeRepository{
		events: []domain.OutboxEvent{
			{
				ID:        "event-id",
				EventType: domain.OutboxEventTypeAvatarUploaded,
				Payload:   payload,
				Status:    domain.OutboxEventStatusPending,
				Attempts:  1,
			},
		},
	}

	publisher := &fakePublisher{
		uploadedErr: errors.New("rabbitmq is unavailable"),
	}

	dispatcher := NewDispatcherWithConfig(
		repo,
		publisher,
		logger.NewNop(),
		DispatcherConfig{
			BatchSize:      10,
			RetryBaseDelay: time.Second,
			RetryMaxDelay:  time.Minute,
			PublishTimeout: time.Second,
		},
	)
	dispatcher.now = func() time.Time {
		return time.Unix(100, 0)
	}

	err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.markPublishedIDs) != 0 {
		t.Fatalf("did not expect published events, got %d", len(repo.markPublishedIDs))
	}

	if len(repo.markFailedIDs) != 1 {
		t.Fatalf("unexpected failed count: got %d, want %d", len(repo.markFailedIDs), 1)
	}

	if repo.markFailedIDs[0] != "event-id" {
		t.Fatalf("unexpected failed event id: got %q", repo.markFailedIDs[0])
	}

	if len(repo.markFailedAvailable) != 1 {
		t.Fatal("expected next attempt time")
	}

	if !repo.markFailedAvailable[0].After(time.Unix(100, 0)) {
		t.Fatalf("expected next attempt time to be in the future")
	}
}

func TestDispatcher_DispatchOnce_ListError(t *testing.T) {
	repo := &fakeRepository{
		listErr: errors.New("db is unavailable"),
	}

	dispatcher := NewDispatcherWithConfig(
		repo,
		&fakePublisher{},
		logger.NewNop(),
		DispatcherConfig{
			BatchSize:      10,
			RetryBaseDelay: time.Second,
			RetryMaxDelay:  time.Minute,
			PublishTimeout: time.Second,
		},
	)

	err := dispatcher.DispatchOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		want     time.Duration
	}{
		{
			name:     "first attempt",
			attempts: 0,
			want:     time.Second,
		},
		{
			name:     "second attempt",
			attempts: 1,
			want:     2 * time.Second,
		},
		{
			name:     "third attempt",
			attempts: 2,
			want:     4 * time.Second,
		},
		{
			name:     "max delay",
			attempts: 10,
			want:     10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateRetryDelay(tt.attempts, time.Second, 10*time.Second)
			if got != tt.want {
				t.Fatalf("unexpected delay: got %s, want %s", got, tt.want)
			}
		})
	}
}

func mustMarshalOutboxPayload(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	return data
}
