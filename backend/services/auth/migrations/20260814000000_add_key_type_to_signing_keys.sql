-- +goose Up
-- Separate Access-token signing keys from Refresh-token signing keys so a
-- Refresh token can never be signed with an Access key and vice versa.
-- Existing rows (previously the only key type) default to 'Access'.
ALTER TABLE signing_keys ADD COLUMN key_type TEXT NOT NULL DEFAULT 'Access';

-- +goose Down
ALTER TABLE signing_keys DROP COLUMN key_type;
