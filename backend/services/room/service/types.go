package service

import (
	"context"
	"time"
)

type UserInfo struct {
	Id        string
	Name      string
	Email     string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserProvider interface {
	GetUser(ctx context.Context, id string) (*UserInfo, error)
}
