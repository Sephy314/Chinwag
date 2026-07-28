package service

import (
	"context"
	"time"
)

type UserInfo struct {
	Id        string
	Name      string
	Email     string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserProvider interface {
	GetUser(ctx context.Context, id string) (*UserInfo, error)
}

type RoomInfo struct {
	Id          string
	Name        string
	Description *string
	MaxMembers  int
	OwnerId     string
	PopAt       time.Time
	PoppedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RoomMemberInfo struct {
	RoomId   string
	UserId   string
	Role     int
	JoinedAt time.Time
	LeftAt   *time.Time
}

type RoomMemberProvider interface {
	GetRoomsByUserId(ctx context.Context, userId string) ([]RoomInfo, error)
	GetMembersByRoomId(ctx context.Context, roomId string) ([]RoomMemberInfo, error)
	GetRoomById(ctx context.Context, roomId string) (*RoomInfo, error)
}
