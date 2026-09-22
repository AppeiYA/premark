package sqlitestore_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"premark/internal/adapter/out/sqlitestore"
	"premark/internal/domain"
)

func setupTestDB(t *testing.T) *sqlitestore.RuleRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return sqlitestore.NewRuleRepository(db)
}

func setupAllRepos(t *testing.T) (*sqlitestore.RuleRepository, *sqlitestore.SnapshotRepository, *sqlitestore.SignalRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return sqlitestore.NewRuleRepository(db),
		sqlitestore.NewSnapshotRepository(db),
		sqlitestore.NewSignalRepository(db)
}

func TestSqliteRuleRepository(t *testing.T) {
	ctx := context.Background()
	ruleRepo, _, _ := setupAllRepos(t)

	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	r1 := domain.Rule{
		ID:                domain.RuleID("r1"),
		Owner:             domain.OwnerID("alice"),
		Symbol:            domain.Symbol("ANTHROPIC"),
		MaxPremiumBps:     -200,
		Budget:            500 * domain.USDCUnit,
		MaxPriceImpactBps: 50,
		Cooldown:          30 * time.Minute,
		Enabled:           true,
		CreatedAt:         now,
		LastTriggeredAt:   time.Time{},
	}

	t.Run("save and get rule", func(t *testing.T) {
		err := ruleRepo.Save(ctx, r1)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}

		got, err := ruleRepo.Get(ctx, r1.ID)
		if err != nil {
			t.Fatalf("unexpected get error: %v", err)
		}

		if got.ID != r1.ID || got.Owner != r1.Owner || got.Symbol != r1.Symbol ||
			got.MaxPremiumBps != r1.MaxPremiumBps || got.Budget != r1.Budget ||
			got.MaxPriceImpactBps != r1.MaxPriceImpactBps || got.Cooldown != r1.Cooldown ||
			got.Enabled != r1.Enabled || !got.CreatedAt.Equal(r1.CreatedAt) ||
			!got.LastTriggeredAt.IsZero() {
			t.Errorf("saved rule does not match retrieved rule: got %+v, want %+v", got, r1)
		}
	})

	t.Run("upsert updates existing rule", func(t *testing.T) {
		triggeredAt := now.Add(10 * time.Minute)
		updated := r1
		updated.Enabled = false
		updated.LastTriggeredAt = triggeredAt

		err := ruleRepo.Save(ctx, updated)
		if err != nil {
			t.Fatalf("unexpected update error: %v", err)
		}

		got, err := ruleRepo.Get(ctx, r1.ID)
		if err != nil {
			t.Fatalf("unexpected get error: %v", err)
		}
		if got.Enabled != false {
			t.Errorf("expected Enabled=false, got true")
		}
		if !got.LastTriggeredAt.Equal(triggeredAt) {
			t.Errorf("expected LastTriggeredAt=%v, got %v", triggeredAt, got.LastTriggeredAt)
		}
	})

	t.Run("get non-existent rule returns ErrRuleNotFound", func(t *testing.T) {
		_, err := ruleRepo.Get(ctx, "non-existent")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("list by owner with ordering", func(t *testing.T) {
		r2 := domain.Rule{
			ID:        domain.RuleID("r2"),
			Owner:     domain.OwnerID("alice"),
			Symbol:    domain.Symbol("OPENAI"),
			Enabled:   true,
			CreatedAt: now.Add(5 * time.Minute),
		}
		rBob := domain.Rule{
			ID:        domain.RuleID("r3"),
			Owner:     domain.OwnerID("bob"),
			Symbol:    domain.Symbol("SPACEX"),
			Enabled:   true,
			CreatedAt: now,
		}
		_ = ruleRepo.Save(ctx, r2)
		_ = ruleRepo.Save(ctx, rBob)

		aliceRules, err := ruleRepo.ListByOwner(ctx, "alice")
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if len(aliceRules) != 2 {
			t.Fatalf("expected 2 rules for alice, got %d", len(aliceRules))
		}
		// Order: CreatedAt asc, ID asc (r1 was at now, r2 was at now + 5m)
		if aliceRules[0].ID != "r1" || aliceRules[1].ID != "r2" {
			t.Errorf("unexpected ordering: %+v", aliceRules)
		}
	})

	t.Run("list enabled rules", func(t *testing.T) {
		// r1 is disabled, r2 and rBob are enabled
		enabledRules, err := ruleRepo.ListEnabled(ctx)
		if err != nil {
			t.Fatalf("unexpected list enabled error: %v", err)
		}
		if len(enabledRules) != 2 {
			t.Fatalf("expected 2 enabled rules, got %d", len(enabledRules))
		}
		// rBob (now) then r2 (now + 5m)
		if enabledRules[0].ID != "r3" || enabledRules[1].ID != "r2" {
			t.Errorf("unexpected enabled ordering: %+v", enabledRules)
		}
	})

	t.Run("delete rule", func(t *testing.T) {
		err := ruleRepo.Delete(ctx, "r1")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}

		_, err = ruleRepo.Get(ctx, "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound after delete, got %v", err)
		}

		// Delete missing rule returns ErrRuleNotFound
		err = ruleRepo.Delete(ctx, "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound when deleting missing rule, got %v", err)
		}
	})
}

