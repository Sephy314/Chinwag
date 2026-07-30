package ws

import (
	"encoding/json"

	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	"github.com/google/uuid"
)

type wsEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func encodeEvent(eventType string, data interface{}) ([]byte, error) {
	return json.Marshal(wsEvent{Type: eventType, Data: data})
}

type HubEventPublisher struct {
	hub *Hub
}

func NewHubEventPublisher(hub *Hub) *HubEventPublisher {
	return &HubEventPublisher{hub: hub}
}

func (p *HubEventPublisher) Publish(roomId uuid.UUID, event service.Event) error {
	data, err := encodeEvent(event.Type, event.Data)
	if err != nil {
		return err
	}
	p.hub.Broadcast(roomId, data)
	return nil
}
