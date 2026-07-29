package handler

import (
	"log/slog"
	"net/http"

	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/services/chat/ws"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	hub        *ws.Hub
	jwksClient *sharedauth.JWKSClient
	log        *slog.Logger
}

func NewWebSocketHandler(hub *ws.Hub, jwksClient *sharedauth.JWKSClient, log *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub:        hub,
		jwksClient: jwksClient,
		log:        log,
	}
}

func (h *WebSocketHandler) ServeWS(c *echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token"})
	}

	claims := &sharedauth.Claims{}
	parsedToken, err := h.jwksClient.ParseToken(token, claims)
	if err != nil || !parsedToken.Valid {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}

	userID := claims.Subject
	uid, err := uuid.Parse(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	roomIdStr := c.Param("roomId")
	roomId, err := uuid.Parse(roomIdStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid room id"})
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.log.Error("ws upgrade error", "error", err)
		return err
	}

	h.hub.HandleConnection(conn, roomId, uid)
	return nil
}
