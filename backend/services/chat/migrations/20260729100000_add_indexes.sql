-- +goose Up
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_chat_messages_room_created;

CREATE INDEX IF NOT EXISTS idx_chat_messages_room_created_id
ON chat_messages (room_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_chat_messages_room_created_id;

CREATE INDEX IF NOT EXISTS idx_chat_messages_room_created
ON chat_messages (room_id, created_at DESC);

-- +goose StatementEnd
