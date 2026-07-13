-- +goose Up
ALTER TABLE users
ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'USER';

-- +goose Down
ALTER TABLE users
DROP COLUMN role;
