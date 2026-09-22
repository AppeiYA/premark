package domain

import (
	"math"
	"testing"
	"time"
)

func TestRule_Precheck(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	sym := Symbol("ANTHROPIC")

	baseRule := Rule{
		ID:                RuleID("rule-1"),
		Owner:             OwnerID("owner-1"),
		Symbol:            sym,
		MaxPremiumBps:     -200, // <= -2% below mark
		Budget:            100 * USDCUnit,
		MaxPriceImpactBps: 100,
		Cooldown:          1 * time.Hour,
		Enabled:           true,
		CreatedAt:         now.Add(-2 * time.Hour),
	}

	validSnapshot := MarketSnapshot{
		Token: Token{
			Symbol: sym,
			Mint:   "MINT_ANTHROPIC",
		},
		TokenPrice: 970.0,
		MarkPrice:  1000.0, // premium = (970/1000 - 1) = -300 bps (<= -200 bps)
		ObservedAt: now.Add(-1 * time.Minute),
	}

	t.Run("rule disabled", func(t *testing.T) {
		r := baseRule
		r.Enabled = false
		d := r.Precheck(validSnapshot, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false, got true")
		}
		if d.Reason != ReasonDisabled {
			t.Errorf("expected Reason=%s, got %s", ReasonDisabled, d.Reason)
		}
	})

	t.Run("cooldown active", func(t *testing.T) {
		r := baseRule
		r.LastTriggeredAt = now.Add(-30 * time.Minute) // 30m < 1h cooldown
		d := r.Precheck(validSnapshot, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false, got true")
		}
		if d.Reason != ReasonCooldownActive {
			t.Errorf("expected Reason=%s, got %s", ReasonCooldownActive, d.Reason)
		}
	})

	t.Run("cooldown exactly expired", func(t *testing.T) {
		r := baseRule
		r.LastTriggeredAt = now.Add(-1 * time.Hour) // exactly 1h == cooldown, NOT active
		d := r.Precheck(validSnapshot, now)
		if !d.Trigger {
			t.Errorf("expected Trigger=true at cooldown boundary, got false with reason %s", d.Reason)
		}
	})

	t.Run("snapshot stale - over 10 minutes", func(t *testing.T) {
		r := baseRule
		snap := validSnapshot
		snap.ObservedAt = now.Add(-10*time.Minute - 1*time.Second)
		d := r.Precheck(snap, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false for stale snapshot, got true")
		}
		if d.Reason != ReasonSnapshotStale {
			t.Errorf("expected Reason=%s, got %s", ReasonSnapshotStale, d.Reason)
		}
	})

	t.Run("snapshot exactly at max age is valid", func(t *testing.T) {
		r := baseRule
		snap := validSnapshot
		snap.ObservedAt = now.Add(-MaxSnapshotAge) // exactly 10 min
		d := r.Precheck(snap, now)
		if !d.Trigger {
			t.Errorf("expected Trigger=true at MaxSnapshotAge boundary, got false with reason %s", d.Reason)
		}
	})

	t.Run("invalid price in snapshot", func(t *testing.T) {
		invalidPrices := []struct {
			name  string
			price float64
			mark  float64
		}{
			{"zero token price", 0, 1000},
			{"negative token price", -10, 1000},
			{"zero mark price", 970, 0},
			{"negative mark price", 970, -100},
			{"NaN token price", math.NaN(), 1000},
			{"Inf mark price", 970, math.Inf(1)},
		}

		for _, tc := range invalidPrices {
			t.Run(tc.name, func(t *testing.T) {
				snap := validSnapshot
				snap.TokenPrice = tc.price
				snap.MarkPrice = tc.mark
				d := baseRule.Precheck(snap, now)
				if d.Trigger {
					t.Errorf("expected Trigger=false, got true")
				}
				if d.Reason != ReasonInvalidPrice {
					t.Errorf("expected Reason=%s, got %s", ReasonInvalidPrice, d.Reason)
				}
			})
		}
	})

	t.Run("premium above threshold", func(t *testing.T) {
		snap := validSnapshot
		snap.TokenPrice = 990.0
		snap.MarkPrice = 1000.0 // premium = -100 bps > -200 bps
		d := baseRule.Precheck(snap, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false, got true")
		}
		if d.Reason != ReasonPremiumAboveThreshold {
			t.Errorf("expected Reason=%s, got %s", ReasonPremiumAboveThreshold, d.Reason)
		}
		if math.Abs(d.APIPremiumBps-(-100.0)) > 1e-6 {
			t.Errorf("expected APIPremiumBps=-100, got %v", d.APIPremiumBps)
		}
	})

	t.Run("premium equal to threshold passes", func(t *testing.T) {
		snap := validSnapshot
		snap.TokenPrice = 980.0
		snap.MarkPrice = 1000.0 // premium = -200 bps == MaxPremiumBps
		d := baseRule.Precheck(snap, now)
		if !d.Trigger {
			t.Errorf("expected Trigger=true when premium == MaxPremiumBps, got false with reason %s", d.Reason)
		}
		if math.Abs(d.APIPremiumBps-(-200.0)) > 1e-6 {
			t.Errorf("expected APIPremiumBps=-200, got %v", d.APIPremiumBps)
		}
	})

	t.Run("premium below threshold triggers", func(t *testing.T) {
		d := baseRule.Precheck(validSnapshot, now)
		if !d.Trigger {
			t.Errorf("expected Trigger=true, got false with reason %s", d.Reason)
		}
		if d.Reason != "" {
			t.Errorf("expected empty reason on trigger, got %s", d.Reason)
		}
		if math.Abs(d.APIPremiumBps-(-300.0)) > 1e-6 {
			t.Errorf("expected APIPremiumBps=-300, got %v", d.APIPremiumBps)
		}
	})
}

