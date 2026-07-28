package scheduler

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
)

type KeyRotator interface {
	Rotate(ctx context.Context) error
}

type KeyRotationScheduler struct {
	rotator  KeyRotator
	interval time.Duration
	stop     chan struct{}
	log      logger.Logger
}

func NewKeyRotationScheduler(r KeyRotator, interval time.Duration, log logger.Logger) *KeyRotationScheduler {
	return &KeyRotationScheduler{
		rotator:  r,
		interval: interval,
		stop:     make(chan struct{}),
		log:      log,
	}
}

func (s *KeyRotationScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("key rotation scheduler started")

	for {
		select {
		case <-ticker.C:
			err := s.rotator.Rotate(ctx)
			if err != nil {
				s.log.Error("key rotation scheduler error", "error", err)
			} else {
				s.log.Info("key rotation completed")
			}
		case <-s.stop:
			s.log.Info("key rotation scheduler stopped")
			return
		case <-ctx.Done():
			s.log.Info("key rotation scheduler stopped: context cancelled")
			return
		}
	}
}

func (s *KeyRotationScheduler) Stop() {
	close(s.stop)
}

func NextMidnight() time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return next.Sub(now)
}
