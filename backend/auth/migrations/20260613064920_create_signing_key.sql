-- +goose Up
CREATE TABLE signing_keys (
    kid TEXT PRIMARY KEY,
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,
    active BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    rotated_at TIMESTAMP
);

-- +goose Down
DROP TABLE signing_keys;