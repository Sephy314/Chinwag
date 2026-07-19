package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type Popper interface {
	PopRooms(ctx context.Context) (int64, error)
}

type SQLPopper struct {
	db sqlx.ExtContext
}

func (p *SQLPopper) PopRooms(ctx context.Context) (int64, error) {
	res, err := p.db.ExecContext(
		ctx,
		`UPDATE rooms 
		 SET popped_at = NOW(), updated_at = NOW()
		 WHERE popped_at IS NULL 
		   AND deleted_at IS NULL 
		   AND pop_at <= NOW()`,
	)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func NewSQLPopper(db sqlx.ExtContext) *SQLPopper {
	return &SQLPopper{db: db}
}

type PopScheduler struct {
	popper   Popper
	interval time.Duration
	stop     chan struct{}
}

func NewPopScheduler(p Popper, interval time.Duration) *PopScheduler {
	return &PopScheduler{
		popper:   p,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (s *PopScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Println("pop scheduler started")

	for {
		select {
		case <-ticker.C:
			rows, err := s.popper.PopRooms(ctx)
			if err != nil {
				log.Printf("pop scheduler error: %v", err)
			} else if rows > 0 {
				log.Printf("popped %d room(s)", rows)
			}
		case <-s.stop:
			log.Println("pop scheduler stopped")
			return
		case <-ctx.Done():
			log.Println("pop scheduler stopped: context cancelled")
			return
		}
	}
}

func (s *PopScheduler) Stop() {
	close(s.stop)
}
