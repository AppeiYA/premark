package usecase

import (
	"context"
	"log/slog"
	"sync"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.ScanRunner = (*Scanner)(nil)

type Scanner struct {
	ingest ports.MarketIngestor
	eval   ports.RuleEvaluator
	log    *slog.Logger
	mu     sync.Mutex
}

func NewScanner(ingest ports.MarketIngestor, eval ports.RuleEvaluator, log *slog.Logger) *Scanner {
	return &Scanner{
		ingest: ingest,
		eval:   eval,
		log:    log,
	}
}

func (s *Scanner) Scan(ctx context.Context) (ports.ScanReport, error) {
	if !s.mu.TryLock() {
		return ports.ScanReport{}, domain.ErrScanInProgress
	}
	defer s.mu.Unlock()

	snaps, err := s.ingest.Ingest(ctx)
	if err != nil {
		return ports.ScanReport{}, err
	}

	evalReport, err := s.eval.EvaluateAll(ctx, snaps)
	if err != nil {
		return ports.ScanReport{Snapshots: len(snaps)}, err
	}

	return ports.ScanReport{
		Snapshots:  len(snaps),
		Evaluation: evalReport,
	}, nil
}
