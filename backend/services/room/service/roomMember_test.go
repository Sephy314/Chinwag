package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func memberSvc(roomRepo *MockRoomRepo, memberRepo *MockRoomMemberRepo) *RoomMemberService {
	return &RoomMemberService{
		repo:     memberRepo,
		roomRepo: roomRepo,
		User:     &MockUserProvider{},
	}
}

func TestRoomMemberService_InviteUser_Success_NoUow(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("AddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.RoomId == roomId && m.UserId == userId && m.Role == domain.MEMBER && m.JoinedAt.After(time.Now().Add(-time.Minute))
	})).Return(nil).Once()

	err := svc.InviteUser(context.Background(), structs.RoomUser{UserId: userId, RoomId: roomId})

	assert.NoError(t, err)
	roomRepo.AssertExpectations(t)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_InviteUser_WithRole(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	role := domain.ADMIN

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("AddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.Role == domain.ADMIN
	})).Return(nil).Once()

	err := svc.InviteUser(context.Background(), structs.RoomUser{UserId: userId, RoomId: roomId, Role: &role})

	assert.NoError(t, err)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_InviteUser_Popped(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	poppedAt := time.Now()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	err := svc.InviteUser(context.Background(), structs.RoomUser{UserId: uuid.New(), RoomId: roomId})

	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	memberRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)
}

func TestRoomMemberService_InviteUser_RoomRepoError(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errors.New("db down")).Once()

	err := svc.InviteUser(context.Background(), structs.RoomUser{UserId: uuid.New(), RoomId: roomId})

	assert.EqualError(t, err, "db down")
	memberRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)
}

