package structs

import (
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
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

type RoomWithRole struct {
	domain.Room
	Role *domain.Role `json:"role,omitempty"`
}

type RoomMemberResponse struct {
	RoomId   string     `json:"room_id"`
	UserId   string     `json:"user_id"`
	UserName string     `json:"user_name"`
	Role     int        `json:"role"`
	JoinedAt time.Time  `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at,omitempty"`
}
