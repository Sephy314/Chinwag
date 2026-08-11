package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/handler"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setAdminQueryClaims(c *echo.Context) {
	claims := &sharedauth.Claims{Role: sharedauth.RoleAdmin, RegisteredClaims: jwt.RegisteredClaims{Subject: "admin1"}}
	c.Set(sharedauth.ClaimsContextKey, claims)
}

func newAdminQueryHandler(t *testing.T, svc *MockQueryService) *handler.AdminQueryHandler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return handler.NewAdminQueryHandler(svc, log)
}

func TestAdminQueryHandler_ListMessages_Success(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := newAdminQueryHandler(t, mockSvc)

	msgs := []structs.MessageResponse{{Id: uuid.New().String(), AuthorName: "alice", CreatedAt: time.Now()}}
	mockSvc.On("AdminListMessages", mock.Anything, mock.MatchedBy(func(req structs.AdminListMessagesRequest) bool {
		return req.Search == "chat"
	})).Return(msgs, (*structs.CursorMeta)(nil), nil).Once()

	c, rec := echotest.ContextConfig{
		QueryValues: map[string][]string{"q": {"chat"}},
	}.ToContextRecorder(t)
	setAdminQueryClaims(c)

	err := h.ListMessages(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.MessageResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	mockSvc.AssertExpectations(t)
}

func TestAdminQueryHandler_GetMessage_Success(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := newAdminQueryHandler(t, mockSvc)

	messageId := uuid.New()
	expected := &structs.MessageResponse{Id: messageId.String(), Content: "hello"}
	mockSvc.On("AdminGetMessage", mock.Anything, messageId).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "messageId", Value: messageId.String()}},
	}.ToContextRecorder(t)
	setAdminQueryClaims(c)

	err := h.GetMessage(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockSvc.AssertExpectations(t)
}

func TestAdminQueryHandler_StatsMessages_Success(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := newAdminQueryHandler(t, mockSvc)

	mockSvc.On("AdminCountMessages", mock.Anything).Return(int64(33), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	setAdminQueryClaims(c)

	err := h.StatsMessages(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]int64]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(33), resp.Data["total_messages"])
	mockSvc.AssertExpectations(t)
}
