package service

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/google/uuid"
)

const maxBackoff = 30 * time.Second

type RawNatsPublisher interface {
	PublishRaw(ctx context.Context, subject string, data []byte) error
}

type OutboxPublisher struct {
	outboxRepo  OutboxRepoInterface
	nats        RawNatsPublisher
	log         *slog.Logger
	interval    time.Duration
	batchSize   int
	mu          sync.Mutex
	retryTimers map[uuid.UUID]time.Time
}

type OutboxRepoInterface interface {
	PollPending(ctx context.Context, batchSize int) ([]repo.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
}

func NewOutboxPublisher(outboxRepo OutboxRepoInterface, nats RawNatsPublisher, log *slog.Logger) *OutboxPublisher {
	return &OutboxPublisher{
		outboxRepo:  outboxRepo,
		nats:        nats,
		log:         log,
		interval:    100 * time.Millisecond,
		batchSize:   50,
		retryTimers: make(map[uuid.UUID]time.Time),
	}
}

func (p *OutboxPublisher) backoffDuration(retryCount int) time.Duration {
	d := time.Duration(float64(p.interval) * math.Pow(2, float64(retryCount)))
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func (p *OutboxPublisher) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.log.Info("outbox publisher started", "interval", p.interval, "batch_size", p.batchSize)

	for {
		select {
		case <-ctx.Done():
			p.log.Info("outbox publisher stopped")
			return
		case <-ticker.C:
			p.cleanupBackoff()
			p.processBatch(ctx)
		}
	}
}

func (p *OutboxPublisher) cleanupBackoff() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for id, until := range p.retryTimers {
		if now.After(until) {
			delete(p.retryTimers, id)
		}
	}
}

func (p *OutboxPublisher) processBatch(ctx context.Context) {
	events, err := p.outboxRepo.PollPending(ctx, p.batchSize)
	if err != nil {
		p.log.Error("outbox poll failed", "error", err)
		return
	}

	for _, evt := range events {
		if p.inBackoff(evt.Id) {
			continue
		}

		if err := p.nats.PublishRaw(ctx, evt.Subject, evt.Payload); err != nil {
			p.log.Error("outbox publish failed",
				"event_id", evt.Id,
				"event_type", evt.EventType,
				"retry_count", evt.RetryCount,
				"error", err,
			)
			if incErr := p.outboxRepo.IncrementRetry(ctx, evt.Id); incErr != nil {
				p.log.Error("failed to increment retry count", "event_id", evt.Id, "error", incErr)
			}
			p.setBackoff(evt.Id, evt.RetryCount+1)
			continue
		}

		p.clearBackoff(evt.Id)

		if err := p.outboxRepo.MarkPublished(ctx, evt.Id); err != nil {
			p.log.Error("failed to mark event published", "event_id", evt.Id, "error", err)
		}
	}
}

func (p *OutboxPublisher) inBackoff(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	until, ok := p.retryTimers[id]
	return ok && time.Now().Before(until)
}

func (p *OutboxPublisher) setBackoff(id uuid.UUID, retryCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retryTimers[id] = time.Now().Add(p.backoffDuration(retryCount))
}

func (p *OutboxPublisher) clearBackoff(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.retryTimers, id)
}
