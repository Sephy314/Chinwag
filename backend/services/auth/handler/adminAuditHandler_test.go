package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminAuditHandler_ListAudit(t *testing.T) {
	auditRepo := new(MockAuditRepo)
	auditSvc := service.NewAuditService(auditRepo)
	h := NewAdminAuditHandler(auditSvc, &noopLogger{})

	events := []domain.AuditEvent{{
		Id: "e1", AdminId: "admin1", Action: "user.disable", TargetType: "user", TargetId: "u1", CreatedAt: time.Now(),
	}}
	auditRepo.On("List", mock.Anything, "", 50, "", "", "").Return(events, (*structs.CursorMeta)(nil), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.ListAudit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.AuditEvent]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "user.disable", resp.Data[0].Action)
	auditRepo.AssertExpectations(t)
}

func TestAdminAuditHandler_RecordAudit_MissingFields(t *testing.T) {
	auditRepo := new(MockAuditRepo)
	auditSvc := service.NewAuditService(auditRepo)
	h := NewAdminAuditHandler(auditSvc, &noopLogger{})

	c, rec := echotest.ContextConfig{
		Headers:  map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody: []byte(`{"admin_id":"admin1","action":"room.delete"}`),
	}.ToContextRecorder(t)

	err := h.RecordAudit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	auditRepo.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
}

func TestAdminAuditHandler_RecordAudit_Success(t *testing.T) {
	auditRepo := new(MockAuditRepo)
	auditSvc := service.NewAuditService(auditRepo)
	h := NewAdminAuditHandler(auditSvc, &noopLogger{})

	auditRepo.On("Insert", mock.Anything, mock.MatchedBy(func(ev domain.AuditEvent) bool {
		return ev.Action == "room.delete" && ev.TargetId == "room1"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:  map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody: []byte(`{"admin_id":"admin1","action":"room.delete","target_type":"room","target_id":"room1"}`),
	}.ToContextRecorder(t)

	err := h.RecordAudit(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	auditRepo.AssertExpectations(t)
}

var _ repo.AuditRepoInterface = (*MockAuditRepo)(nil)
