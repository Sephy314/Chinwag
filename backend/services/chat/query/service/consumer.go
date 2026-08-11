package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/repo"
	"github.com/google/uuid"
)

type wsEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type messageCreatedData struct {
	Id          string `json:"id"`
	RoomId      string `json:"room_id"`
	AuthorId    string `json:"author_id"`
	AuthorName  string `json:"author_name"`
	MessageType int16  `json:"message_type"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
}

type messageUpdatedData struct {
	Id        string `json:"id"`
	RoomId    string `json:"room_id"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

type messageDeletedData struct {
	Id     string `json:"id"`
	RoomId string `json:"room_id"`
}

type ProjectionConsumer struct {
	repo repo.ProjectionRepoInterface
	log  *slog.Logger
}

func NewProjectionConsumer(projectionRepo repo.ProjectionRepoInterface, log *slog.Logger) *ProjectionConsumer {
	return &ProjectionConsumer{
		repo: projectionRepo,
		log:  log,
	}
}

func (c *ProjectionConsumer) Handle(roomId uuid.UUID, data []byte) {
	var ev wsEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		c.log.Error("failed to unmarshal event", "error", err)
		return
	}

	c.log.Debug("nats: handling event",
		"room_id", roomId.String(),
		"event_type", ev.Type,
	)

	switch ev.Type {
	case "new_message":
		c.handleCreated(ev.Data)
	case "updated_message":
		c.handleUpdated(ev.Data)
	case "deleted_message":
		c.handleDeleted(ev.Data)
	default:
		c.log.Warn("unknown event type", "type", ev.Type)
	}
}

func (c *ProjectionConsumer) handleCreated(data json.RawMessage) {
	var d messageCreatedData
	if err := json.Unmarshal(data, &d); err != nil {
		c.log.Error("failed to unmarshal created data", "error", err)
		return
	}

	createdAt, err := time.Parse(time.RFC3339Nano, d.CreatedAt)
	if err != nil {
		c.log.Error("failed to parse created_at", "error", err)
		return
	}

	msgId, _ := uuid.Parse(d.Id)
	roomId, _ := uuid.Parse(d.RoomId)
	authorId, _ := uuid.Parse(d.AuthorId)

	projection := domain.MessageProjection{
		Id:          msgId,
		RoomId:      roomId,
		AuthorId:    authorId,
		AuthorName:  d.AuthorName,
		MessageType: d.MessageType,
		Content:     d.Content,
		CreatedAt:   createdAt,
	}

	if err := c.repo.Upsert(context.Background(), projection); err != nil {
		c.log.Error("failed to upsert projection", "id", d.Id, "error", err)
		return
	}
	c.log.Debug("nats: projection upserted", "id", d.Id, "room_id", d.RoomId)
}

func (c *ProjectionConsumer) handleUpdated(data json.RawMessage) {
	var d messageUpdatedData
	if err := json.Unmarshal(data, &d); err != nil {
		c.log.Error("failed to unmarshal updated data", "error", err)
		return
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, d.UpdatedAt)
	if err != nil {
		c.log.Error("failed to parse updated_at", "error", err)
		return
	}

	msgId, _ := uuid.Parse(d.Id)
	if err := c.repo.UpdateContent(context.Background(), msgId, d.Content, updatedAt); err != nil {
		c.log.Error("failed to update projection content", "id", d.Id, "error", err)
		return
	}
	c.log.Debug("nats: projection updated", "id", d.Id, "room_id", d.RoomId)
}

func (c *ProjectionConsumer) handleDeleted(data json.RawMessage) {
	var d messageDeletedData
	if err := json.Unmarshal(data, &d); err != nil {
		c.log.Error("failed to unmarshal deleted data", "error", err)
		return
	}

	msgId, _ := uuid.Parse(d.Id)
	if err := c.repo.SoftDelete(context.Background(), msgId, time.Now()); err != nil {
		c.log.Error("failed to soft delete projection", "id", d.Id, "error", err)
		return
	}
	c.log.Debug("nats: projection deleted", "id", d.Id, "room_id", d.RoomId)
}
