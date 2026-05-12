package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

const outboxEventColumns = `
	id::text,
	event_type,
	payload,
	status,
	attempts,
	COALESCE(last_error, ''),
	available_at,
	published_at,
	created_at,
	updated_at
`

type pgxRow interface {
	Scan(dest ...any) error
}

const defaultOutboxPendingLimit = 100

// OutboxRepository работает с таблицей outbox_events.
type OutboxRepository struct {
	db *pgxpool.Pool
}

// NewOutboxRepository создаёт repository для работы с outbox_events.
func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

// Create создаёт outbox-событие.
func (r *OutboxRepository) Create(ctx context.Context, event domain.OutboxEvent) (domain.OutboxEvent, error) {
	event = prepareOutboxEventForCreate(event)

	if !event.EventType.IsValid() {
		return domain.OutboxEvent{}, domain.ErrInvalidOutboxEventType
	}

	if !event.Status.IsValid() {
		return domain.OutboxEvent{}, domain.ErrInvalidOutboxEventStatus
	}

	if !json.Valid(event.Payload) || len(event.Payload) == 0 {
		return domain.OutboxEvent{}, domain.ErrInvalidOutboxPayload
	}

	query := `
		INSERT INTO outbox_events (
			event_type,
			payload,
			status,
			available_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + outboxEventColumns

	createdEvent, err := scanOutboxEvent(r.db.QueryRow(
		ctx,
		query,
		string(event.EventType),
		event.Payload,
		string(event.Status),
		event.AvailableAt,
	))
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("create outbox event: %w", err)
	}

	return createdEvent, nil
}

// ListPending возвращает pending outbox-события, которые готовы к публикации.
func (r *OutboxRepository) ListPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = defaultOutboxPendingLimit
	}

	query := `
		SELECT ` + outboxEventColumns + `
		FROM outbox_events
		WHERE status = $1
			AND available_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, string(domain.OutboxEventStatusPending), limit)
	if err != nil {
		return nil, fmt.Errorf("query pending outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.OutboxEvent, 0)

	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

// MarkPublished помечает outbox-событие опубликованным.
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ErrInvalidOutboxPayload
	}

	query := `
		UPDATE outbox_events
		SET
			status = $2,
			published_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.db.Exec(ctx, query, id, string(domain.OutboxEventStatusPublished)); err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}

	return nil
}

// MarkFailed помечает outbox-событие как pending с увеличением attempts и задержкой следующей попытки.
func (r *OutboxRepository) MarkFailed(
	ctx context.Context,
	id string,
	lastError string,
	availableAt time.Time,
) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ErrInvalidOutboxPayload
	}

	if availableAt.IsZero() {
		availableAt = time.Now().Add(time.Minute)
	}

	query := `
		UPDATE outbox_events
		SET
			status = $2,
			attempts = attempts + 1,
			last_error = $3,
			available_at = $4,
			updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.db.Exec(
		ctx,
		query,
		id,
		string(domain.OutboxEventStatusPending),
		lastError,
		availableAt,
	); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	return nil
}

// prepareOutboxEventForCreate заполняет значения outbox-события перед созданием.
func prepareOutboxEventForCreate(event domain.OutboxEvent) domain.OutboxEvent {
	if event.Status == "" {
		event.Status = domain.OutboxEventStatusPending
	}

	if event.AvailableAt.IsZero() {
		event.AvailableAt = time.Now()
	}

	return event
}

// scanOutboxEvent сканирует outbox-событие из строки PostgreSQL.
func scanOutboxEvent(row pgxRow) (domain.OutboxEvent, error) {
	var event domain.OutboxEvent
	var eventType string
	var status string
	var payload []byte

	if err := row.Scan(
		&event.ID,
		&eventType,
		&payload,
		&status,
		&event.Attempts,
		&event.LastError,
		&event.AvailableAt,
		&event.PublishedAt,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return domain.OutboxEvent{}, err
	}

	event.EventType = domain.OutboxEventType(eventType)
	event.Payload = json.RawMessage(payload)
	event.Status = domain.OutboxEventStatus(status)

	return event, nil
}
