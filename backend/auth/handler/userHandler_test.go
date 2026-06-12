package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sephy314/chinwag/auth/handler"
	mocked "github.com/Sephy314/chinwag/auth/mock"
	"github.com/Sephy314/chinwag/auth/services"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/labstack/echo/v5"
)

func NewTestUserHandler() handler.UserHandler {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}

	svc := services.NewUserService(mockedCache, mockedRepo)
	hdl := handler.NewUserHandler(*svc)

	return *hdl
}

func TestUserHandler_CreateUser(t *testing.T) {
	hdl := NewTestUserHandler()

	req := structs.CreateUserReq{
		Username: "testUser",
		Email:    "test@test.com",
		Password: "test!1234",
	}

	marshaled, _ := json.Marshal(req)

	mockedRequest := httptest.NewRequest(
		http.MethodPost,
		"/user",
		bytes.NewBuffer(marshaled),
	)

	mockedResponse := httptest.NewRecorder()

	e := echo.New()

	c := e.NewContext(mockedRequest, mockedResponse)

	err := hdl.CreateUser(c)
	if err != nil {
		t.Fatal(err)
	}

}
