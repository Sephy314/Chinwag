-- +goose Up
CREATE TABLE message_projections (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL,
    author_id UUID NOT NULL,
    author_name VARCHAR(256) NOT NULL,
    message_type SMALLINT NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_proj_room_created_id
    ON message_projections (room_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_proj_author
    ON message_projections (author_id);

CREATE INDEX idx_proj_deleted
    ON message_projections (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS message_projections;
