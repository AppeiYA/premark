package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
	"premark/internal/usecase"
)

type fakeIngestor struct {
	snaps []domain.MarketSnapshot
	err   error
	delay time.Duration
}

func (f *fakeIngestor) Ingest(ctx context.Context) ([]domain.MarketSnapshot, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.snaps, nil
}

type fakeEvaluator struct {
	report ports.EvaluationReport
	err    error
}

func (f *fakeEvaluator) EvaluateAll(ctx context.Context, snapshots []domain.MarketSnapshot) (ports.EvaluationReport, error) {
	if f.err != nil {
		return ports.EvaluationReport{}, f.err
	}
	return f.report, nil
}

func TestScanner_Scan(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent scan returns ErrScanInProgress", func(t *testing.T) {
		ingest := &fakeIngestor{delay: 50 * time.Millisecond}
		eval := &fakeEvaluator{}
		scanner := usecase.NewScanner(ingest, eval, nil)

		var wg sync.WaitGroup
		var err1, err2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err1 = scanner.Scan(ctx)
		}()
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // Ensure first goroutine has acquired the lock
			_, err2 = scanner.Scan(ctx)
		}()
		wg.Wait()

		if err1 != nil && !errors.Is(err1, domain.ErrScanInProgress) {
			t.Errorf("unexpected error in goroutine 1: %v", err1)
		}
		if err2 != nil && !errors.Is(err2, domain.ErrScanInProgress) {
			t.Errorf("unexpected error in goroutine 2: %v", err2)
		}
		if !errors.Is(err1, domain.ErrScanInProgress) && !errors.Is(err2, domain.ErrScanInProgress) {
			t.Errorf("expected at least one call to return ErrScanInProgress")
		}
	})

	t.Run("ingest failure returns error", func(t *testing.T) {
		ingest := &fakeIngestor{err: errors.New("ingest error")}
		eval := &fakeEvaluator{}
		scanner := usecase.NewScanner(ingest, eval, nil)

		_, err := scanner.Scan(ctx)
		if err == nil || err.Error() != "ingest error" {
			t.Errorf("expected ingest error, got %v", err)
		}
	})

	t.Run("evaluator failure returns error and snapshot count", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{{}, {}}
		ingest := &fakeIngestor{snaps: snaps}
		eval := &fakeEvaluator{err: errors.New("eval error")}
		scanner := usecase.NewScanner(ingest, eval, nil)

		report, err := scanner.Scan(ctx)
		if err == nil || err.Error() != "eval error" {
			t.Errorf("expected eval error, got %v", err)
		}
		if report.Snapshots != 2 {
			t.Errorf("expected Snapshots=2, got %d", report.Snapshots)
		}
	})

	t.Run("successful scan returns complete report", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{{}, {}, {}}
		ingest := &fakeIngestor{snaps: snaps}
		eval := &fakeEvaluator{
			report: ports.EvaluationReport{
				Evaluated: 5,
				Signals:   []domain.Signal{{ID: "sig-1"}},
			},
		}
		scanner := usecase.NewScanner(ingest, eval, nil)

		report, err := scanner.Scan(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Snapshots != 3 {
			t.Errorf("expected Snapshots=3, got %d", report.Snapshots)
		}
		if report.Evaluation.Evaluated != 5 {
			t.Errorf("expected Evaluated=5, got %d", report.Evaluation.Evaluated)
		}
		if len(report.Evaluation.Signals) != 1 {
			t.Errorf("expected 1 signal, got %d", len(report.Evaluation.Signals))
		}
	})
}
