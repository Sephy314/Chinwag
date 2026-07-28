package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	chatRouter "github.com/Sephy314/chinwag/backend/monolith/chat/router"
	"github.com/Sephy314/chinwag/backend/monolith/conn"
	"github.com/Sephy314/chinwag/backend/monolith/conn/bridge"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
	roomRouter "github.com/Sephy314/chinwag/backend/monolith/room/router"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type authUserAdapter struct {
	authServiceURL string
	httpClient     *http.Client
}

func newAuthUserAdapter(authServiceURL string) *authUserAdapter {
	return &authUserAdapter{
		authServiceURL: authServiceURL,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type wrappedResponse[T any] struct {
	Data T      `json:"data"`
	Err  string `json:"err,omitempty"`
}

func (a *authUserAdapter) GetUser(ctx context.Context, id string) (*bridge.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.authServiceURL+"/user/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call auth service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var wrapped wrappedResponse[userResponse]
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if wrapped.Err != "" {
		return nil, errors.New(wrapped.Err)
	}

	return &bridge.UserInfo{
		Id:        wrapped.Data.ID,
		Name:      wrapped.Data.Name,
		Email:     wrapped.Data.Email,
		Role:      wrapped.Data.Role,
		CreatedAt: wrapped.Data.CreatedAt,
		UpdatedAt: wrapped.Data.UpdatedAt,
	}, nil
}

func SetUpRouter(log logger.Logger) (*echo.Echo, error) {
	conns, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	e := echo.New()

	if e == nil {
		return nil, errors.New("no echo object")
	}

	e.HTTPErrorHandler = appMiddleware.GlobalErrorHandler(log)

	e.Use(middleware.RequestID())
	e.Use(appMiddleware.RequestIDInjector())
	e.Use(appMiddleware.ResponseIDInjector())
	e.Use(middleware.RequestLogger())

	// Global rate limiter: 100 requests per minute per IP
	globalStore := appMiddleware.NewRedisSlidingWindowStore(conns.Rds, 100, time.Minute)
	e.Use(appMiddleware.NewRateLimitMiddleware(globalStore, appMiddleware.IPExtractor))

	e.Use(middleware.Recover())

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"http://localhost:3000",
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},

		AllowCredentials: true,
	}))

	SetUpSwaggerRoutes(e)

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081"
	}

	userAdapter := newAuthUserAdapter(authServiceURL)

	roomMemberProv := roomRouter.SetUpRoomRouter(e, userAdapter, log)

	chatRouter.SetUpChatRouter(e, userAdapter, roomMemberProv, log)

	return e, nil
}
