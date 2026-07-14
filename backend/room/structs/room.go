package structs

import (
	"github.com/Sephy314/chinwag/room/domain"
	"github.com/google/uuid"
)

type CreateRoomRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	MaxMembers  int     `json:"max_members"`
}

type RoomUser struct {
	UserId uuid.UUID    `json:"userId"`
	RoomId uuid.UUID    `json:"roomId"`
	Role   *domain.Role `json:"role,omitempty"`
}
