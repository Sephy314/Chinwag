package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/handler"
	mocked "github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_CreateUser(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}

	mockedRepo.
		On("CreateUser",
			mock.Anything,
			mock.MatchedBy(func(u domain.User) bool {
				return u.Name == "testUser" &&
					u.Email == "test@test.com" &&
					u.Password != "" &&
					u.Id != ""
			}),
		).
		Return(nil)

	svc := service.NewUserService(mockedCache, mockedRepo)
	hdl := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"username":"testUser","email":"test@test.com","password":"test!1234"}`),
	}.ServeWithHandler(t, hdl.CreateUser)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)

	assert.NoError(t, err)
	assert.Equal(t, "testUser", resp["name"])
	assert.Equal(t, "test@test.com", resp["email"])

	mockedRepo.AssertExpectations(t)
}
func TestUserHandler_GetUser(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}

	expected := &domain.User{
		Id:    "user-1",
		Name:  "testUser",
		Email: "test@test.com",
	}

	mockedRepo.
		On("GetUser",
			mock.Anything,
			"user-1",
		).
		Return(expected, nil)

	svc := service.NewUserService(mockedCache, mockedRepo)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "id", Value: "user-1"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "user-1", resp["id"])
	assert.Equal(t, "testUser", resp["name"])
	assert.Equal(t, "test@test.com", resp["email"])

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_DeleteUser(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}

	mockedRepo.
		On("DeleteUser",
			mock.Anything,
			"user-1",
		).
		Return(nil)

	svc := service.NewUserService(mockedCache, mockedRepo)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "id", Value: "user-1"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
	}.ServeWithHandler(t, h.DeleteUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "ok", resp)

	mockedRepo.AssertExpectations(t)
}
