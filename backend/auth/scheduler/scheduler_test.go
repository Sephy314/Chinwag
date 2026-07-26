package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/shared/logger"
	"github.com/stretchr/testify/assert"
)

type mockRotator struct {
	returnErr error
	calls     atomic.Int32
}

func (m *mockRotator) Rotate(_ context.Context) error {
	m.calls.Add(1)
	return m.returnErr
}

func TestKeyRotationScheduler_CallsRotate(t *testing.T) {
	mr := &mockRotator{}
	s := NewKeyRotationScheduler(mr, 10*time.Millisecond, logger.New())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.Start(ctx)

	assert.GreaterOrEqual(t, int(mr.calls.Load()), 1)
}

func TestKeyRotationScheduler_ErrorDoesNotPanic(t *testing.T) {
	mr := &mockRotator{returnErr: errors.New("rotate failed")}
	s := NewKeyRotationScheduler(mr, 10*time.Millisecond, logger.New())

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() {
		s.Start(ctx)
	})
	assert.GreaterOrEqual(t, int(mr.calls.Load()), 1)
}

func TestKeyRotationScheduler_Stop(t *testing.T) {
	mr := &mockRotator{}
	s := NewKeyRotationScheduler(mr, 10*time.Millisecond, logger.New())

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

func TestKeyRotationScheduler_ContextCancel(t *testing.T) {
	mr := &mockRotator{}
	s := NewKeyRotationScheduler(mr, 10*time.Millisecond, logger.New())

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

func TestKeyRotationScheduler_MockRotate(t *testing.T) {
	mr := &mockRotator{}

	err := mr.Rotate(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int32(1), mr.calls.Load())
}

func TestKeyRotationScheduler_MockRotate_Error(t *testing.T) {
	mr := &mockRotator{returnErr: errors.New("rotation failed")}

	err := mr.Rotate(context.Background())
	assert.Error(t, err)
	assert.Equal(t, int32(1), mr.calls.Load())
}
