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

func TestEvaluator_EvaluateAll(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	sym := domain.Symbol("ANTHROPIC")

	baseRule := domain.Rule{
		ID:                "rule-1",
		Owner:             "alice",
		Symbol:            sym,
		MaxPremiumBps:     -200,
		Budget:            100 * domain.USDCUnit,
		MaxPriceImpactBps: 100,
		Cooldown:          1 * time.Hour,
		Enabled:           true,
		CreatedAt:         now.Add(-2 * time.Hour),
	}

	validSnap := testsupport.Snap("ANTHROPIC", 950, 1000, now) // -500 bps
	validQuote := testsupport.QuoteFor("ANTHROPIC", 100*domain.USDCUnit, 100.0/950.0, 20.0, now)

	t.Run("rules repo ListEnabled error returns error", func(t *testing.T) {
		ctx := context.Background()
		underlying := memstore.NewRuleRepository()
		failingRules := testsupport.NewFailingRuleRepo(underlying)
		failingRules.FailListEnabled = errors.New("db error")

		eval := usecase.NewEvaluator(
			failingRules,
			testsupport.NewFakeQuoter(),
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		_, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("context cancellation during loop returns ctx.Err()", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already cancelled

		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		eval := usecase.NewEvaluator(
			rulesRepo,
			testsupport.NewFakeQuoter(),
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		_, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("no market data for symbol records skip", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		eval := usecase.NewEvaluator(
			rulesRepo,
			testsupport.NewFakeQuoter(),
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{}) // empty snapshots
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Evaluated != 1 {
			t.Errorf("expected Evaluated=1, got %d", report.Evaluated)
		}
		if len(report.Skips) != 1 {
			t.Fatalf("expected 1 skip, got %d", len(report.Skips))
		}
		if report.Skips[0].Reason != domain.ReasonNoMarketData {
			t.Errorf("expected ReasonNoMarketData, got %s", report.Skips[0].Reason)
		}
	})

	t.Run("precheck fails records skip", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		r := baseRule
		r.MaxPremiumBps = -1000 // -10% threshold, but snapshot has -500 bps (-5%) => above threshold
		_ = rulesRepo.Save(ctx, r)

		eval := usecase.NewEvaluator(
			rulesRepo,
			testsupport.NewFakeQuoter(),
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Skips) != 1 {
			t.Fatalf("expected 1 skip, got %d", len(report.Skips))
		}
		if report.Skips[0].Reason != domain.ReasonPremiumAboveThreshold {
			t.Errorf("expected ReasonPremiumAboveThreshold, got %s", report.Skips[0].Reason)
		}
	})

	t.Run("quoter failure records rule failure", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		quoter := testsupport.NewFakeQuoter()
		quoter.SetError(sym, errors.New("no route found"))

		eval := usecase.NewEvaluator(
			rulesRepo,
			quoter,
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Failures) != 1 {
			t.Fatalf("expected 1 failure, got %d", len(report.Failures))
		}
		if report.Failures[0].RuleID != baseRule.ID {
			t.Errorf("expected failure for rule %s, got %s", baseRule.ID, report.Failures[0].RuleID)
		}
	})

	t.Run("decide fails records skip", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		highImpactQuote := validQuote
		highImpactQuote.PriceImpactBps = 200.0 // 200 > baseRule.MaxPriceImpactBps (100)

		quoter := testsupport.NewFakeQuoter()
		quoter.SetQuote(sym, highImpactQuote)

		eval := usecase.NewEvaluator(
			rulesRepo,
			quoter,
			memstore.NewSignalRepository(),
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Skips) != 1 {
			t.Fatalf("expected 1 skip, got %d", len(report.Skips))
		}
		if report.Skips[0].Reason != domain.ReasonPriceImpactTooHigh {
			t.Errorf("expected ReasonPriceImpactTooHigh, got %s", report.Skips[0].Reason)
		}
	})

	t.Run("signals repo save error records failure", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		quoter := testsupport.NewFakeQuoter()
		quoter.SetQuote(sym, validQuote)

		underlyingSignals := memstore.NewSignalRepository()
		failingSignals := testsupport.NewFailingSignalRepo(underlyingSignals)
		failingSignals.FailSave = errors.New("db write failed")

		eval := usecase.NewEvaluator(
			rulesRepo,
			quoter,
			failingSignals,
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Failures) != 1 {
			t.Fatalf("expected 1 failure, got %d", len(report.Failures))
		}
		if len(report.Signals) != 0 {
			t.Errorf("expected 0 signals on save failure, got %d", len(report.Signals))
		}
	})

	t.Run("rules repo mark triggered error records failure", func(t *testing.T) {
		ctx := context.Background()
		underlyingRules := memstore.NewRuleRepository()
		_ = underlyingRules.Save(ctx, baseRule)

		failingRules := testsupport.NewFailingRuleRepo(underlyingRules)
		failingRules.FailSave = errors.New("failed updating rule last triggered")

		quoter := testsupport.NewFakeQuoter()
		quoter.SetQuote(sym, validQuote)

		signalsRepo := memstore.NewSignalRepository()

		eval := usecase.NewEvaluator(
			failingRules,
			quoter,
			signalsRepo,
			&testsupport.FakeNotifier{},
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Failures) != 1 {
			t.Fatalf("expected 1 failure, got %d", len(report.Failures))
		}
		// Signal is still appended
		if len(report.Signals) != 1 {
			t.Errorf("expected 1 signal created, got %d", len(report.Signals))
		}
	})

	t.Run("successful evaluation creates signal, updates rule, and notifies", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		quoter := testsupport.NewFakeQuoter()
		quoter.SetQuote(sym, validQuote)

		signalsRepo := memstore.NewSignalRepository()
		notifier := &testsupport.FakeNotifier{}
		clock := testsupport.NewFixedClock(now)
		ids := testsupport.NewSeqIDs("sig")

		eval := usecase.NewEvaluator(
			rulesRepo,
			quoter,
			signalsRepo,
			notifier,
			clock,
			ids,
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Evaluated != 1 {
			t.Errorf("expected Evaluated=1, got %d", report.Evaluated)
		}
		if len(report.Signals) != 1 {
			t.Fatalf("expected 1 signal in report, got %d", len(report.Signals))
		}

		sig := report.Signals[0]
		if sig.ID != "sig-1" {
			t.Errorf("expected signal ID sig-1, got %s", sig.ID)
		}
		if sig.Owner != "alice" {
			t.Errorf("expected Owner=alice, got %s", sig.Owner)
		}

		// Verify signal was saved in repo
		savedSigs, err := signalsRepo.ListByOwner(ctx, "alice", 10)
		if err != nil || len(savedSigs) != 1 {
			t.Fatalf("expected 1 signal saved in repo, got %d, err %v", len(savedSigs), err)
		}

		// Verify rule was marked triggered
		updatedRule, err := rulesRepo.Get(ctx, baseRule.ID)
		if err != nil {
			t.Fatalf("failed to get updated rule: %v", err)
		}
		if !updatedRule.LastTriggeredAt.Equal(now) {
			t.Errorf("expected LastTriggeredAt=%v, got %v", now, updatedRule.LastTriggeredAt)
		}

		// Verify notifier was called
		if len(notifier.Signals) != 1 {
			t.Fatalf("expected 1 notification sent, got %d", len(notifier.Signals))
		}
		if notifier.Signals[0].ID != sig.ID {
			t.Errorf("expected notified signal %s, got %s", sig.ID, notifier.Signals[0].ID)
		}
	})

	t.Run("notifier failure is logged and does not prevent signal reporting", func(t *testing.T) {
		ctx := context.Background()
		rulesRepo := memstore.NewRuleRepository()
		_ = rulesRepo.Save(ctx, baseRule)

		quoter := testsupport.NewFakeQuoter()
		quoter.SetQuote(sym, validQuote)

		signalsRepo := memstore.NewSignalRepository()
		notifier := &testsupport.FakeNotifier{Err: errors.New("telegram network error")}

		eval := usecase.NewEvaluator(
			rulesRepo,
			quoter,
			signalsRepo,
			notifier,
			testsupport.NewFixedClock(now),
			testsupport.NewSeqIDs("sig"),
			nil,
		)

		report, err := eval.EvaluateAll(ctx, []domain.MarketSnapshot{validSnap})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Signals) != 1 {
			t.Fatalf("expected 1 signal despite notify failure, got %d", len(report.Signals))
		}
	})
}
