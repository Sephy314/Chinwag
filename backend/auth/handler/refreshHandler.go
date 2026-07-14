package handler

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RefreshHandler interface {
	Refresh(c *echo.Context) error
}

type RefreshHandlerImpl struct {
	service    service.RefreshTokenServiceInterface
	jwtService service.JwtServiceInterface
}

func NewRefreshHandler(service service.RefreshTokenServiceInterface, jwtService service.JwtServiceInterface) *RefreshHandlerImpl {
	return &RefreshHandlerImpl{
		service:    service,
		jwtService: jwtService,
	}
}

// Refresh godoc
// @Summary      Refresh access token
// @Description  Issue a new JWT access token using the refresh token stored in the HttpOnly cookie named "refresh". The old refresh token is invalidated and a new refresh token cookie is set (path: /auth, maxAge: 7 days, secure, SameSite=Lax). Returns the new access token in the response body.
// @Tags         auth
// @Produce      json
// @Success      200 {object} structs.LoginUserResp "Returns {\"token\": \"<new_jwt_access_token>\"}. New refresh token is set as an HttpOnly cookie."
// @Failure      400 {object} map[string]interface{} "Missing or invalid refresh token cookie"
// @Failure      404 {object} map[string]string "Refresh token not found (expired or revoked)"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /auth/refresh [post]
func (h *RefreshHandlerImpl) Refresh(c *echo.Context) error {
	//var req *structs.RefreshRequest
	//
	//if err := c.Bind(&req); err != nil {
	//	return c.JSON(http.StatusBadRequest, map[string]interface{}{})
	//}

	ctx := c.Request().Context()

	cookie, err := c.Cookie("refresh")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{})
	}

	uid, err := h.service.GetUserIdByRefreshToken(ctx, cookie.Value)

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	token, err := h.jwtService.NewAccessToken(ctx, *uid, domain.USER)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()

	err = h.service.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      *uid,
		RefreshToken: refreshToken,
	})

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    refreshToken,
		Path:     "/auth",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	return c.JSON(http.StatusOK, structs.LoginUserResp{
		Token: *token,
	})
}
