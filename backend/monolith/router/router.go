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
	"github.com/Sephy314/chinwag/backend/monolith/chat/service"
	"github.com/Sephy314/chinwag/backend/monolith/conn"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
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
	Data    T      `json:"data"`
	Message string `json:"message"`
}

func (a *authUserAdapter) GetUser(ctx context.Context, id string) (*service.UserInfo, error) {
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

	if wrapped.Message != "" {
		return nil, errors.New(wrapped.Message)
	}

	return &service.UserInfo{
		Id:        wrapped.Data.ID,
		Name:      wrapped.Data.Name,
		Email:     wrapped.Data.Email,
		Role:      wrapped.Data.Role,
		CreatedAt: wrapped.Data.CreatedAt,
		UpdatedAt: wrapped.Data.UpdatedAt,
	}, nil
}

type roomAdapter struct {
	roomServiceURL string
	httpClient     *http.Client
}

func newRoomAdapter(roomServiceURL string) *roomAdapter {
	return &roomAdapter{
		roomServiceURL: roomServiceURL,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *roomAdapter) GetRoomsByUserId(ctx context.Context, userId string) ([]service.RoomInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.roomServiceURL+"/users/"+userId+"/rooms", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call room service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var wrapped wrappedResponse[[]service.RoomInfo]
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if wrapped.Message != "" {
		return nil, errors.New(wrapped.Message)
	}

	return wrapped.Data, nil
}

func (a *roomAdapter) GetMembersByRoomId(ctx context.Context, roomId string) ([]service.RoomMemberInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.roomServiceURL+"/rooms/"+roomId+"/members", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call room service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Room service returns members wrapped in response envelope, need to unwrap
	var roomResp struct {
		Data []struct {
			RoomId   string     `json:"room_id"`
			UserId   string     `json:"user_id"`
			Role     int        `json:"role"`
			JoinedAt time.Time  `json:"joined_at"`
			LeftAt   *time.Time `json:"left_at"`
		} `json:"data"`
		Err string `json:"message"`
	}
	if err := json.Unmarshal(body, &roomResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if roomResp.Err != "" {
		return nil, errors.New(roomResp.Err)
	}

	result := make([]service.RoomMemberInfo, len(roomResp.Data))
	for i, m := range roomResp.Data {
		result[i] = service.RoomMemberInfo{
			RoomId:   m.RoomId,
			UserId:   m.UserId,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
			LeftAt:   m.LeftAt,
		}
	}
	return result, nil
}

func (a *roomAdapter) GetRoomById(ctx context.Context, roomId string) (*service.RoomInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.roomServiceURL+"/rooms/"+roomId, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call room service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var roomResp struct {
		Data struct {
			Id          string     `json:"id"`
			Name        string     `json:"name"`
			Description *string    `json:"description"`
			MaxMembers  int        `json:"max_members"`
			OwnerId     string     `json:"owner_id"`
			PopAt       time.Time  `json:"pop_at"`
			PoppedAt    *time.Time `json:"popped_at"`
			CreatedAt   time.Time  `json:"created_at"`
			UpdatedAt   time.Time  `json:"updated_at"`
		} `json:"data"`
		Err string `json:"message"`
	}
	if err := json.Unmarshal(body, &roomResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if roomResp.Err != "" {
		return nil, errors.New(roomResp.Err)
	}

	return &service.RoomInfo{
		Id:          roomResp.Data.Id,
		Name:        roomResp.Data.Name,
		Description: roomResp.Data.Description,
		MaxMembers:  roomResp.Data.MaxMembers,
		OwnerId:     roomResp.Data.OwnerId,
		PopAt:       roomResp.Data.PopAt,
		PoppedAt:    roomResp.Data.PoppedAt,
		CreatedAt:   roomResp.Data.CreatedAt,
		UpdatedAt:   roomResp.Data.UpdatedAt,
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

	roomServiceURL := os.Getenv("ROOM_SERVICE_URL")
	if roomServiceURL == "" {
		roomServiceURL = "http://localhost:8082"
	}

	userAdapter := newAuthUserAdapter(authServiceURL)
	roomMemberProv := newRoomAdapter(roomServiceURL)

	chatRouter.SetUpChatRouter(e, userAdapter, roomMemberProv, log)

	return e, nil
}
