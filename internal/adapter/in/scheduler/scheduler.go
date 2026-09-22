package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

type Scheduler struct {
	scanner  ports.ScanRunner
	interval time.Duration
	log      *slog.Logger
}

func New(scanner ports.ScanRunner, interval time.Duration, log *slog.Logger) *Scheduler {
	return &Scheduler{
		scanner:  scanner,
		interval: interval,
		log:      log,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.interval <= 0 {
		return fmt.Errorf("scheduler interval must be > 0: got %v", s.interval)
	}

	runScan := func() {
		_, err := s.scanner.Scan(ctx)
		if err != nil {
			if errors.Is(err, domain.ErrScanInProgress) {
				if s.log != nil {
					s.log.Debug("scan skipped: already in progress")
				}
			} else {
				if s.log != nil {
					s.log.Error("scan failed", "error", err)
				}
			}
		}
	}

	// Runs one scan immediately
	runScan()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runScan()
		}
	}
}
