-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS avatars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id VARCHAR(255) NOT NULL,

    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),

    width INTEGER NOT NULL DEFAULT 0 CHECK (width >= 0),
    height INTEGER NOT NULL DEFAULT 0 CHECK (height >= 0),

    s3_key VARCHAR(500) NOT NULL,
    thumbnail_s3_keys JSONB NOT NULL DEFAULT '{}'::jsonb,

    upload_status VARCHAR(50) NOT NULL DEFAULT 'uploading',
    processing_status VARCHAR(50) NOT NULL DEFAULT 'pending',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT avatars_upload_status_check CHECK (
        upload_status IN ('uploading', 'uploaded', 'failed')
    ),

    CONSTRAINT avatars_processing_status_check CHECK (
        processing_status IN ('pending', 'processing', 'completed', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_avatars_user_id
    ON avatars(user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_avatars_status
    ON avatars(upload_status, processing_status);

CREATE INDEX IF NOT EXISTS idx_avatars_created_at
    ON avatars(created_at);

CREATE INDEX IF NOT EXISTS idx_avatars_user_id_created_at
    ON avatars(user_id, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS avatars;

DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd