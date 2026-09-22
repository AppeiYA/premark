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

func TestMarketQuery_ListMarket(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	snapRepo := memstore.NewSnapshotRepository()

	t.Run("source error returns error", func(t *testing.T) {
		source := &testsupport.FakeSource{Err: errors.New("upstream failed")}
		query := usecase.NewMarketQuery(source, snapRepo, clock)

		_, err := query.ListMarket(ctx)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("filters invalid prices and sorts by premium bps ascending then symbol", func(t *testing.T) {
		s1 := testsupport.Snap("SPACEX", 110, 100, now)         // +1000 bps
		s2 := testsupport.Snap("ANTHROPIC", 90, 100, now)       // -1000 bps
		s3 := testsupport.Snap("OPENAI", 95, 100, now)          // -500 bps
		s4 := testsupport.Snap("FIGUREAI", 90, 100, now)        // -1000 bps (tie with ANTHROPIC)
		sInvalid := testsupport.Snap("BADPRICE", -10, 100, now) // invalid price

		source := &testsupport.FakeSource{
			Snapshots: []domain.MarketSnapshot{s1, s2, s3, s4, sInvalid},
		}
		query := usecase.NewMarketQuery(source, snapRepo, clock)

		views, err := query.ListMarket(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(views) != 4 {
			t.Fatalf("expected 4 views (1 invalid filtered), got %d", len(views))
		}

		// Expected order:
		// 1. ANTHROPIC (-1000 bps)
		// 2. FIGUREAI (-1000 bps, ANTHROPIC < FIGUREAI alphabetically)
		// 3. OPENAI (-500 bps)
		// 4. SPACEX (+1000 bps)
		expectedOrder := []string{"ANTHROPIC", "FIGUREAI", "OPENAI", "SPACEX"}
		for i, exp := range expectedOrder {
			if string(views[i].Snapshot.Token.Symbol) != exp {
				t.Errorf("at index %d expected %s, got %s", i, exp, views[i].Snapshot.Token.Symbol)
			}
		}

		// Check labels
		if views[0].Label != domain.LabelCheap {
			t.Errorf("expected ANTHROPIC to be cheap, got %s", views[0].Label)
		}
		if views[3].Label != domain.LabelRich {
			t.Errorf("expected SPACEX to be rich, got %s", views[3].Label)
		}
	})
}

func TestMarketQuery_GetMarket(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	snapRepo := memstore.NewSnapshotRepository()

	source := &testsupport.FakeSource{
		Snapshots: []domain.MarketSnapshot{
			testsupport.Snap("ANTHROPIC", 950, 1000, now),
			testsupport.Snap("INVALID", 0, 1000, now),
		},
	}
	query := usecase.NewMarketQuery(source, snapRepo, clock)

	t.Run("invalid symbol returns ErrInvalidSymbol", func(t *testing.T) {
		_, err := query.GetMarket(ctx, "!")
		if !errors.Is(err, domain.ErrInvalidSymbol) {
			t.Errorf("expected ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("upstream error returns error", func(t *testing.T) {
		errSource := &testsupport.FakeSource{Err: errors.New("timeout")}
		q := usecase.NewMarketQuery(errSource, snapRepo, clock)
		_, err := q.GetMarket(ctx, "ANTHROPIC")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("token not found returns ErrTokenNotFound", func(t *testing.T) {
		_, err := query.GetMarket(ctx, "NONEXISTENT")
		if !errors.Is(err, domain.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("found token with invalid price returns ErrInvalidPrice", func(t *testing.T) {
		_, err := query.GetMarket(ctx, "INVALID")
		if !errors.Is(err, domain.ErrInvalidPrice) {
			t.Errorf("expected ErrInvalidPrice, got %v", err)
		}
	})

	t.Run("found valid token returns MarketView", func(t *testing.T) {
		view, err := query.GetMarket(ctx, "anthropic")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if view.Snapshot.Token.Symbol != "ANTHROPIC" {
			t.Errorf("expected ANTHROPIC, got %s", view.Snapshot.Token.Symbol)
		}
		if view.Label != domain.LabelCheap {
			t.Errorf("expected LabelCheap, got %s", view.Label)
		}
	})
}

func TestMarketQuery_History(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	snapRepo := memstore.NewSnapshotRepository()
	source := &testsupport.FakeSource{}
	query := usecase.NewMarketQuery(source, snapRepo, clock)

	// Populate history
	s1 := testsupport.Snap("ANTHROPIC", 940, 1000, now.Add(-5*time.Hour))
	s2 := testsupport.Snap("ANTHROPIC", 950, 1000, now.Add(-2*time.Hour))
	sOld := testsupport.Snap("ANTHROPIC", 900, 1000, now.Add(-48*time.Hour))
	_ = snapRepo.SaveAll(ctx, []domain.MarketSnapshot{s1, s2, sOld})

	t.Run("invalid symbol returns ErrInvalidSymbol", func(t *testing.T) {
		_, err := query.History(ctx, "!", 24*time.Hour)
		if !errors.Is(err, domain.ErrInvalidSymbol) {
			t.Errorf("expected ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("window too small (<1h) returns ErrInvalidArgument", func(t *testing.T) {
		_, err := query.History(ctx, "ANTHROPIC", 30*time.Minute)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("window too large (>168h) returns ErrInvalidArgument", func(t *testing.T) {
		_, err := query.History(ctx, "ANTHROPIC", 169*time.Hour)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("valid window returns snapshots since window", func(t *testing.T) {
		snaps, err := query.History(ctx, "ANTHROPIC", 24*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(snaps) != 2 {
			t.Fatalf("expected 2 snapshots within 24h, got %d", len(snaps))
		}
		if snaps[0].TokenPrice != 940 || snaps[1].TokenPrice != 950 {
			t.Errorf("unexpected history order: %+v", snaps)
		}
	})
}
