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

	"github.com/Sephy314/chinwag/backend/services/chat/command/conn"
	"github.com/Sephy314/chinwag/backend/services/chat/command/handler"
	chatmigrations "github.com/Sephy314/chinwag/backend/services/chat/command/migrations"
	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/Sephy314/chinwag/backend/services/chat/command/router"
	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	"github.com/Sephy314/chinwag/backend/services/chat/command/ws"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/google/uuid"
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
	Err  string       `json:"message"`
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

type roomMemberAdapter struct {
	roomServiceURL string
	httpClient     *http.Client
}

func newRoomMemberAdapter(roomServiceURL string) *roomMemberAdapter {
	return &roomMemberAdapter{
		roomServiceURL: roomServiceURL,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *roomMemberAdapter) GetMembersByRoomId(ctx context.Context, roomId string) ([]service.RoomMemberInfo, error) {
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

func (a *roomMemberAdapter) GetRoomById(ctx context.Context, roomId string) (*service.RoomInfo, error) {
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

func main() {
	_ = godotenv.Load()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := LoadConfig()

	if err := chatmigrations.RunAll(cfg.DBUrl, log); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	conns, err := conn.NewConnection(&conn.ConnectionConfig{
		DBUrl:         cfg.DBUrl,
		RedisAddr:     cfg.RedisAddr,
		RedisPassword: cfg.RedisPassword,
		NatsURL:       cfg.NatsURL,
		Log:           log,
	})
	if err != nil {
		log.Error("failed to create connections", "error", err)
		os.Exit(1)
	}
	defer conns.Close()

	userAdapter := newAuthUserAdapter(cfg.AuthServiceURL)
	roomMemberProv := newRoomMemberAdapter(cfg.RoomServiceURL)

	chatRepoImpl := repo.NewChatRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	jwksClient := sharedauth.NewJWKSClient(cfg.JWKSURL, 5*time.Minute)
	jwksClient.SetLogger(log)

	hub := ws.NewHub(log)
	go hub.Run()

	if conns.Nats != nil {
		ctx := context.Background()
		consumerName := "chat-worker-" + uuid.New().String()
		if err := conns.Nats.Consume(ctx, consumerName, hub.Broadcast); err != nil {
			log.Error("failed to start NATS consumer", "error", err)
			os.Exit(1)
		}

		outboxRepo := repo.NewOutboxRepo(conns.DB)
		outboxPublisher := service.NewOutboxPublisher(outboxRepo, conns.Nats, log)
		go outboxPublisher.Start(ctx)

		log.Info("using NATS JetStream", "consumer", consumerName, "nats_url", cfg.NatsURL)
	} else {
		log.Info("running without NATS — outbox events will accumulate until NATS is configured")
	}

	chatSvc := service.NewChatService(chatRepoImpl, unitOfWork, userAdapter, roomMemberProv)
	chatHandler := handler.NewChatHandler(chatSvc)
	wsHandler := handler.NewWebSocketHandler(hub, conns.Rds, log)

	r := router.NewRouter(chatHandler, wsHandler, log)

	r.Setup(&router.RouterConfig{
		Port:        cfg.Port,
		JWKSURL:     cfg.JWKSURL,
		FrontendURL: cfg.FrontendURL,
		DPoPStore:   dpopSetNXAdapter{rds: conns.Rds},
	})

	log.Info("chat command service starting", "port", cfg.Port)

	if err := r.Echo.Start("0.0.0.0:" + cfg.Port); err != nil {
		log.Error("chat command service failed to start", "error", err)
		os.Exit(1)
	}
}
