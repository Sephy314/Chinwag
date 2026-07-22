package bridge

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
}

type UserServiceGetter interface {
	GetUser(ctx context.Context, id string) (*UserInfo, error)
}

type UserAdapter struct {
	getUser     func(ctx context.Context, id string) (*UserInfo, error)
	userService UserServiceGetter
}

func NewUserAdapter(getUser func(ctx context.Context, id string) (*UserInfo, error)) *UserAdapter {
	return &UserAdapter{getUser: getUser}
}

func (a *UserAdapter) SetUserService(getter UserServiceGetter) {
	a.userService = getter
}

func (a *UserAdapter) GetUser(ctx context.Context, id string) (*UserInfo, error) {
	if a.userService != nil {
		return a.userService.GetUser(ctx, id)
	}
	return a.getUser(ctx, id)
}

type RoomMemberAdapter struct {
	getRoomsByUserId  func(ctx context.Context, userId string) ([]RoomInfo, error)
	getMembersByRoomId func(ctx context.Context, roomId string) ([]RoomMemberInfo, error)
}

func NewRoomMemberAdapter(
	getRoomsByUserId func(ctx context.Context, userId string) ([]RoomInfo, error),
	getMembersByRoomId func(ctx context.Context, roomId string) ([]RoomMemberInfo, error),
) *RoomMemberAdapter {
	return &RoomMemberAdapter{
		getRoomsByUserId:  getRoomsByUserId,
		getMembersByRoomId: getMembersByRoomId,
	}
}

func (a *RoomMemberAdapter) GetRoomsByUserId(ctx context.Context, userId string) ([]RoomInfo, error) {
	return a.getRoomsByUserId(ctx, userId)
}

func (a *RoomMemberAdapter) GetMembersByRoomId(ctx context.Context, roomId string) ([]RoomMemberInfo, error) {
	return a.getMembersByRoomId(ctx, roomId)
}
