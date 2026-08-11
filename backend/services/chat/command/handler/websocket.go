package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/command/ws"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// wsTicketTTL bounds how long an issued WS ticket remains valid. Tickets are
// short-lived and single-use so the WebSocket handshake never carries the
// durable access token in the URL.
const wsTicketTTL = 30 * time.Second

const consumeWsTicketScript = `
local v = redis.call('GET', KEYS[1])
if not v then
  return ''
end
redis.call('DEL', KEYS[1])
return v
`

type wsTicketPayload struct {
	UserID string `json:"user_id"`
	RoomID string `json:"room_id"`
}

type WebSocketHandler struct {
	hub *ws.Hub
	rds *redis.Client
	log *slog.Logger
}

func NewWebSocketHandler(hub *ws.Hub, rds *redis.Client, log *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		rds: rds,
		log: log,
	}
}

// IssueWsTicket mints a short-lived, single-use ticket bound to the current
// authenticated user and the target room. It is protected by the DPoP-aware
// auth middleware, and the resulting ticket is consumed atomically at the
// WebSocket upgrade so it cannot be replayed.
func (h *WebSocketHandler) IssueWsTicket(c *echo.Context) error {
	ctx := c.Request().Context()

	userID, err := sharedauth.UserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	roomID, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid room id"})
	}

	ticket := uuid.Must(uuid.NewV7()).String()
	payload := wsTicketPayload{UserID: userID, RoomID: roomID.String()}
	data, err := json.Marshal(payload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to issue ticket"})
	}

	if err := h.rds.Set(ctx, "ws:ticket:"+ticket, data, wsTicketTTL).Err(); err != nil {
		h.log.Error("ws ticket issue error", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to issue ticket"})
	}

	h.log.Info("ws: ticket issued", "user_id", userID, "room_id", roomID.String())
	return c.JSON(http.StatusOK, response.OK(map[string]any{
		"ticket":     ticket,
		"expires_in": int(wsTicketTTL.Seconds()),
	}))
}

func (h *WebSocketHandler) ServeWS(c *echo.Context) error {
	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing ws ticket"})
	}

	payload, err := h.consumeWsTicket(c.Request().Context(), ticket)
	if err != nil {
		h.log.Error("ws ticket consume error", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to consume ticket"})
	}
	if payload == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired ws ticket"})
	}

	roomID, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid room id"})
	}
	if payload.RoomID != roomID.String() {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "ws ticket not valid for this room"})
	}

	uid, err := uuid.Parse(payload.UserID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.log.Error("ws upgrade error", "error", err)
		return err
	}

	h.hub.HandleConnection(conn, roomID, uid)
	h.log.Info("ws: connected", "user_id", payload.UserID, "room_id", roomID.String())
	return nil
}

func (h *WebSocketHandler) consumeWsTicket(ctx context.Context, ticket string) (*wsTicketPayload, error) {
	res, err := h.rds.Eval(ctx, consumeWsTicketScript, []string{"ws:ticket:" + ticket}).Result()
	if err != nil {
		return nil, err
	}

	s, ok := res.(string)
	if !ok || s == "" {
		return nil, nil
	}

	var p wsTicketPayload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}
