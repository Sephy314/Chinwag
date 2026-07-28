package scheduler

import (
	"context"
	"log/slog"
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
	log      *slog.Logger
}

func NewPopScheduler(p Popper, interval time.Duration, log *slog.Logger) *PopScheduler {
	return &PopScheduler{
		popper:   p,
		interval: interval,
		stop:     make(chan struct{}),
		log:      log,
	}
}

func (s *PopScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("pop scheduler started")

	for {
		select {
		case <-ticker.C:
			rows, err := s.popper.PopRooms(ctx)
			if err != nil {
				s.log.Error("pop scheduler error", "err", err)
			} else {
				s.log.Info("pop scheduler tick", "popped", rows)
			}
		case <-s.stop:
			s.log.Info("pop scheduler stopped")
			return
		case <-ctx.Done():
			s.log.Info("pop scheduler stopped: context cancelled")
			return
		}
	}
}

func (s *PopScheduler) Stop() {
	close(s.stop)
}
