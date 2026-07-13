package structs

import (
	"github.com/Sephy314/chinwag/room/domain"
	"github.com/google/uuid"
)

type CreateRoomRequest struct {
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	MaxMembers  int       `json:"max_members"`
	OwnerId     uuid.UUID `json:"owner_id"`
}

type RoomUser struct {
	UserId uuid.UUID
	RoomId uuid.UUID
	Role   *domain.Role
}
