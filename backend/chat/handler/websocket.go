package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Sephy314/chinwag/shared/utils"
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

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	roomID uuid.UUID
	userID uuid.UUID
}

type BroadcastMessage struct {
	RoomID  uuid.UUID
	Message []byte
}

type Hub struct {
	rooms      map[uuid.UUID]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.rooms[client.roomID] == nil {
				h.rooms[client.roomID] = make(map[*Client]bool)
			}
			h.rooms[client.roomID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.rooms[client.roomID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.rooms, client.roomID)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.rooms[msg.RoomID]
			h.mu.RUnlock()
			for client := range clients {
				select {
				case client.send <- msg.Message:
				default:
					h.mu.Lock()
					delete(h.rooms[client.roomID], client)
					close(client.send)
					h.mu.Unlock()
				}
			}
		}
	}
}

func (h *Hub) Broadcast(roomId uuid.UUID, message []byte) {
	h.broadcast <- BroadcastMessage{RoomID: roomId, Message: message}
}

// ServeWS godoc
// @Summary      WebSocket connection
// @Description  Upgrade to a WebSocket connection for real-time messaging in a room. Pass the JWT token as a query parameter. After upgrade, the server broadcasts new_message, updated_message, and deleted_message events. Client can send {"type":"ping"} and receive {"type":"pong"}.
// @Tags         chat
// @Produce      json
// @Param        roomId path string true "Room UUID" 
// @Param        token  query string true "JWT access token"
// @Success      101 {object} any "Switching Protocols to WebSocket"
// @Failure      400 {object} map[string]string "Invalid UUID or token format"
// @Failure      401 {object} map[string]string "Missing or invalid token"
// @Router       /chat/rooms/{roomId}/ws [get]
func (h *Hub) ServeWS(c *echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token"})
	}

	userID, err := utils.GetUserIDFromToken(token)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}
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
		log.Println("ws upgrade error:", err)
		return err
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		roomID: roomId,
		userID: uid,
	}
	h.register <- client

	go client.writePump()
	go client.readPump()

	return nil
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(message, &msg) == nil {
			if msg.Type == "ping" {
				pong, _ := json.Marshal(map[string]string{"type": "pong"})
				select {
				case c.send <- pong:
				default:
				}
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
