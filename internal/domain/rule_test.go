package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewRule(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	ruleID := RuleID("rule-123")

	t.Run("valid parameters with defaults", func(t *testing.T) {
		p := NewRuleParams{
			Owner:         "user-1",
			Symbol:        "ANTHROPIC",
			MaxPremiumBps: -200,
			Budget:        500 * USDCUnit,
		}

		r, err := NewRule(ruleID, p, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.ID != ruleID {
			t.Errorf("expected ID=%s, got %s", ruleID, r.ID)
		}
		if r.Owner != "user-1" {
			t.Errorf("expected Owner=user-1, got %s", r.Owner)
		}
		if r.Symbol != "ANTHROPIC" {
			t.Errorf("expected Symbol=ANTHROPIC, got %s", r.Symbol)
		}
		if r.MaxPremiumBps != -200 {
			t.Errorf("expected MaxPremiumBps=-200, got %d", r.MaxPremiumBps)
		}
		if r.Budget != 500*USDCUnit {
			t.Errorf("expected Budget=%d, got %d", 500*USDCUnit, r.Budget)
		}
		if r.MaxPriceImpactBps != DefaultMaxPriceImpactBps {
			t.Errorf("expected default MaxPriceImpactBps=%d, got %d", DefaultMaxPriceImpactBps, r.MaxPriceImpactBps)
		}
		if r.Cooldown != DefaultCooldown {
			t.Errorf("expected default Cooldown=%v, got %v", DefaultCooldown, r.Cooldown)
		}
		if !r.Enabled {
			t.Errorf("expected rule to be enabled by default")
		}
		if !r.CreatedAt.Equal(now) {
			t.Errorf("expected CreatedAt=%v, got %v", now, r.CreatedAt)
		}
		if !r.LastTriggeredAt.IsZero() {
			t.Errorf("expected LastTriggeredAt to be zero")
		}
	})

	t.Run("valid parameters with custom values and boundaries", func(t *testing.T) {
		p := NewRuleParams{
			Owner:             "  user-trimmed  ",
			Symbol:            "openai",
			MaxPremiumBps:     MinMaxPremiumBps,
			Budget:            MinBudget,
			MaxPriceImpactBps: MinPriceImpactBps,
			Cooldown:          MinCooldown,
		}

		r, err := NewRule(ruleID, p, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Owner != "user-trimmed" {
			t.Errorf("expected owner to be trimmed to 'user-trimmed', got %q", r.Owner)
		}
		if r.Symbol != "OPENAI" {
			t.Errorf("expected symbol uppercase 'OPENAI', got %s", r.Symbol)
		}
		if r.MaxPremiumBps != MinMaxPremiumBps {
			t.Errorf("expected MaxPremiumBps=%d, got %d", MinMaxPremiumBps, r.MaxPremiumBps)
		}
		if r.Budget != MinBudget {
			t.Errorf("expected Budget=%d, got %d", MinBudget, r.Budget)
		}
		if r.MaxPriceImpactBps != MinPriceImpactBps {
			t.Errorf("expected MaxPriceImpactBps=%d, got %d", MinPriceImpactBps, r.MaxPriceImpactBps)
		}
		if r.Cooldown != MinCooldown {
			t.Errorf("expected Cooldown=%v, got %v", MinCooldown, r.Cooldown)
		}

		// Upper boundaries
		pMax := NewRuleParams{
			Owner:             "user-2",
			Symbol:            "SPACEX",
			MaxPremiumBps:     MaxMaxPremiumBps,
			Budget:            MaxBudget,
			MaxPriceImpactBps: MaxPriceImpactBpsLimit,
			Cooldown:          MaxCooldown,
		}

		rMax, err := NewRule(ruleID, pMax, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rMax.MaxPremiumBps != MaxMaxPremiumBps {
			t.Errorf("expected MaxPremiumBps=%d, got %d", MaxMaxPremiumBps, rMax.MaxPremiumBps)
		}
		if rMax.Budget != MaxBudget {
			t.Errorf("expected Budget=%d, got %d", MaxBudget, rMax.Budget)
		}
		if rMax.MaxPriceImpactBps != MaxPriceImpactBpsLimit {
			t.Errorf("expected MaxPriceImpactBps=%d, got %d", MaxPriceImpactBpsLimit, rMax.MaxPriceImpactBps)
		}
		if rMax.Cooldown != MaxCooldown {
			t.Errorf("expected Cooldown=%v, got %v", MaxCooldown, rMax.Cooldown)
		}
	})

	t.Run("owner validation", func(t *testing.T) {
		testCases := []struct {
			name  string
			owner string
		}{
			{"empty owner", ""},
			{"whitespace owner", "   "},
			{"exceeds max length", strings.Repeat("a", MaxOwnerLength+1)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				p := NewRuleParams{
					Owner:         tc.owner,
					Symbol:        "ANTHROPIC",
					MaxPremiumBps: -200,
					Budget:        100 * USDCUnit,
				}
				_, err := NewRule(ruleID, p, now)
				if !errors.Is(err, ErrInvalidRule) {
					t.Errorf("expected ErrInvalidRule, got %v", err)
				}
			})
		}
	})

	t.Run("symbol validation", func(t *testing.T) {
		p := NewRuleParams{
			Owner:         "user-1",
			Symbol:        "A", // too short
			MaxPremiumBps: -200,
			Budget:        100 * USDCUnit,
		}
		_, err := NewRule(ruleID, p, now)
		if !errors.Is(err, ErrInvalidRule) {
			t.Errorf("expected ErrInvalidRule, got %v", err)
		}
		if !errors.Is(err, ErrInvalidSymbol) {
			t.Errorf("expected wrapped ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("max premium bps validation", func(t *testing.T) {
		outOfRange := []int{MinMaxPremiumBps - 1, MaxMaxPremiumBps + 1}
		for _, bps := range outOfRange {
			p := NewRuleParams{
				Owner:         "user-1",
				Symbol:        "ANTHROPIC",
				MaxPremiumBps: bps,
				Budget:        100 * USDCUnit,
			}
			_, err := NewRule(ruleID, p, now)
			if !errors.Is(err, ErrInvalidRule) {
				t.Errorf("expected ErrInvalidRule for bps=%d, got %v", bps, err)
			}
		}
	})

	t.Run("budget validation", func(t *testing.T) {
		outOfRange := []USDC{MinBudget - 1, MaxBudget + 1}
		for _, budget := range outOfRange {
			p := NewRuleParams{
				Owner:         "user-1",
				Symbol:        "ANTHROPIC",
				MaxPremiumBps: -200,
				Budget:        budget,
			}
			_, err := NewRule(ruleID, p, now)
			if !errors.Is(err, ErrInvalidRule) {
				t.Errorf("expected ErrInvalidRule for budget=%d, got %v", budget, err)
			}
		}
	})

	t.Run("max price impact validation", func(t *testing.T) {
		outOfRange := []int{MinPriceImpactBps - 1, MaxPriceImpactBpsLimit + 1}
		for _, impact := range outOfRange {
			if impact == 0 {
				continue // 0 is special-cased to default
			}
			p := NewRuleParams{
				Owner:             "user-1",
				Symbol:            "ANTHROPIC",
				MaxPremiumBps:     -200,
				Budget:            100 * USDCUnit,
				MaxPriceImpactBps: impact,
			}
			_, err := NewRule(ruleID, p, now)
			if !errors.Is(err, ErrInvalidRule) {
				t.Errorf("expected ErrInvalidRule for impact=%d, got %v", impact, err)
			}
		}
	})

	t.Run("cooldown validation", func(t *testing.T) {
		outOfRange := []time.Duration{MinCooldown - 1*time.Second, MaxCooldown + 1*time.Second}
		for _, cd := range outOfRange {
			p := NewRuleParams{
				Owner:         "user-1",
				Symbol:        "ANTHROPIC",
				MaxPremiumBps: -200,
				Budget:        100 * USDCUnit,
				Cooldown:      cd,
			}
			_, err := NewRule(ruleID, p, now)
			if !errors.Is(err, ErrInvalidRule) {
				t.Errorf("expected ErrInvalidRule for cooldown=%v, got %v", cd, err)
			}
		}
	})
}

func TestRule_Methods(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	rule := Rule{
		ID:        RuleID("rule-1"),
		Owner:     OwnerID("alice"),
		Symbol:    Symbol("OPENAI"),
		Enabled:   true,
		CreatedAt: now,
	}

	t.Run("OwnedBy", func(t *testing.T) {
		if !rule.OwnedBy("alice") {
			t.Errorf("expected OwnedBy('alice')=true")
		}
		if rule.OwnedBy("bob") {
			t.Errorf("expected OwnedBy('bob')=false")
		}
	})

	t.Run("WithEnabled", func(t *testing.T) {
		disabled := rule.WithEnabled(false)
		if disabled.Enabled {
			t.Errorf("expected Enabled=false")
		}
		if !rule.Enabled {
			t.Errorf("original rule should remain immutable")
		}
	})

	t.Run("MarkTriggered", func(t *testing.T) {
		triggerTime := now.Add(5 * time.Minute)
		triggered := rule.MarkTriggered(triggerTime)
		if !triggered.LastTriggeredAt.Equal(triggerTime) {
			t.Errorf("expected LastTriggeredAt=%v, got %v", triggerTime, triggered.LastTriggeredAt)
		}
		if !rule.LastTriggeredAt.IsZero() {
			t.Errorf("original rule should remain unmodified")
		}
	})
}
