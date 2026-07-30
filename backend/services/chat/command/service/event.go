package service

import "github.com/google/uuid"

type Event struct {
	Type string
	Data interface{}
}

type EventPublisher interface {
	Publish(roomId uuid.UUID, event Event) error
}
