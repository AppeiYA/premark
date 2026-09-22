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

func TestRuleService_CreateRule(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	ids := testsupport.NewSeqIDs("rule")

	t.Run("invalid params returns validation error", func(t *testing.T) {
		rulesRepo := memstore.NewRuleRepository()
		source := &testsupport.FakeSource{}
		svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

		p := domain.NewRuleParams{
			Owner:         "",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        100 * domain.USDCUnit,
		}

		_, err := svc.CreateRule(ctx, p)
		if !errors.Is(err, domain.ErrInvalidRule) {
			t.Errorf("expected ErrInvalidRule, got %v", err)
		}
	})

	t.Run("source fetch error returns upstream error", func(t *testing.T) {
		rulesRepo := memstore.NewRuleRepository()
		source := &testsupport.FakeSource{Err: errors.New("upstream timeout")}
		svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

		p := domain.NewRuleParams{
			Owner:         "alice",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        100 * domain.USDCUnit,
		}

		_, err := svc.CreateRule(ctx, p)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("token not found in market returns ErrTokenNotFound", func(t *testing.T) {
		rulesRepo := memstore.NewRuleRepository()
		source := &testsupport.FakeSource{
			Snapshots: []domain.MarketSnapshot{
				testsupport.Snap("OPENAI", 1000, 1000, now),
			},
		}
		svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

		p := domain.NewRuleParams{
			Owner:         "alice",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        100 * domain.USDCUnit,
		}

		_, err := svc.CreateRule(ctx, p)
		if !errors.Is(err, domain.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("repository save error", func(t *testing.T) {
		underlying := memstore.NewRuleRepository()
		failingRepo := testsupport.NewFailingRuleRepo(underlying)
		failingRepo.FailSave = errors.New("db disk full")

		source := &testsupport.FakeSource{
			Snapshots: []domain.MarketSnapshot{
				testsupport.Snap("ANTHROPIC", 950, 1000, now),
			},
		}
		svc := usecase.NewRuleService(failingRepo, source, clock, ids)

		p := domain.NewRuleParams{
			Owner:         "alice",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        100 * domain.USDCUnit,
		}

		_, err := svc.CreateRule(ctx, p)
		if err == nil || err.Error() != "db disk full" {
			t.Errorf("expected db error, got %v", err)
		}
	})

	t.Run("successful rule creation", func(t *testing.T) {
		rulesRepo := memstore.NewRuleRepository()
		source := &testsupport.FakeSource{
			Snapshots: []domain.MarketSnapshot{
				testsupport.Snap("ANTHROPIC", 950, 1000, now),
			},
		}
		svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

		p := domain.NewRuleParams{
			Owner:         "alice",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        100 * domain.USDCUnit,
		}

		rule, err := svc.CreateRule(ctx, p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if rule.ID == "" {
			t.Errorf("expected non-empty rule ID")
		}
		if rule.Owner != "alice" {
			t.Errorf("expected Owner=alice, got %s", rule.Owner)
		}
		if rule.Symbol != "ANTHROPIC" {
			t.Errorf("expected Symbol=ANTHROPIC, got %s", rule.Symbol)
		}
		if !rule.Enabled {
			t.Errorf("expected rule to be enabled")
		}

		// Verify persisted in repo
		persisted, err := rulesRepo.Get(ctx, rule.ID)
		if err != nil {
			t.Fatalf("failed to retrieve saved rule: %v", err)
		}
		if persisted.ID != rule.ID {
			t.Errorf("expected persisted ID %s, got %s", rule.ID, persisted.ID)
		}
	})
}

func TestRuleService_ListRules(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	ids := testsupport.NewSeqIDs("rule")
	rulesRepo := memstore.NewRuleRepository()
	source := &testsupport.FakeSource{}
	svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

	t.Run("empty owner returns ErrInvalidArgument", func(t *testing.T) {
		_, err := svc.ListRules(ctx, "")
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("successful list returns only owner rules", func(t *testing.T) {
		r1 := domain.Rule{ID: "r1", Owner: "alice", Symbol: "OPENAI", CreatedAt: now}
		r2 := domain.Rule{ID: "r2", Owner: "bob", Symbol: "SPACEX", CreatedAt: now}
		r3 := domain.Rule{ID: "r3", Owner: "alice", Symbol: "ANTHROPIC", CreatedAt: now.Add(time.Minute)}
		_ = rulesRepo.Save(ctx, r1)
		_ = rulesRepo.Save(ctx, r2)
		_ = rulesRepo.Save(ctx, r3)

		aliceRules, err := svc.ListRules(ctx, "alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(aliceRules) != 2 {
			t.Fatalf("expected 2 rules for alice, got %d", len(aliceRules))
		}
		if aliceRules[0].ID != "r1" || aliceRules[1].ID != "r3" {
			t.Errorf("unexpected rules returned: %+v", aliceRules)
		}
	})
}

func TestRuleService_SetRuleEnabled(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	ids := testsupport.NewSeqIDs("rule")
	rulesRepo := memstore.NewRuleRepository()
	source := &testsupport.FakeSource{}
	svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

	rule := domain.Rule{ID: "r1", Owner: "alice", Symbol: "OPENAI", Enabled: true, CreatedAt: now}
	_ = rulesRepo.Save(ctx, rule)

	t.Run("empty owner returns ErrInvalidArgument", func(t *testing.T) {
		_, err := svc.SetRuleEnabled(ctx, "", "r1", false)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("rule not found returns ErrRuleNotFound", func(t *testing.T) {
		_, err := svc.SetRuleEnabled(ctx, "alice", "non-existent", false)
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("wrong owner returns ErrRuleNotFound", func(t *testing.T) {
		_, err := svc.SetRuleEnabled(ctx, "bob", "r1", false)
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("successful enable/disable update", func(t *testing.T) {
		updated, err := svc.SetRuleEnabled(ctx, "alice", "r1", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Enabled {
			t.Errorf("expected Enabled=false")
		}

		persisted, _ := rulesRepo.Get(ctx, "r1")
		if persisted.Enabled {
			t.Errorf("expected persisted Enabled=false")
		}

		reEnabled, err := svc.SetRuleEnabled(ctx, "alice", "r1", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reEnabled.Enabled {
			t.Errorf("expected Enabled=true")
		}
	})
}

func TestRuleService_DeleteRule(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewFixedClock(now)
	ids := testsupport.NewSeqIDs("rule")
	rulesRepo := memstore.NewRuleRepository()
	source := &testsupport.FakeSource{}
	svc := usecase.NewRuleService(rulesRepo, source, clock, ids)

	rule := domain.Rule{ID: "r1", Owner: "alice", Symbol: "OPENAI", Enabled: true, CreatedAt: now}
	_ = rulesRepo.Save(ctx, rule)

	t.Run("empty owner returns ErrInvalidArgument", func(t *testing.T) {
		err := svc.DeleteRule(ctx, "", "r1")
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("rule not found returns ErrRuleNotFound", func(t *testing.T) {
		err := svc.DeleteRule(ctx, "alice", "non-existent")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("wrong owner returns ErrRuleNotFound", func(t *testing.T) {
		err := svc.DeleteRule(ctx, "bob", "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound, got %v", err)
		}
	})

	t.Run("successful deletion", func(t *testing.T) {
		err := svc.DeleteRule(ctx, "alice", "r1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = rulesRepo.Get(ctx, "r1")
		if !errors.Is(err, domain.ErrRuleNotFound) {
			t.Errorf("expected ErrRuleNotFound after deletion, got %v", err)
		}
	})
}
