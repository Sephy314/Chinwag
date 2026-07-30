-- +goose Up
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    room_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_outbox_pending
    ON outbox_events (created_at ASC)
    WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