func TestRule_Decide(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	sym := Symbol("ANTHROPIC")

	rule := Rule{
		ID:                RuleID("rule-1"),
		Owner:             OwnerID("owner-1"),
		Symbol:            sym,
		MaxPremiumBps:     -200, // <= -2%
		Budget:            100 * USDCUnit,
		MaxPriceImpactBps: 100, // 1%
		Cooldown:          1 * time.Hour,
		Enabled:           true,
		CreatedAt:         now.Add(-2 * time.Hour),
	}

	snap := MarketSnapshot{
		Token: Token{
			Symbol: sym,
			Mint:   "MINT_ANTHROPIC",
		},
		TokenPrice: 970.0,
		MarkPrice:  1000.0,
		ObservedAt: now.Add(-1 * time.Minute),
	}

	// 100 USDC / (100 / 970) tokens => quote price = 970.0
	validQuote := Quote{
		Symbol:         sym,
		InputMint:      "USDC",
		OutputMint:     "MINT_ANTHROPIC",
		InAmount:       100 * USDCUnit,
		OutTokens:      100.0 / 970.0,
		PriceImpactBps: 20.0,
		SwapURL:        "https://jup.ag/swap/USDC-ANTHROPIC",
		QuotedAt:       now,
	}

	t.Run("failing precheck returned directly", func(t *testing.T) {
		r := rule
		r.Enabled = false
		d := r.Decide(snap, validQuote, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonDisabled {
			t.Errorf("expected Reason=%s, got %s", ReasonDisabled, d.Reason)
		}
	})

	t.Run("invalid quote - symbol mismatch", func(t *testing.T) {
		q := validQuote
		q.Symbol = Symbol("SOLANA")
		d := rule.Decide(snap, q, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonInvalidQuote {
			t.Errorf("expected Reason=%s, got %s", ReasonInvalidQuote, d.Reason)
		}
		if d.ExecPremiumBps != 0 {
			t.Errorf("expected ExecPremiumBps=0 on invalid quote, got %v", d.ExecPremiumBps)
		}
	})

	t.Run("invalid quote - budget mismatch", func(t *testing.T) {
		q := validQuote
		q.InAmount = 50 * USDCUnit
		d := rule.Decide(snap, q, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonInvalidQuote {
			t.Errorf("expected Reason=%s, got %s", ReasonInvalidQuote, d.Reason)
		}
	})

	t.Run("invalid quote - zero or negative outTokens", func(t *testing.T) {
		for _, out := range []float64{0, -1, math.NaN(), math.Inf(1)} {
			q := validQuote
			q.OutTokens = out
			d := rule.Decide(snap, q, now)
			if d.Trigger {
				t.Errorf("expected Trigger=false for outTokens=%v", out)
			}
			if d.Reason != ReasonInvalidQuote {
				t.Errorf("expected Reason=%s, got %s", ReasonInvalidQuote, d.Reason)
			}
		}
	})

	t.Run("price divergence exceeds 10% (1000 bps)", func(t *testing.T) {
		q := validQuote
		// quotePrice = 100 / (100 / 1100) = 1100.0
		// divergence = |1100 / 970 - 1| * 10000 = 1340 bps > 1000 bps
		q.OutTokens = 100.0 / 1100.0
		d := rule.Decide(snap, q, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonPriceDivergence {
			t.Errorf("expected Reason=%s, got %s", ReasonPriceDivergence, d.Reason)
		}
		if d.ExecPremiumBps == 0 {
			t.Errorf("expected ExecPremiumBps to be populated on price divergence skip")
		}
	})

	t.Run("price impact too high", func(t *testing.T) {
		q := validQuote
		q.PriceImpactBps = 150.0 // 150 > rule.MaxPriceImpactBps (100)
		d := rule.Decide(snap, q, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonPriceImpactTooHigh {
			t.Errorf("expected Reason=%s, got %s", ReasonPriceImpactTooHigh, d.Reason)
		}
		if d.ExecPremiumBps == 0 {
			t.Errorf("expected ExecPremiumBps to be populated on price impact skip")
		}
	})

	t.Run("executable premium above threshold", func(t *testing.T) {
		q := validQuote
		// quote price = 985, mark price = 1000 => exec premium = -150 bps > -200 bps
		q.OutTokens = 100.0 / 985.0
		d := rule.Decide(snap, q, now)
		if d.Trigger {
			t.Errorf("expected Trigger=false")
		}
		if d.Reason != ReasonExecPremiumAboveThreshold {
			t.Errorf("expected Reason=%s, got %s", ReasonExecPremiumAboveThreshold, d.Reason)
		}
		if math.Abs(d.ExecPremiumBps-(-150.0)) > 1e-4 {
			t.Errorf("expected ExecPremiumBps=-150, got %v", d.ExecPremiumBps)
		}
	})

	t.Run("executable premium equal to threshold passes", func(t *testing.T) {
		q := validQuote
		// quote price = 980, mark price = 1000 => exec premium = -200 bps == MaxPremiumBps
		q.OutTokens = 100.0 / 980.0
		d := rule.Decide(snap, q, now)
		if !d.Trigger {
			t.Errorf("expected Trigger=true when exec premium == MaxPremiumBps, got %s", d.Reason)
		}
		if math.Abs(d.ExecPremiumBps-(-200.0)) > 1e-4 {
			t.Errorf("expected ExecPremiumBps=-200, got %v", d.ExecPremiumBps)
		}
	})

	t.Run("all criteria met triggers signal", func(t *testing.T) {
		d := rule.Decide(snap, validQuote, now)
		if !d.Trigger {
			t.Fatalf("expected Trigger=true, got false with reason %s", d.Reason)
		}
		if d.Reason != "" {
			t.Errorf("expected empty reason on trigger, got %s", d.Reason)
		}
		if math.Abs(d.APIPremiumBps-(-300.0)) > 1e-6 {
			t.Errorf("expected APIPremiumBps=-300, got %v", d.APIPremiumBps)
		}
		// quote price = 970, mark = 1000 => exec premium = -300 bps
		if math.Abs(d.ExecPremiumBps-(-300.0)) > 1e-4 {
			t.Errorf("expected ExecPremiumBps=-300, got %v", d.ExecPremiumBps)
		}
	})
}