func TestSqliteSnapshotRepository(t *testing.T) {
	ctx := context.Background()
	_, snapRepo, _ := setupAllRepos(t)

	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	t.Run("save empty list is no-op", func(t *testing.T) {
		err := snapRepo.SaveAll(ctx, nil)
		if err != nil {
			t.Errorf("unexpected error on empty save: %v", err)
		}
	})

	t.Run("save and history query with filtering and ordering", func(t *testing.T) {
		snaps := []domain.MarketSnapshot{
			{
				Token:            domain.Token{Symbol: "ANTHROPIC", Name: "Anthropic", Description: "AI", Mint: "MINT1"},
				TokenPrice:       900,
				MarkPrice:        1000,
				Supply:           500,
				MarkValuation:    500000,
				ImpliedValuation: 450000,
				ObservedAt:       now.Add(-2 * time.Hour),
			},
			{
				Token:            domain.Token{Symbol: "ANTHROPIC", Name: "Anthropic", Description: "AI", Mint: "MINT1"},
				TokenPrice:       950,
				MarkPrice:        1000,
				Supply:           500,
				MarkValuation:    500000,
				ImpliedValuation: 475000,
				ObservedAt:       now.Add(-1 * time.Hour),
			},
			{
				Token:            domain.Token{Symbol: "OPENAI", Name: "OpenAI", Description: "AI", Mint: "MINT2"},
				TokenPrice:       2000,
				MarkPrice:        2100,
				Supply:           100,
				MarkValuation:    210000,
				ImpliedValuation: 200000,
				ObservedAt:       now.Add(-1 * time.Hour),
			},
		}

		err := snapRepo.SaveAll(ctx, snaps)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}

		// Query since now - 90 minutes (should return only the snapshot at now - 1 hour)
		history, err := snapRepo.History(ctx, "ANTHROPIC", now.Add(-90*time.Minute))
		if err != nil {
			t.Fatalf("unexpected history error: %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("expected 1 snapshot, got %d", len(history))
		}
		if history[0].TokenPrice != 950 {
			t.Errorf("expected TokenPrice=950, got %v", history[0].TokenPrice)
		}

		// Query since now - 3 hours (should return both ANTHROPIC snapshots, oldest first)
		allHistory, err := snapRepo.History(ctx, "ANTHROPIC", now.Add(-3*time.Hour))
		if err != nil {
			t.Fatalf("unexpected all history error: %v", err)
		}
		if len(allHistory) != 2 {
			t.Fatalf("expected 2 snapshots, got %d", len(allHistory))
		}
		if allHistory[0].TokenPrice != 900 || allHistory[1].TokenPrice != 950 {
			t.Errorf("expected chronological order 900 then 950, got %+v", allHistory)
		}

		// Empty result for unknown symbol is not an error
		emptyHistory, err := snapRepo.History(ctx, "NONEXISTENT", now.Add(-3*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error for nonexistent symbol: %v", err)
		}
		if len(emptyHistory) != 0 {
			t.Errorf("expected 0 snapshots, got %d", len(emptyHistory))
		}
	})
}

func TestSqliteSignalRepository(t *testing.T) {
	ctx := context.Background()
	_, _, sigRepo := setupAllRepos(t)

	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	sig1 := domain.Signal{
		ID:             domain.SignalID("sig-1"),
		RuleID:         domain.RuleID("r1"),
		Owner:          domain.OwnerID("alice"),
		Symbol:         domain.Symbol("ANTHROPIC"),
		TokenPrice:     950.0,
		MarkPrice:      1000.0,
		APIPremiumBps:  -500.0,
		ExecPremiumBps: -480.0,
		Quote: domain.Quote{
			Symbol:         domain.Symbol("ANTHROPIC"),
			InputMint:      "USDC",
			OutputMint:     "MINT1",
			InAmount:       100 * domain.USDCUnit,
			OutTokens:      0.105,
			PriceImpactBps: 20.0,
			SwapURL:        "https://jup.ag/swap/USDC-ANTHROPIC",
			QuotedAt:       now,
		},
		CreatedAt: now,
	}

	sig2 := domain.Signal{
		ID:             domain.SignalID("sig-2"),
		RuleID:         domain.RuleID("r2"),
		Owner:          domain.OwnerID("alice"),
		Symbol:         domain.Symbol("OPENAI"),
		TokenPrice:     2000.0,
		MarkPrice:      2100.0,
		APIPremiumBps:  -476.0,
		ExecPremiumBps: -470.0,
		Quote: domain.Quote{
			Symbol:         domain.Symbol("OPENAI"),
			InputMint:      "USDC",
			OutputMint:     "MINT2",
			InAmount:       200 * domain.USDCUnit,
			OutTokens:      0.1,
			PriceImpactBps: 10.0,
			SwapURL:        "https://jup.ag/swap/USDC-OPENAI",
			QuotedAt:       now.Add(time.Minute),
		},
		CreatedAt: now.Add(time.Minute),
	}

	t.Run("save and list by owner with limit and order", func(t *testing.T) {
		err := sigRepo.Save(ctx, sig1)
		if err != nil {
			t.Fatalf("unexpected save sig1: %v", err)
		}
		err = sigRepo.Save(ctx, sig2)
		if err != nil {
			t.Fatalf("unexpected save sig2: %v", err)
		}

		// List with limit 1 should return newest first (sig-2)
		sigs, err := sigRepo.ListByOwner(ctx, "alice", 1)
		if err != nil {
			t.Fatalf("unexpected list: %v", err)
		}
		if len(sigs) != 1 {
			t.Fatalf("expected 1 signal, got %d", len(sigs))
		}
		if sigs[0].ID != "sig-2" {
			t.Errorf("expected sig-2, got %s", sigs[0].ID)
		}
		if sigs[0].Quote.SwapURL != "https://jup.ag/swap/USDC-OPENAI" {
			t.Errorf("unexpected quote swap URL: %s", sigs[0].Quote.SwapURL)
		}

		// List with limit 10 returns both (sig2 then sig1)
		allSigs, err := sigRepo.ListByOwner(ctx, "alice", 10)
		if err != nil {
			t.Fatalf("unexpected list all: %v", err)
		}
		if len(allSigs) != 2 {
			t.Fatalf("expected 2 signals, got %d", len(allSigs))
		}
		if allSigs[0].ID != "sig-2" || allSigs[1].ID != "sig-1" {
			t.Errorf("expected descending order sig-2 then sig-1, got %+v", allSigs)
		}
	})
}
