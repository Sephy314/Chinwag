-- +goose Up
ALTER TABLE rooms
ADD pop_at
    TIMESTAMPTZ
    NOT NULL
    DEFAULT NOW() + INTERVAL '1 day';

ALTER TABLE rooms
ADD popped_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE rooms
    DROP pop_at;
ALTER TABLE rooms
    DROP popped_at;