func TestRoomMemberService_InviteUser_Uow(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := &RoomMemberService{
		repo:     memberRepo,
		roomRepo: roomRepo,
		User:     &MockUserProvider{},
		uow:      makeUow(roomRepo, memberRepo),
	}

	roomId := uuid.New()
	userId := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("AddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.RoomId == roomId && m.UserId == userId
	})).Return(nil).Once()

	err := svc.InviteUser(context.Background(), structs.RoomUser{UserId: userId, RoomId: roomId})

	assert.NoError(t, err)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_KickUser_Success(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("RemoveMember", mock.Anything, userId, roomId).Return(nil).Once()

	err := svc.KickUser(context.Background(), structs.RoomUser{UserId: userId, RoomId: roomId})

	assert.NoError(t, err)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_KickUser_Popped(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	err := svc.KickUser(context.Background(), structs.RoomUser{UserId: uuid.New(), RoomId: roomId})

	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	memberRepo.AssertNotCalled(t, "RemoveMember", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomMemberService_KickUser_RepoError(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("RemoveMember", mock.Anything, userId, roomId).Return(errors.New("db down")).Once()

	err := svc.KickUser(context.Background(), structs.RoomUser{UserId: userId, RoomId: roomId})

	assert.EqualError(t, err, "db down")
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetUserByRoomId_Success(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	expected := []domain.RoomMember{{RoomId: roomId, UserId: uuid.New(), Role: domain.MEMBER}}

	memberRepo.On("GetMembersByRoomId", mock.Anything, roomId).Return(expected, nil).Once()

	members, err := svc.GetUserByRoomId(context.Background(), roomId)

	assert.NoError(t, err)
	assert.Len(t, members, 1)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetUserByRoomId_Error(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	memberRepo.On("GetMembersByRoomId", mock.Anything, roomId).Return(nil, errors.New("boom")).Once()

	members, err := svc.GetUserByRoomId(context.Background(), roomId)

	assert.Nil(t, members)
	assert.EqualError(t, err, "boom")
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetRoomsByUserId(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	userId := uuid.New()
	expected := []domain.Room{{Id: uuid.New(), Name: "A"}}

	memberRepo.On("GetRoomsByUserId", mock.Anything, userId).Return(expected, nil).Once()

	rooms, err := svc.GetRoomsByUserId(context.Background(), userId)

	assert.NoError(t, err)
	assert.Len(t, rooms, 1)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetRoomById(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	expected := domain.Room{Id: roomId, Name: "General"}

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(expected, nil).Once()

	room, err := svc.GetRoomById(context.Background(), roomId)

	assert.NoError(t, err)
	assert.Equal(t, "General", room.Name)
	roomRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetUserByRoomIdAndUserId_Success(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	expected := domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.ADMIN}

	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(expected, nil).Once()

	member, err := svc.GetUserByRoomIdAndUserId(context.Background(), userId, roomId)

	assert.NoError(t, err)
	assert.NotNil(t, member)
	assert.Equal(t, domain.ADMIN, member.Role)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetUserByRoomIdAndUserId_Error(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()

	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound).Once()

	member, err := svc.GetUserByRoomIdAndUserId(context.Background(), userId, roomId)

	assert.Nil(t, member)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_UpdateRoomMember_Success_NoUow(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	existing := domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.MEMBER, JoinedAt: time.Now()}
	newRole := domain.ADMIN

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(existing, nil).Once()
	memberRepo.On("UpdateMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.Role == domain.ADMIN
	})).Return(nil).Once()

	member, err := svc.UpdateRoomMember(context.Background(), userId, roomId, structs.UpdateRoomMemberRequest{Role: &newRole})

	assert.NoError(t, err)
	assert.NotNil(t, member)
	assert.Equal(t, domain.ADMIN, member.Role)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_UpdateRoomMember_Popped(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	member, err := svc.UpdateRoomMember(context.Background(), uuid.New(), roomId, structs.UpdateRoomMemberRequest{})

	assert.Nil(t, member)
	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	memberRepo.AssertNotCalled(t, "GetMemberByRoomIdAndMemberId", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomMemberService_UpdateRoomMember_MemberNotFound(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound).Once()

	member, err := svc.UpdateRoomMember(context.Background(), userId, roomId, structs.UpdateRoomMemberRequest{})

	assert.Nil(t, member)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_UpdateRoomMember_Uow(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := &RoomMemberService{
		repo:     memberRepo,
		roomRepo: roomRepo,
		User:     &MockUserProvider{},
		uow:      makeUow(roomRepo, memberRepo),
	}

	roomId := uuid.New()
	userId := uuid.New()
	existing := domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.MEMBER, JoinedAt: time.Now()}
	newRole := domain.ADMIN

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(existing, nil).Once()
	memberRepo.On("UpdateMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.Role == domain.ADMIN
	})).Return(nil).Once()

	member, err := svc.UpdateRoomMember(context.Background(), userId, roomId, structs.UpdateRoomMemberRequest{Role: &newRole})

	assert.NoError(t, err)
	assert.NotNil(t, member)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_SetUserRole_Success(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberRepo.On("SetUserRole", mock.Anything, userId, roomId, domain.ADMIN).Return(nil).Once()

	err := svc.SetUserRole(context.Background(), userId, roomId, domain.ADMIN)

	assert.NoError(t, err)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_SetUserRole_Popped(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	err := svc.SetUserRole(context.Background(), uuid.New(), roomId, domain.ADMIN)

	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	memberRepo.AssertNotCalled(t, "SetUserRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomMemberService_GetUserRole_Admin(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(
		domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.ADMIN}, nil,
	).Once()

	role, err := svc.GetUserRole(context.Background(), userId, roomId)

	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, domain.ADMIN, *role)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_GetUserRole_Error(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound).Once()

	role, err := svc.GetUserRole(context.Background(), userId, roomId)

	assert.Nil(t, role)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_HasManagerPermission_Admin_True(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(
		domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.ADMIN}, nil,
	).Once()

	ok, err := svc.HasManagerPermission(context.Background(), userId, roomId)

	assert.NoError(t, err)
	assert.True(t, ok)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_HasManagerPermission_Member_False(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(
		domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.MEMBER}, nil,
	).Once()

	ok, err := svc.HasManagerPermission(context.Background(), userId, roomId)

	assert.NoError(t, err)
	assert.False(t, ok)
	memberRepo.AssertExpectations(t)
}

func TestRoomMemberService_HasManagerPermission_Error(t *testing.T) {
	roomRepo := new(MockRoomRepo)
	memberRepo := new(MockRoomMemberRepo)
	svc := memberSvc(roomRepo, memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound).Once()

	ok, err := svc.HasManagerPermission(context.Background(), userId, roomId)

	assert.False(t, ok)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	memberRepo.AssertExpectations(t)
}
