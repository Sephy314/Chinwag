package structs

import "time"

type CreateInviteLinkRequest struct {
	TTLHours  *int  `json:"ttl_hours,omitempty"`
	SingleUse *bool `json:"single_use,omitempty"`
}

type InviteLinkResponse struct {
	Token     string    `json:"token"`
	RoomId    string    `json:"room_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
