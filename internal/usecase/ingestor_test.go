package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"premark/internal/adapter/out/memstore"
	"premark/internal/domain"
	"premark/internal/testsupport"
	"premark/internal/usecase"
)

func TestIngestor_Ingest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	t.Run("source fetch error returns error", func(t *testing.T) {
		source := &testsupport.FakeSource{Err: errors.New("upstream failed")}
		snapRepo := memstore.NewSnapshotRepository()
		ingestor := usecase.NewIngestor(source, snapRepo, nil)

		_, err := ingestor.Ingest(ctx)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("save failure is logged and does not fail Ingest", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{testsupport.Snap("ANTHROPIC", 950, 1000, now)}
		source := &testsupport.FakeSource{Snapshots: snaps}

		underlying := memstore.NewSnapshotRepository()
		failingRepo := testsupport.NewFailingSnapshotRepo(underlying)
		failingRepo.FailSaveAll = errors.New("sqlite locked")

		ingestor := usecase.NewIngestor(source, failingRepo, nil)

		res, err := ingestor.Ingest(ctx)
		if err != nil {
			t.Fatalf("expected nil error despite save failure, got %v", err)
		}
		if len(res) != 1 {
			t.Errorf("expected 1 snapshot, got %d", len(res))
		}
	})

	t.Run("successful ingest saves and returns snapshots", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{
			testsupport.Snap("ANTHROPIC", 950, 1000, now),
			testsupport.Snap("OPENAI", 900, 1000, now),
		}
		source := &testsupport.FakeSource{Snapshots: snaps}
		snapRepo := memstore.NewSnapshotRepository()
		ingestor := usecase.NewIngestor(source, snapRepo, nil)

		res, err := ingestor.Ingest(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 snapshots, got %d", len(res))
		}

		// Verify saved in repo
		history, _ := snapRepo.History(ctx, "ANTHROPIC", now.Add(-time.Hour))
		if len(history) != 1 {
			t.Errorf("expected 1 snapshot saved in history, got %d", len(history))
		}
	})
}
