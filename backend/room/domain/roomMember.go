package domain

import (
	"time"

	"github.com/google/uuid"
)

type RoomMember struct {
	RoomId   uuid.UUID  `db:"room_id" json:"room_id"`
	UserId   uuid.UUID  `db:"user_id" json:"user_id"`
	Role     Role       `db:"role" json:"role"`
	JoinedAt time.Time  `db:"joined_at" json:"joined_at"`
	LeftAt   *time.Time `db:"left_at" json:"left_at"`
}

type Role int

const (
	MEMBER Role = iota
	ADMIN
)
