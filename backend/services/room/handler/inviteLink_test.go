package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInviteLinkHandler_CreateInviteLink_Success(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	ttlHours := 5

	expected := &structs.InviteLinkResponse{Token: token, RoomId: roomId.String(), ExpiresAt: time.Now().Add(5 * time.Hour)}
	inviteSvc.On("CreateInviteLink", mock.Anything, roomId, userId, mock.MatchedBy(func(req structs.CreateInviteLinkRequest) bool {
		return req.TTLHours != nil && *req.TTLHours == ttlHours
	})).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"ttl_hours":5}`),
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.CreateInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[structs.InviteLinkResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, token, resp.Data.Token)
	inviteSvc.AssertExpectations(t)
}

func TestInviteLinkHandler_CreateInviteLink_InvalidRoomID(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: "bad"}},
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.CreateInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	inviteSvc.AssertNotCalled(t, "CreateInviteLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInviteLinkHandler_CreateInviteLink_Unauthorized(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	c, _ := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: uuid.New().String()}},
	}.ToContextRecorder(t)

	err := h.CreateInviteLink(c)
	assert.ErrorIs(t, err, echo.ErrUnauthorized)
	inviteSvc.AssertNotCalled(t, "CreateInviteLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInviteLinkHandler_CreateInviteLink_Forbidden(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	roomId := uuid.New()
	userId := uuid.New()
	inviteSvc.On("CreateInviteLink", mock.Anything, roomId, userId, mock.Anything).Return(nil, &errs.AppError{Status: http.StatusForbidden, Message: "Admin permission is required"}).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.CreateInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	inviteSvc.AssertExpectations(t)
}

func TestInviteLinkHandler_JoinByInviteLink_Success(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	token := uuid.New().String()
	roomId := uuid.New()
	userId := uuid.New()

	inviteSvc.On("JoinByInviteLink", mock.Anything, token, userId).Return(roomId, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "token", Value: token}},
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.JoinByInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]string]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, roomId.String(), resp.Data["room_id"])
	inviteSvc.AssertExpectations(t)
}

func TestInviteLinkHandler_JoinByInviteLink_EmptyToken(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "token", Value: ""}},
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.JoinByInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	inviteSvc.AssertNotCalled(t, "JoinByInviteLink", mock.Anything, mock.Anything, mock.Anything)
}

func TestInviteLinkHandler_JoinByInviteLink_NotFound(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	token := uuid.New().String()
	userId := uuid.New()
	inviteSvc.On("JoinByInviteLink", mock.Anything, token, userId).Return(uuid.Nil, errs.ErrInviteNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "token", Value: token}},
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.JoinByInviteLink(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	inviteSvc.AssertExpectations(t)
}

func TestInviteLinkHandler_JoinByInviteLink_Unauthorized(t *testing.T) {
	inviteSvc := new(MockInviteLinkService)
	h := NewInviteLinkHandler(inviteSvc)

	c, _ := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "token", Value: uuid.New().String()}},
	}.ToContextRecorder(t)

	err := h.JoinByInviteLink(c)
	assert.ErrorIs(t, err, echo.ErrUnauthorized)
	inviteSvc.AssertNotCalled(t, "JoinByInviteLink", mock.Anything, mock.Anything, mock.Anything)
}
