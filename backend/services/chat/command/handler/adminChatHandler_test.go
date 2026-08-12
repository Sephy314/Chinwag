package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/chat/command/handler"
	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setAdminChatClaims(c *echo.Context) {
	claims := &sharedauth.Claims{Role: sharedauth.RoleAdmin, RegisteredClaims: jwt.RegisteredClaims{Subject: "admin1"}}
	c.Set(sharedauth.ClaimsContextKey, claims)
}

func newAdminChatHandler(t *testing.T, svc *MockChatService) *handler.AdminChatHandler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	audit := service.NewAuditClient("", "", "", "", log)
	return handler.NewAdminChatHandler(svc, audit, log)
}

func TestAdminChatHandler_DeleteMessage_Success(t *testing.T) {
	mockSvc := new(MockChatService)
	h := newAdminChatHandler(t, mockSvc)

	messageId := uuid.New()
	mockSvc.On("AdminDeleteMessage", mock.Anything, messageId).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "messageId", Value: messageId.String()}},
	}.ToContextRecorder(t)
	setAdminChatClaims(c)

	err := h.DeleteMessage(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockSvc.AssertExpectations(t)
}

func TestAdminChatHandler_DeleteMessage_InvalidID(t *testing.T) {
	mockSvc := new(MockChatService)
	h := newAdminChatHandler(t, mockSvc)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "messageId", Value: "not-a-uuid"}},
	}.ToContextRecorder(t)
	setAdminChatClaims(c)

	err := h.DeleteMessage(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "AdminDeleteMessage", mock.Anything, mock.Anything)
}
