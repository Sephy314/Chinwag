package domain

import (
	"time"

	"github.com/google/uuid"
)

type RoomMember struct {
	RoomId   uuid.UUID  `db:"room_id"`
	UserId   uuid.UUID  `db:"user_id"`
	Role     Role       `db:"role"`
	JoinedAt time.Time  `db:"joined_at"`
	LeftAt   *time.Time `db:"left_at"`
}

type Role int

const (
	MEMBER Role = iota
	ADMIN
)
