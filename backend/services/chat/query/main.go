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
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/conn"
	"github.com/Sephy314/chinwag/backend/services/chat/query/handler"
	chatquerymigrations "github.com/Sephy314/chinwag/backend/services/chat/query/migrations"
	"github.com/Sephy314/chinwag/backend/services/chat/query/repo"
	"github.com/Sephy314/chinwag/backend/services/chat/query/router"
	"github.com/Sephy314/chinwag/backend/services/chat/query/service"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
)

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
			RoomId   string  `json:"room_id"`
			UserId   string  `json:"user_id"`
			Role     int     `json:"role"`
			JoinedAt string  `json:"joined_at"`
			LeftAt   *string `json:"left_at"`
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

func main() {
	_ = godotenv.Load()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := LoadConfig()

	if err := chatquerymigrations.RunAll(cfg.DBUrl, log); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	conns, err := conn.NewConnection(&conn.ConnectionConfig{
		DBUrl:    cfg.DBUrl,
		NatsURL:  cfg.NatsURL,
		NatsName: "chinwag-chat-query",
		Log:      log,
	})
	if err != nil {
		log.Error("failed to create connections", "error", err)
		os.Exit(1)
	}
	defer conns.Close()

	projectionRepo := repo.NewProjectionRepo(conns.DB)
	memberAdapter := newRoomMemberAdapter(cfg.RoomServiceURL)

	querySvc := service.NewQueryService(projectionRepo, memberAdapter)
	queryHandler := handler.NewQueryHandler(querySvc)

	if conns.Js != nil {
		consumer := service.NewProjectionConsumer(projectionRepo, log)

		stream, err := conns.Js.Stream(context.Background(), "CHAT_EVENTS")
		if err != nil {
			log.Error("failed to get stream", "error", err)
			os.Exit(1)
		}

		cons, err := stream.CreateOrUpdateConsumer(context.Background(), jetstream.ConsumerConfig{
			Name:          "chat-projection",
			Description:   "Projection consumer for CQRS query service",
			FilterSubject: "chat.room.>",
			DeliverPolicy: jetstream.DeliverNewPolicy,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3,
		})
		if err != nil {
			log.Error("failed to create consumer", "error", err)
			os.Exit(1)
		}

		cc, err := cons.Consume(func(msg jetstream.Msg) {
			if !strings.HasPrefix(msg.Subject(), "chat.room.") {
				log.Warn("unexpected subject format, skipping", "subject", msg.Subject())
				msg.Ack()
				return
			}

			roomIdStr := msg.Subject()[strings.LastIndex(msg.Subject(), ".")+1:]
			roomId, err := uuid.Parse(roomIdStr)
			if err != nil {
				log.Warn("invalid room id in subject", "subject", msg.Subject(), "error", err)
				msg.Ack()
				return
			}

			consumer.Handle(roomId, msg.Data())
			msg.Ack()
		})
		if err != nil {
			log.Error("failed to start consumer", "error", err)
			os.Exit(1)
		}
		defer cc.Stop()

		log.Info("projection consumer started")
	}

	r := router.NewRouter(queryHandler, log)
	r.Setup(&router.RouterConfig{
		Port:        cfg.Port,
		JWKSURL:     cfg.JWKSURL,
		FrontendURL: cfg.FrontendURL,
	})

	log.Info("chat query service starting", "port", cfg.Port)

	if err := r.Echo.Start("0.0.0.0:" + cfg.Port); err != nil {
		log.Error("chat query service failed to start", "error", err)
		os.Exit(1)
	}
}
