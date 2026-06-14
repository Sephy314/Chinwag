-- +goose Up
ALTER TABLE users
ADD COLUMN updated_at
TIMESTAMP DEFAULT NOW();

-- +goose Down
ALTER TABLE users
DROP COLUMN updated_at;
