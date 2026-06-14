-- +goose Up
ALTER TABLE signing_keys
DROP COLUMN IF EXISTS active;

ALTER TABLE signing_keys
ADD COLUMN status TEXT NOT NULL default 'ACTIVE';

ALTER TABLE signing_keys
ADD COLUMN expired_at TIMESTAMP;


-- +goose Down
ALTER TABLE signing_keys
DROP COLUMN  status;

ALTER TABLE signing_keys
ADD COLUMN status BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE signing_keys
DROP COLUMN expired_at;