-- +goose Up
ALTER TABLE users
ADD COLUMN provider VARCHAR(50) DEFAULT 'local',
ADD COLUMN provider_id VARCHAR(255);

CREATE INDEX idx_users_provider_email ON users (provider, email);

-- +goose Down
DROP INDEX IF EXISTS idx_users_provider_email;
ALTER TABLE users
DROP COLUMN IF EXISTS provider,
DROP COLUMN IF EXISTS provider_id;
