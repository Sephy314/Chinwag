package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/auth/errs"
	"github.com/Sephy314/chinwag/auth/structs"
)

type MockCache struct {
	GetFunc    func(ctx context.Context, key string) (string, error)
	SetFunc    func(ctx context.Context, key string, value string, ttl time.Duration) error
	DeleteFunc func(ctx context.Context, key string) error
}

func (m *MockCache) TTL(_ context.Context, _ string) (time.Duration, error) {
	panic("implement me")
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	return m.GetFunc(ctx, key)
}

func (m *MockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return m.SetFunc(ctx, key, value.(string), ttl)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	return m.DeleteFunc(ctx, key)
}

func TestGetUserIdByRefreshToken(t *testing.T) {
	cache := &MockCache{
		GetFunc: func(ctx context.Context, key string) (string, error) {
			if key != "rt:token123" {
				t.Fatalf("unexpected key: %s", key)
			}

			return "user123", nil
		},
	}

	service := NewRefreshTokenService(
		cache,
		"rt:",
		time.Hour,
	)

	userId, err := service.GetUserIdByRefreshToken(
		context.Background(),
		"token123",
	)

	if err != nil {
		t.Fatal(err)
	}

	if *userId != "user123" {
		t.Fatalf("expected user123 got %s", *userId)
	}
}

func TestGetUserIdByRefreshToken_NotFound(t *testing.T) {
	cache := &MockCache{
		GetFunc: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("not found")
		},
	}

	service := NewRefreshTokenService(
		cache,
		"rt:",
		time.Hour,
	)

	_, err := service.GetUserIdByRefreshToken(
		context.Background(),
		"token123",
	)

	if !errors.Is(err, errs.ErrCacheNotFound) {
		t.Fatalf("expected ErrCacheNotFound got %v", err)
	}
}

func TestInsertRefreshToken(t *testing.T) {
	cache := &MockCache{
		SetFunc: func(
			ctx context.Context,
			key string,
			value string,
			ttl time.Duration,
		) error {

			if key != "rt:token123" {
				t.Fatalf("unexpected key %s", key)
			}

			if value != "user123" {
				t.Fatalf("unexpected value %s", value)
			}

			if ttl != time.Hour {
				t.Fatalf("unexpected ttl %v", ttl)
			}

			return nil
		},
	}

	service := NewRefreshTokenService(
		cache,
		"rt:",
		time.Hour,
	)

	err := service.InsertRefreshToken(
		context.Background(),
		structs.RefreshToken{
			RefreshToken: "token123",
			Subject:      "user123",
		},
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRefreshToken(t *testing.T) {
	cache := &MockCache{
		DeleteFunc: func(ctx context.Context, key string) error {
			if key != "rt:token123" {
				t.Fatalf("unexpected key %s", key)
			}

			return nil
		},
	}

	service := NewRefreshTokenService(
		cache,
		"rt:",
		time.Hour,
	)

	err := service.RemoveRefreshToken(
		context.Background(),
		"token123",
	)

	if err != nil {
		t.Fatal(err)
	}
}
