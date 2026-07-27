package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/monolith/auth/domain"
	"github.com/Sephy314/chinwag/backend/monolith/auth/handler"
	"github.com/Sephy314/chinwag/backend/monolith/auth/mocked"
	"github.com/Sephy314/chinwag/backend/monolith/auth/service"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJwksHandler_ServeJWKS_Success(t *testing.T) {
	e := echo.New()

	ctx := context.Background()
	mockedRepo := mocked.JwkRepo{}

	mockedRepo.
		On("GetVersion", ctx).
		Return(new(time.Now()), nil)

	mockedRepo.
		On("Load", ctx).
		Return([]domain.SigningKeyEntity{}, nil)

	mockedRepo.
		On("Count", mock.Anything).
		Return(new(int64(1)), nil)

	mockService := service.NewJwksService(&mockedRepo, logger.New())

	hdl := handler.NewJwksHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)

	err := hdl.ServeJWKS(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &result)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}
