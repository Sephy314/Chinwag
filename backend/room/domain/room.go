package domain

import (
	"time"

	"github.com/google/uuid"
)

type Room struct {
	Id          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Description *string    `db:"description"`
	MaxMembers  int        `db:"max_members"`
	OwnerId     uuid.UUID  `db:"owner_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}
