package scheduler

import (
	"context"
	"log"
	"time"
)

type KeyRotator interface {
	Rotate(ctx context.Context) error
}

type KeyRotationScheduler struct {
	rotator  KeyRotator
	interval time.Duration
	stop     chan struct{}
}

func NewKeyRotationScheduler(r KeyRotator, interval time.Duration) *KeyRotationScheduler {
	return &KeyRotationScheduler{
		rotator:  r,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (s *KeyRotationScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Println("key rotation scheduler started")

	for {
		select {
		case <-ticker.C:
			err := s.rotator.Rotate(ctx)
			if err != nil {
				log.Printf("key rotation scheduler error: %v", err)
			} else {
				log.Println("key rotation completed")
			}
		case <-s.stop:
			log.Println("key rotation scheduler stopped")
			return
		case <-ctx.Done():
			log.Println("key rotation scheduler stopped: context cancelled")
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
