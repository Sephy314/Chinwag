package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type OutboxEvent struct {
	Id           uuid.UUID  `db:"id"`
	EventType    string     `db:"event_type"`
	Subject      string     `db:"subject"`
	Payload      []byte     `db:"payload"`
	RoomId       uuid.UUID  `db:"room_id"`
	CreatedAt    time.Time  `db:"created_at"`
	PublishedAt  *time.Time `db:"published_at"`
	RetryCount   int        `db:"retry_count"`
}

type OutboxRepoInterface interface {
	Insert(ctx context.Context, event OutboxEvent) error
	PollPending(ctx context.Context, batchSize int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
}

type OutboxRepo struct {
	db sqlx.ExtContext
}

func NewOutboxRepo(db sqlx.ExtContext) *OutboxRepo {
	return &OutboxRepo{db: db}
}

func (r *OutboxRepo) Insert(ctx context.Context, event OutboxEvent) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO outbox_events (id, event_type, subject, payload, room_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		event.Id, event.EventType, event.Subject, event.Payload, event.RoomId,
	)
	return err
}

func (r *OutboxRepo) PollPending(ctx context.Context, batchSize int) ([]OutboxEvent, error) {
	var events []OutboxEvent
	err := sqlx.SelectContext(
		ctx, r.db, &events,
		`SELECT id, event_type, subject, payload, room_id, created_at, published_at, retry_count
		 FROM outbox_events
		 WHERE published_at IS NULL AND retry_count < 20
		 ORDER BY created_at ASC
		 LIMIT $1`,
		batchSize,
	)
	return events, err
}

func (r *OutboxRepo) MarkPublished(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE outbox_events SET published_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

func (r *OutboxRepo) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE outbox_events SET retry_count = retry_count + 1 WHERE id = $1`,
		id,
	)
	return err
}
