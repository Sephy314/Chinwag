-- +goose Up
CREATE TABLE chat_messages (
    id              UUID PRIMARY KEY,

    room_id         UUID NOT NULL,
    author_id       UUID NOT NULL,

    message_type    SMALLINT NOT NULL DEFAULT 0,
    content         TEXT NOT NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_chat_messages_room_created
    ON chat_messages (room_id, created_at DESC);

CREATE INDEX idx_chat_messages_author
    ON chat_messages (author_id);

CREATE INDEX idx_chat_messages_deleted
    ON chat_messages (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS chat_messages;
