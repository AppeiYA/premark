package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"premark/internal/adapter/in/scheduler"
	"premark/internal/ports"
)

type mockScanRunner struct {
	scans int32
}

func (m *mockScanRunner) Scan(ctx context.Context) (ports.ScanReport, error) {
	atomic.AddInt32(&m.scans, 1)
	return ports.ScanReport{}, nil
}

func TestScheduler(t *testing.T) {
	t.Run("invalid interval returns error", func(t *testing.T) {
		s := scheduler.New(&mockScanRunner{}, 0, nil)
		err := s.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error for interval 0, got nil")
		}
	})

	t.Run("runs initial scan immediately and respects context cancellation", func(t *testing.T) {
		runner := &mockScanRunner{}
		s := scheduler.New(runner, 1*time.Hour, nil) // long interval

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)

		go func() {
			errCh <- s.Run(ctx)
		}()

		// Wait briefly to allow immediate scan to execute
		time.Sleep(20 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("expected nil error on context cancellation, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("scheduler did not exit on context cancel")
		}

		if atomic.LoadInt32(&runner.scans) != 1 {
			t.Errorf("expected 1 immediate scan, got %d", atomic.LoadInt32(&runner.scans))
		}
	})

	t.Run("ticker triggers subsequent scans", func(t *testing.T) {
		runner := &mockScanRunner{}
		s := scheduler.New(runner, 25*time.Millisecond, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Run(ctx)
		}()

		// Wait enough for immediate scan + at least 2 ticks
		time.Sleep(80 * time.Millisecond)
		cancel()

		<-errCh
		count := atomic.LoadInt32(&runner.scans)
		if count < 3 {
			t.Errorf("expected at least 3 scans, got %d", count)
		}
	})
}
