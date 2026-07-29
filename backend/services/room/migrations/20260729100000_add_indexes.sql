-- +goose Up
-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_rooms_owner_id_name
ON rooms (owner_id, name) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rooms_pop_at
ON rooms (pop_at) WHERE popped_at IS NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_room_member_room_id
ON room_member (room_id) WHERE left_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_room_member_user_id
ON room_member (user_id) WHERE left_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_rooms_owner_id_name;
DROP INDEX IF EXISTS idx_rooms_pop_at;
DROP INDEX IF EXISTS idx_room_member_room_id;
DROP INDEX IF EXISTS idx_room_member_user_id;

-- +goose StatementEnd
