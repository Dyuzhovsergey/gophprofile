-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,

    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,

    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT outbox_events_status_check CHECK (
        status IN ('pending', 'published', 'failed')
    ),

    CONSTRAINT outbox_events_event_type_check CHECK (
        event_type IN ('avatar.uploaded', 'avatar.deleted')
    )
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events(available_at, created_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_events_status
    ON outbox_events(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox_events;
-- +goose StatementEnd