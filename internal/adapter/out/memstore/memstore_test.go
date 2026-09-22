package memstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"premark/internal/adapter/out/memstore"
	"premark/internal/domain"
)

func TestMemstoreRuleRepository(t *testing.T) {
	ctx := context.Background()
	repo := memstore.NewRuleRepository()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	r1 := domain.Rule{
		ID:        "r1",
		Owner:     "alice",
		Symbol:    "ANTHROPIC",
		Enabled:   true,
		CreatedAt: now,
	}

	t.Run("save and get rule", func(t *testing.T) {
		err := repo.Save(ctx, r1)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}

		got, err := repo.Get(ctx, "r1")
		if err != nil {
			t.Fatalf("unexpected get error: %v", err)
		}
		if got.ID != "r1" {
			t.Errorf("expected r1, got %s", got.ID)
		}
	})

	t.Run("get missing rule returns ErrRuleNotFound", func(t *testing.T) {
		_, err := repo.Get(ctx, "missing")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("list by owner with ordering", func(t *testing.T) {
		r2 := domain.Rule{ID: "r2", Owner: "alice", Symbol: "OPENAI", Enabled: false, CreatedAt: now.Add(time.Minute)}
		rBob := domain.Rule{ID: "r3", Owner: "bob", Symbol: "SPACEX", Enabled: true, CreatedAt: now}
		_ = repo.Save(ctx, r2)
		_ = repo.Save(ctx, rBob)

		aliceRules, err := repo.ListByOwner(ctx, "alice")
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if len(aliceRules) != 2 {
			t.Fatalf("expected 2 rules for alice, got %d", len(aliceRules))
		}
		if aliceRules[0].ID != "r1" || aliceRules[1].ID != "r2" {
			t.Errorf("unexpected ordering: %+v", aliceRules)
		}
	})

	t.Run("list enabled rules", func(t *testing.T) {
		enabled, err := repo.ListEnabled(ctx)
		if err != nil {
			t.Fatalf("unexpected list enabled error: %v", err)
		}
		if len(enabled) != 2 {
			t.Fatalf("expected 2 enabled rules, got %d", len(enabled))
		}
	})

	t.Run("delete rule", func(t *testing.T) {
		err := repo.Delete(ctx, "r1")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}

		_, err = repo.Get(ctx, "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound after delete, got %v", err)
		}

		err = repo.Delete(ctx, "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound on missing delete, got %v", err)
		}
	})
}

func TestMemstoreSnapshotRepository(t *testing.T) {
	ctx := context.Background()
	repo := memstore.NewSnapshotRepository()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	t.Run("save and history query", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{
			{Token: domain.Token{Symbol: "ANTHROPIC"}, TokenPrice: 900, ObservedAt: now.Add(-2 * time.Hour)},
			{Token: domain.Token{Symbol: "ANTHROPIC"}, TokenPrice: 950, ObservedAt: now.Add(-1 * time.Hour)},
			{Token: domain.Token{Symbol: "OPENAI"}, TokenPrice: 2000, ObservedAt: now.Add(-1 * time.Hour)},
		}

		err := repo.SaveAll(ctx, snaps)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}

		history, err := repo.History(ctx, "ANTHROPIC", now.Add(-90*time.Minute))
		if err != nil {
			t.Fatalf("unexpected history error: %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("expected 1 snapshot, got %d", len(history))
		}
		if history[0].TokenPrice != 950 {
			t.Errorf("expected TokenPrice=950, got %v", history[0].TokenPrice)
		}

		allHistory, err := repo.History(ctx, "ANTHROPIC", now.Add(-3*time.Hour))
		if err != nil {
			t.Fatalf("unexpected all history error: %v", err)
		}
		if len(allHistory) != 2 {
			t.Fatalf("expected 2 snapshots, got %d", len(allHistory))
		}
		if allHistory[0].TokenPrice != 900 || allHistory[1].TokenPrice != 950 {
			t.Errorf("expected chronological order 900 then 950, got %+v", allHistory)
		}
	})
}

func TestMemstoreSignalRepository(t *testing.T) {
	ctx := context.Background()
	repo := memstore.NewSignalRepository()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	sig1 := domain.Signal{ID: "sig-1", Owner: "alice", CreatedAt: now}
	sig2 := domain.Signal{ID: "sig-2", Owner: "alice", CreatedAt: now.Add(time.Minute)}
	sig3 := domain.Signal{ID: "sig-3", Owner: "bob", CreatedAt: now}

	t.Run("save and list by owner with limit and order", func(t *testing.T) {
		_ = repo.Save(ctx, sig1)
		_ = repo.Save(ctx, sig2)
		_ = repo.Save(ctx, sig3)

		sigs, err := repo.ListByOwner(ctx, "alice", 1)
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if len(sigs) != 1 {
			t.Fatalf("expected 1 signal, got %d", len(sigs))
		}
		if sigs[0].ID != "sig-2" {
			t.Errorf("expected newest first (sig-2), got %s", sigs[0].ID)
		}

		allAlice, err := repo.ListByOwner(ctx, "alice", 10)
		if err != nil {
			t.Fatalf("unexpected list all error: %v", err)
		}
		if len(allAlice) != 2 {
			t.Fatalf("expected 2 signals, got %d", len(allAlice))
		}
		if allAlice[0].ID != "sig-2" || allAlice[1].ID != "sig-1" {
			t.Errorf("expected sig-2 then sig-1, got %+v", allAlice)
		}
	})
}
