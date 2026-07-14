package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type stubTransaction struct {
	roomRepo       repo.RoomRepoInterface
	roomMemberRepo repo.RoomMemberRepoInterface
}

func (t *stubTransaction) RoomRepo() repo.RoomRepoInterface {
	return t.roomRepo
}

func (t *stubTransaction) RoomMemberRepo() repo.RoomMemberRepoInterface {
	return t.roomMemberRepo
}

type stubUnitOfWork struct {
	tx     repo.Transaction
	called bool
	err    error
}

func (u *stubUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx repo.Transaction) error) error {
	u.called = true
	if u.err != nil {
		return u.err
	}
	return fn(ctx, u.tx)
}

func TestCreateRoom_UsesUnitOfWork(t *testing.T) {
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)

	req := structs.CreateRoomRequest{
		Name:        "Transactional Room",
		Description: nil,
		MaxMembers:  8,
	}

	roomRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.MaxMembers == req.MaxMembers && room.OwnerId == ownerId
	})).Return(nil)

	memberRepo.On("AddMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.RoomId != uuid.Nil && member.UserId == ownerId && member.Role == domain.ADMIN
	})).Return(nil)

	uow := &stubUnitOfWork{
		tx: &stubTransaction{
			roomRepo:       roomRepo,
			roomMemberRepo: memberRepo,
		},
	}

	service := NewRoomService(new(MockRoomRepo), uow)
	result, err := service.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, uow.called)
	roomRepo.AssertExpectations(t)
	memberRepo.AssertExpectations(t)
}

func TestInviteUser_UsesUnitOfWork(t *testing.T) {
	ctx := context.Background()
	memberRepo := new(MockRoomMemberRepo)

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   &role,
	}

	memberRepo.On("AddMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.UserId == userId && member.RoomId == roomId && member.Role == role
	})).Return(nil)

	uow := &stubUnitOfWork{
		tx: &stubTransaction{
			roomMemberRepo: memberRepo,
		},
	}

	service := NewRoomMemberService(new(MockRoomMemberRepo), uow)
	err := service.InviteUser(ctx, req)

	assert.NoError(t, err)
	assert.True(t, uow.called)
	memberRepo.AssertExpectations(t)
}

func TestCreateRoom_TransactionErrorIsReturned(t *testing.T) {
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	uowErr := errors.New("tx failed")

	service := NewRoomService(new(MockRoomRepo), &stubUnitOfWork{err: uowErr})
	room, err := service.CreateRoom(ctx, structs.CreateRoomRequest{
		Name:       "Broken",
		MaxMembers: 4,
	})

	assert.Error(t, err)
	assert.Equal(t, uowErr, err)
	assert.NotNil(t, room)
}
