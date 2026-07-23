package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
)

func TestServeWS_MissingToken(t *testing.T) {
	h := NewHub()
	go h.Run()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
		},
	}.ToContextRecorder(t)

	err := h.ServeWS(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "missing token", body["error"])
}

func TestServeWS_InvalidToken(t *testing.T) {
	h := NewHub()
	go h.Run()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
		},
		QueryValues: map[string][]string{
			"token": {"not-a-valid-jwt"},
		},
	}.ToContextRecorder(t)

	err := h.ServeWS(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "invalid token", body["error"])
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	h := NewHub()
	go h.Run()

	roomID := uuid.New()
	userID := uuid.New()

	client := &Client{
		hub:    h,
		conn:   nil,
		send:   make(chan []byte, 256),
		roomID: roomID,
		userID: userID,
	}

	h.register <- client
	time.Sleep(10 * time.Millisecond)

	h.mu.RLock()
	assert.Contains(t, h.rooms, roomID)
	assert.Contains(t, h.rooms[roomID], client)
	assert.Len(t, h.rooms[roomID], 1)
	h.mu.RUnlock()

	h.unregister <- client
	time.Sleep(10 * time.Millisecond)

	h.mu.RLock()
	if _, ok := h.rooms[roomID]; ok {
		assert.NotContains(t, h.rooms[roomID], client)
	} else {
		// room was deleted when last client unregistered
	}
	h.mu.RUnlock()
}

func TestHub_BroadcastToRoom(t *testing.T) {
	h := NewHub()
	go h.Run()

	roomID := uuid.New()
	otherRoomID := uuid.New()

	client1 := &Client{
		hub:    h,
		conn:   nil,
		send:   make(chan []byte, 256),
		roomID: roomID,
		userID: uuid.New(),
	}
	client2 := &Client{
		hub:    h,
		conn:   nil,
		send:   make(chan []byte, 256),
		roomID: roomID,
		userID: uuid.New(),
	}
	client3 := &Client{
		hub:    h,
		conn:   nil,
		send:   make(chan []byte, 256),
		roomID: otherRoomID,
		userID: uuid.New(),
	}

	h.register <- client1
	h.register <- client2
	h.register <- client3
	time.Sleep(10 * time.Millisecond)

	msg := []byte(`{"type":"test","data":"hello"}`)
	h.Broadcast(roomID, msg)

	select {
	case received := <-client1.send:
		assert.Equal(t, msg, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client1 did not receive broadcast")
	}

	select {
	case received := <-client2.send:
		assert.Equal(t, msg, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client2 did not receive broadcast")
	}

	select {
	case <-client3.send:
		t.Fatal("client3 should not have received broadcast")
	case <-time.After(50 * time.Millisecond):
	}

	h.mu.Lock()
	assert.Len(t, h.rooms[roomID], 2)
	assert.Len(t, h.rooms[otherRoomID], 1)
	h.mu.Unlock()
}

func TestHub_StalledClientRemoved(t *testing.T) {
	h := NewHub()
	go h.Run()

	roomID := uuid.New()
	fullSend := make(chan []byte, 1)
	fullSend <- []byte("full")

	client := &Client{
		hub:    h,
		conn:   nil,
		send:   fullSend,
		roomID: roomID,
		userID: uuid.New(),
	}

	h.register <- client
	time.Sleep(10 * time.Millisecond)

	h.mu.RLock()
	assert.Contains(t, h.rooms[roomID], client)
	h.mu.RUnlock()

	h.Broadcast(roomID, []byte("overflow"))
	time.Sleep(10 * time.Millisecond)

	h.mu.RLock()
	_, exists := h.rooms[roomID][client]
	h.mu.RUnlock()
	assert.False(t, exists, "stalled client should be removed")
}

func TestHub_BroadcastAllRoomsIsolated(t *testing.T) {
	h := NewHub()
	go h.Run()

	roomA := uuid.New()
	roomB := uuid.New()

	clientA := &Client{
		hub:    h, conn: nil, send: make(chan []byte, 256),
		roomID: roomA, userID: uuid.New(),
	}
	clientB := &Client{
		hub:    h, conn: nil, send: make(chan []byte, 256),
		roomID: roomB, userID: uuid.New(),
	}

	h.register <- clientA
	h.register <- clientB
	time.Sleep(10 * time.Millisecond)

	msgA := []byte("to-room-a")
	msgB := []byte("to-room-b")

	h.Broadcast(roomA, msgA)
	h.Broadcast(roomB, msgB)

	select {
	case received := <-clientA.send:
		assert.Equal(t, msgA, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("clientA did not receive")
	}

	select {
	case received := <-clientB.send:
		assert.Equal(t, msgB, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("clientB did not receive")
	}
}



func TestNewHub_InitialState(t *testing.T) {
	h := NewHub()
	assert.NotNil(t, h.rooms)
	assert.NotNil(t, h.register)
	assert.NotNil(t, h.unregister)
	assert.NotNil(t, h.broadcast)
	assert.Empty(t, h.rooms)
}

func TestHub_BroadcastWithNoClients(t *testing.T) {
	h := NewHub()
	go h.Run()

	h.Broadcast(uuid.New(), []byte("nobody"))
	time.Sleep(10 * time.Millisecond)
}
