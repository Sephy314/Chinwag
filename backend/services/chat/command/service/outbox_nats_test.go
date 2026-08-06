package service

import (
	"context"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestOutboxPublisher_PublishFailure_NATSUnavailable verifies that when NATS is
// down (connection closed), the event is NOT marked published, the retry count
// is incremented, and the event enters backoff so it is not hammered.
func TestOutboxPublisher_PublishFailure_NATSUnavailable(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger()

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)
	mockNats.On("PublishRaw", mock.Anything, evt.Subject, evt.Payload).Return(nats.ErrConnectionClosed)
	mockOutbox.On("IncrementRetry", mock.Anything, evtID).Return(nil)
	mockOutbox.AssertNotCalled(t, "MarkPublished", mock.Anything, mock.Anything)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockOutbox.AssertExpectations(t)
	mockNats.AssertExpectations(t)

	// The failed event must now be in backoff.
	assert.True(t, publisher.inBackoff(evtID), "event should enter backoff after a NATS failure")
}

// TestOutboxPublisher_EventInBackoff_IsSkipped verifies that while an event is
// in backoff (e.g. NATS still down), a subsequent batch does NOT attempt to
// republish it.
func TestOutboxPublisher_EventInBackoff_IsSkipped(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger()

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)
	publisher.setBackoff(evtID, 0) // 100ms backoff — far in the future for this run

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockNats.AssertNotCalled(t, "PublishRaw", mock.Anything, mock.Anything, mock.Anything)
	mockOutbox.AssertNotCalled(t, "IncrementRetry", mock.Anything, mock.Anything)
	mockOutbox.AssertNotCalled(t, "MarkPublished", mock.Anything, mock.Anything)
	mockOutbox.AssertExpectations(t)
}

// TestOutboxPublisher_BackoffExpired_Retries verifies that once the backoff
// window for an event has passed, the next batch attempts to publish again.
func TestOutboxPublisher_BackoffExpired_Retries(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger()

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)
	// Manually place the event's backoff window in the past.
	publisher.retryTimers[evtID] = time.Now().Add(-time.Second)

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)
	mockNats.On("PublishRaw", mock.Anything, evt.Subject, evt.Payload).Return(nil)
	mockOutbox.On("MarkPublished", mock.Anything, evtID).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockNats.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
}

func TestOutboxPublisher_BackoffDuration_Exponential(t *testing.T) {
	publisher := NewOutboxPublisher(nil, nil, testLogger())

	assert.Equal(t, publisher.interval, publisher.backoffDuration(0))
	assert.Equal(t, 2*publisher.interval, publisher.backoffDuration(1))
	assert.Equal(t, 4*publisher.interval, publisher.backoffDuration(2))
}

func TestOutboxPublisher_BackoffDuration_CapsAtMax(t *testing.T) {
	publisher := NewOutboxPublisher(nil, nil, testLogger())

	assert.Equal(t, maxBackoff, publisher.backoffDuration(10))
	assert.Equal(t, maxBackoff, publisher.backoffDuration(100))
	assert.True(t, publisher.backoffDuration(10) <= maxBackoff)
}

func TestOutboxPublisher_CleanupBackoff_RemovesOnlyExpired(t *testing.T) {
	publisher := NewOutboxPublisher(nil, nil, testLogger())

	expiredID := uuid.New()
	activeID := uuid.New()
	publisher.retryTimers[expiredID] = time.Now().Add(-time.Second)
	publisher.retryTimers[activeID] = time.Now().Add(time.Hour)

	publisher.cleanupBackoff()

	_, expiredGone := publisher.retryTimers[expiredID]
	_, activeStillThere := publisher.retryTimers[activeID]
	assert.False(t, expiredGone, "expired backoff timer should be removed")
	assert.True(t, activeStillThere, "active backoff timer should remain")
}
