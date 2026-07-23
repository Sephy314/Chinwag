package structs

import (
	"time"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/google/uuid"
)

type CreateRoomRequest struct {
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	MaxMembers  int        `json:"max_members"`
	PopAt       *time.Time `json:"pop_at,omitempty"`
}

type UpdateRoomRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	MaxMembers  *int    `json:"max_members,omitempty"`
}

type UpdateRoomMemberRequest struct {
	Role *domain.Role `json:"role,omitempty"`
}

type AddRoomMemberRequest struct {
	UserID uuid.UUID    `json:"user_id"`
	Role   *domain.Role `json:"role,omitempty"`
}

type RoomUser struct {
	UserId uuid.UUID    `json:"userId"`
	RoomId uuid.UUID    `json:"roomId"`
	Role   *domain.Role `json:"role,omitempty"`
}
