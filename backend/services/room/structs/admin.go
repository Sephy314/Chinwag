package structs

import (
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/google/uuid"
)

// AdminCreateRoomRequest lets an admin create a room and optionally specify the
// owner. When OwnerId is omitted the acting admin becomes the owner.
type AdminCreateRoomRequest struct {
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	MaxMembers  int        `json:"max_members"`
	PopAt       *time.Time `json:"pop_at,omitempty"`
	OwnerId     *uuid.UUID `json:"owner_id,omitempty"`
}

type AdminUpdateRoomRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	MaxMembers  *int    `json:"max_members,omitempty"`
}

type ListRoomsRequest struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
	Search string `query:"q"`
}

type AdminRoomMemberRequest struct {
	UserID uuid.UUID    `json:"user_id"`
	Role   *domain.Role `json:"role,omitempty"`
}

type AdminUpdateRoomMemberRequest struct {
	Role *domain.Role `json:"role,omitempty"`
}
