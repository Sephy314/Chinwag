-- +goose Up
-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_signing_keys_status 
ON signing_keys (status);

CREATE INDEX IF NOT EXISTS idx_signing_keys_updated_at 
ON signing_keys (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at 
ON users (deleted_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_signing_keys_status;
DROP INDEX IF EXISTS idx_signing_keys_updated_at;
DROP INDEX IF EXISTS idx_users_deleted_at;

-- +goose StatementEnd
