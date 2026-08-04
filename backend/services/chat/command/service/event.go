package service

import "github.com/google/uuid"

type Event struct {
	Type string
	Data any
}

type EventPublisher interface {
	Publish(roomId uuid.UUID, event Event) error
}
