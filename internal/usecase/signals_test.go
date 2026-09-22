package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"premark/internal/adapter/out/memstore"
	"premark/internal/domain"
	"premark/internal/usecase"
)

func TestSignalQuery_ListSignals(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	signalRepo := memstore.NewSignalRepository()
	query := usecase.NewSignalQuery(signalRepo)

	sigAlice1 := domain.Signal{ID: "sig-1", Owner: "alice", Symbol: "ANTHROPIC", CreatedAt: now.Add(-10 * time.Minute)}
	sigAlice2 := domain.Signal{ID: "sig-2", Owner: "alice", Symbol: "OPENAI", CreatedAt: now.Add(-5 * time.Minute)}
	sigBob := domain.Signal{ID: "sig-3", Owner: "bob", Symbol: "SPACEX", CreatedAt: now}
	_ = signalRepo.Save(ctx, sigAlice1)
	_ = signalRepo.Save(ctx, sigAlice2)
	_ = signalRepo.Save(ctx, sigBob)

	t.Run("empty owner returns ErrInvalidArgument", func(t *testing.T) {
		_, err := query.ListSignals(ctx, "", 20)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("negative limit returns ErrInvalidArgument", func(t *testing.T) {
		_, err := query.ListSignals(ctx, "alice", -1)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("limit exceeds 100 returns ErrInvalidArgument", func(t *testing.T) {
		_, err := query.ListSignals(ctx, "alice", 101)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("default limit 0 behaves as 20", func(t *testing.T) {
		sigs, err := query.ListSignals(ctx, "alice", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sigs) != 2 {
			t.Fatalf("expected 2 signals, got %d", len(sigs))
		}
		// descending order: sig-2 (5 min ago) then sig-1 (10 min ago)
		if sigs[0].ID != "sig-2" || sigs[1].ID != "sig-1" {
			t.Errorf("unexpected order: %+v", sigs)
		}
	})

	t.Run("explicit limit respected", func(t *testing.T) {
		sigs, err := query.ListSignals(ctx, "alice", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sigs) != 1 {
			t.Fatalf("expected 1 signal, got %d", len(sigs))
		}
		if sigs[0].ID != "sig-2" {
			t.Errorf("expected sig-2, got %s", sigs[0].ID)
		}
	})
}
