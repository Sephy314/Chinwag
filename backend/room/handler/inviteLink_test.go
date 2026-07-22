package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/room/handler"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockInviteLinkService struct {
	mock.Mock
}

func (m *MockInviteLinkService) CreateInviteLink(ctx context.Context, roomId uuid.UUID, createdBy uuid.UUID, req structs.CreateInviteLinkRequest) (*structs.InviteLinkResponse, error) {
	args := m.Called(ctx, roomId, createdBy, req)

	var result *structs.InviteLinkResponse
	if args.Get(0) != nil {
		result = args.Get(0).(*structs.InviteLinkResponse)
	}

	return result, args.Error(1)
}

func (m *MockInviteLinkService) JoinByInviteLink(ctx context.Context, token string, userId uuid.UUID) error {
	args := m.Called(ctx, token, userId)
	return args.Error(0)
}

func TestCreateInviteLink_Success(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	expected := &structs.InviteLinkResponse{
		Token:     uuid.New().String(),
		RoomId:    roomID.String(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, structs.CreateInviteLinkRequest{}).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[structs.InviteLinkResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)
	assert.Equal(t, roomID.String(), resp.Data.RoomId)
	assert.NotEmpty(t, resp.Data.Token)

	mockSvc.AssertExpectations(t)
}

func TestCreateInviteLink_InvalidRoomId(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.CreateInviteLink)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateInviteLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateInviteLink_Unauthorized(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ServeWithHandler(t, h.CreateInviteLink)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateInviteLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateInviteLink_NonAdmin(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, structs.CreateInviteLinkRequest{}).
		Return(nil, &errs.AppError{Status: 403, Message: "Admin permission is required"})

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "Admin permission is required", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestCreateInviteLink_PoppedRoom(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, structs.CreateInviteLinkRequest{}).
		Return(nil, errs.ErrRoomPopped)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusGone, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "Room has been popped", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestCreateInviteLink_RoomNotFound(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, structs.CreateInviteLinkRequest{}).
		Return(nil, errors.New("sql: no rows in result set"))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	mockSvc.AssertExpectations(t)
}

func TestCreateInviteLink_CustomTTL(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	ttlHours := 48
	req := structs.CreateInviteLinkRequest{TTLHours: &ttlHours}

	expected := &structs.InviteLinkResponse{
		Token:     uuid.New().String(),
		RoomId:    roomID.String(),
	}

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, req).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"ttl_hours":48}`),
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[structs.InviteLinkResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestCreateInviteLink_SingleUse(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	singleUse := true
	req := structs.CreateInviteLinkRequest{SingleUse: &singleUse}

	expected := &structs.InviteLinkResponse{
		Token:     uuid.New().String(),
		RoomId:    roomID.String(),
	}

	mockSvc.On("CreateInviteLink", mock.Anything, roomID, userID, req).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"single_use":true}`),
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.CreateInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[structs.InviteLinkResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_Success(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_EmptyToken(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: ""},
		},
	}.ServeWithHandler(t, h.JoinByInviteLink)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "JoinByInviteLink", mock.Anything, mock.Anything, mock.Anything)
}

func TestJoinByInviteLink_Unauthorized(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ServeWithHandler(t, h.JoinByInviteLink)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockSvc.AssertNotCalled(t, "JoinByInviteLink", mock.Anything, mock.Anything, mock.Anything)
}

func TestJoinByInviteLink_ExpiredToken(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(errs.ErrInviteNotFound)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "Invite link not found", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_AlreadyMember(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(errs.ErrAlreadyMember)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "User is already a member of this room", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_PoppedRoom(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(errs.ErrRoomPopped)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusGone, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "Room has been popped", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_UserDeleted(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(errs.ErrUserDeleted)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)
	assert.Equal(t, "User not found or has been deleted", resp.Message)

	mockSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_InternalError(t *testing.T) {
	mockSvc := new(MockInviteLinkService)
	h := handler.NewInviteLinkHandler(mockSvc)

	token := uuid.New().String()
	userID := uuid.New()

	mockSvc.On("JoinByInviteLink", mock.Anything, token, userID).Return(errors.New("database connection lost"))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "token", Value: token},
		},
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.JoinByInviteLink(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp.Success)

	mockSvc.AssertExpectations(t)
}
