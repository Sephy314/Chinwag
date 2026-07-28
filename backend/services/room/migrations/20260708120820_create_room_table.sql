-- +goose Up
CREATE TABLE rooms (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(255),
    max_members INT NOT NULL DEFAULT 16,
    owner_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE room_member (
    room_id VARCHAR(255) NOT NULL REFERENCES rooms(id)
                         ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    role SMALLINT NOT NULL DEFAULT 0,
    -- 0 Member
    -- 1 Admin

    joined_at TIMESTAMPTZ DEFAULT now(),
    left_at TIMESTAMPTZ,

    PRIMARY KEY (room_id, user_id)
);

-- +goose Down
DROP TABLE room_member;
DROP TABLE rooms;
