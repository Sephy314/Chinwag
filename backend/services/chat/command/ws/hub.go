package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

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
	log        *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage, 256),
		log:        log,
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
			h.log.Info("ws: client connected", "room_id", client.roomID.String(), "user_id", client.userID.String())

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
			h.log.Info("ws: client disconnected", "room_id", client.roomID.String(), "user_id", client.userID.String())

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

func (h *Hub) HandleConnection(conn *websocket.Conn, roomID, userID uuid.UUID) {
	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		roomID: roomID,
		userID: userID,
	}
	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
