-- +goose Up
ALTER TABLE signing_keys
DROP COLUMN rotated_at;

ALTER TABLE signing_keys
ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT NOW();

-- +goose Down
ALTER TABLE signing_keys
ADD COLUMN rotated_at TIMESTAMP;

ALTER TABLE signing_keys
DROP COLUMN updated_at;
