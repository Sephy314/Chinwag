-- +goose Up
DROP INDEX IF EXISTS idx_outbox_pending;
CREATE INDEX idx_outbox_pending
    ON outbox_events (created_at ASC)
    WHERE published_at IS NULL AND retry_count < 20;

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_pending;
CREATE INDEX idx_outbox_pending
    ON outbox_events (created_at ASC)
    WHERE published_at IS NULL;
