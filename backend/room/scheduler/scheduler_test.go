package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockPopper struct {
	returnRows int64
	returnErr  error
	calls      atomic.Int32
}

func (m *mockPopper) PopRooms(_ context.Context) (int64, error) {
	m.calls.Add(1)
	return m.returnRows, m.returnErr
}

func TestPopScheduler_CallsPopRooms(t *testing.T) {
	mp := &mockPopper{returnRows: 3}
	s := NewPopScheduler(mp, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.Start(ctx)

	assert.GreaterOrEqual(t, int(mp.calls.Load()), 1)
}

func TestPopScheduler_ErrorDoesNotPanic(t *testing.T) {
	mp := &mockPopper{returnErr: errors.New("db down")}
	s := NewPopScheduler(mp, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() {
		s.Start(ctx)
	})
	assert.GreaterOrEqual(t, int(mp.calls.Load()), 1)
}

func TestPopScheduler_Stop(t *testing.T) {
	mp := &mockPopper{returnRows: 0}
	s := NewPopScheduler(mp, 10*time.Millisecond)

	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	s.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func TestPopScheduler_ContextCancel(t *testing.T) {
	mp := &mockPopper{returnRows: 0}
	s := NewPopScheduler(mp, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancel")
	}
}

func TestPopper_MockPopRooms(t *testing.T) {
	mp := &mockPopper{returnRows: 5, returnErr: nil}

	rows, err := mp.PopRooms(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), rows)
}

func TestPopper_MockPopRooms_Error(t *testing.T) {
	mp := &mockPopper{returnRows: 0, returnErr: errors.New("connection refused")}

	rows, err := mp.PopRooms(context.Background())
	assert.Error(t, err)
	assert.Equal(t, int64(0), rows)
}
