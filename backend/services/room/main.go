package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/conn"
	"github.com/Sephy314/chinwag/backend/services/room/handler"
	roommigrations "github.com/Sephy314/chinwag/backend/services/room/migrations"
	"github.com/Sephy314/chinwag/backend/services/room/repo"
	"github.com/Sephy314/chinwag/backend/services/room/router"
	"github.com/Sephy314/chinwag/backend/services/room/scheduler"
	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/shared/cache"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type dpopSetNXAdapter struct {
	rds *redis.Client
}

func (a dpopSetNXAdapter) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return a.rds.SetNX(ctx, key, value, ttl).Result()
}

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

type wrappedResponse struct {
	Data userResponse `json:"data"`
	Err  string       `json:"err,omitempty"`
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

	var wrapped wrappedResponse
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if wrapped.Err != "" {
		return nil, errors.New(wrapped.Err)
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

func main() {
	_ = godotenv.Load()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := LoadConfig()

	if err := roommigrations.RunAll(cfg.DBUrl, log); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	conns, err := conn.NewConnection(&conn.ConnectionConfig{
		DBUrl:         cfg.DBUrl,
		RedisAddr:     cfg.RedisAddr,
		RedisPassword: cfg.RedisPassword,
	})
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conns.DB.Close()

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userAdapter := newAuthUserAdapter(cfg.AuthServiceURL)

	roomRepo := repo.NewRoomRepo(conns.DB)
	roomMemberRepo := repo.NewRoomMemberRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	roomService := service.NewRoomService(roomRepo, unitOfWork)
	roomMemberService := service.NewRoomMemberService(roomMemberRepo, roomRepo, userAdapter, unitOfWork)
	inviteLinkService := service.NewInviteLinkService(cacheRedis, roomMemberService, userAdapter, roomRepo)

	roomHandler := handler.NewRoomHandler(roomService, roomMemberService)
	roomMemberHandler := handler.NewRoomMemberHandler(roomMemberService, roomService, userAdapter)
	inviteLinkHandler := handler.NewInviteLinkHandler(inviteLinkService)

	popScheduler := scheduler.NewPopScheduler(scheduler.NewSQLPopper(conns.DB), 1*time.Minute, log)
	go popScheduler.Start(context.Background())

	r := router.NewRouter(roomHandler, roomMemberHandler, inviteLinkHandler, log)

	r.Setup(&router.RouterConfig{
		Port:        cfg.Port,
		JWKSURL:     cfg.JWKSURL,
		FrontendURL: cfg.FrontendURL,
		DPoPStore:   dpopSetNXAdapter{rds: conns.Rds},
	})

	log.Info("room service starting", "port", cfg.Port)

	if err := r.Echo.Start("0.0.0.0:" + cfg.Port); err != nil {
		log.Error("room service failed to start", "error", err)
		os.Exit(1)
	}
}
