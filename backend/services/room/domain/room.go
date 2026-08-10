package domain

import (
	"time"

	"github.com/google/uuid"
)

type Room struct {
	Id          uuid.UUID  `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	Description *string    `db:"description" json:"description"`
	MaxMembers  int        `db:"max_members" json:"max_members"`
	OwnerId     uuid.UUID  `db:"owner_id" json:"owner_id"`
	PopAt       *time.Time `db:"pop_at" json:"pop_at,omitempty"`
	PoppedAt    *time.Time `db:"popped_at" json:"popped_at"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at"`
}
