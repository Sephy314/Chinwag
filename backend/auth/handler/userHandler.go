package handler

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

type UserHandler struct {
	Service *service.UserService
}

func NewUserHandler(s *service.UserService) *UserHandler {
	hdl := UserHandler{
		Service: s,
	}
	return &hdl
}

// Health godoc
// @Summary      Health check
// @Description  Check the health status of the auth service
// @Tags         auth
// @Produce      json
// @Success      200 {object} response.Response[string]
// @Router       /auth/health [get]
func (h *UserHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

// CreateUser godoc
// @Summary      Register a new user
// @Description  Create a new user account with username, email, and password. Password is hashed with bcrypt before storage.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body structs.CreateUserReq true "User registration info" example({"username":"john","email":"john@example.com","password":"secret123"})
// @Success      200 {object} response.Response[structs.UserResponse] "Successfully created user"
// @Failure      400 {object} response.Response[any] "Invalid request body"
// @Failure      409 {object} response.Response[any] "User already exists"
// @Router       /auth/user [post]
func (h *UserHandler) CreateUser(c *echo.Context) error {
	var req structs.CreateUserReq

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	usr, err := h.Service.CreateUser(c.Request().Context(), req)

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(usr.ToProjection()))
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Retrieve user information. The id parameter accepts either a UUID (user ID) or an email address. If the parameter matches an email format, it queries by email; otherwise, it queries by ID.
// @Tags         auth
// @Produce      json
// @Param        id path string true "User ID (UUID) or email address" example(john@example.com)
// @Success      200 {object} response.Response[structs.UserResponse] "User found"
// @Failure      404 {object} response.Response[any] "User not found"
// @Router       /auth/user/{id} [get]
func (h *UserHandler) GetUser(c *echo.Context) error {
	var user *domain.User
	var err error

	id := c.Param("id")

	if utils.IsEmail(id) {
		user, err = h.Service.GetUserByEmail(c.Request().Context(), id)
	} else {
		user, err = h.Service.GetUser(c.Request().Context(), id)
	}

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(user.ToProjection()))
}

// DeleteUser godoc
// @Summary      Delete a user
// @Description  Delete a user account by ID
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} response.Response[any]
// @Router       /auth/user/{id} [delete]
func (h *UserHandler) DeleteUser(c *echo.Context) error {
	id := c.Param("id")
	err := h.Service.DeleteUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password. On success, returns a JWT access token in the response body and sets an HttpOnly refresh token cookie (path: /auth, maxAge: 7 days, secure, SameSite=Lax).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body structs.LoginReq true "Login credentials" example({"email":"john@example.com","password":"secret123"})
// @Success      200 {object} response.Response[map[string]interface{}] "Returns {\"token\": \"<jwt_access_token>\"}. Refresh token is set as an HttpOnly cookie named \"refresh\"."
// @Failure      400 {object} response.Response[any] "Invalid credentials"
// @Router       /auth/login [post]
func (h *UserHandler) Login(c *echo.Context) error {
	var req structs.LoginReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	tokens, err := h.Service.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    tokens.RefreshToken,
		Path:     "/auth",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	return c.JSON(http.StatusOK, response.OK(map[string]string{
		"token": tokens.AccessToken,
	}))
}

// WhoAmI godoc
// @Summary      Get current user info
// @Description  Retrieve the currently authenticated user's information. Requires a valid JWT access token in the Authorization header (Bearer token). The token's subject claim is used to look up the user.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Response[map[string]interface{}] "Returns {\"user\": {\"id\": \"...\", \"name\": \"...\", \"email\": \"...\"}}"
// @Failure      401 {object} response.Response[any] "Unauthorized - invalid or missing token"
// @Router       /auth/whoami [get]
func (h *UserHandler) WhoAmI(c *echo.Context) error {
	token, err := echo.ContextGet[*jwt.Token](c, "user")
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	uid, err := token.Claims.GetSubject()
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	i, err := h.Service.GetUser(c.Request().Context(), uid)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(map[string]interface{}{
		"user": i.ToProjection(),
	}))
}
