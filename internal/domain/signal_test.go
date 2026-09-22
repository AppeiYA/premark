package domain

import (
	"testing"
	"time"
)

func TestNewSignal(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	sym := Symbol("ANTHROPIC")

	rule := Rule{
		ID:     RuleID("rule-99"),
		Owner:  OwnerID("user-alice"),
		Symbol: sym,
	}

	snap := MarketSnapshot{
		Token: Token{
			Symbol: sym,
			Mint:   "MINT_ANTHROPIC",
		},
		TokenPrice: 950.0,
		MarkPrice:  1000.0,
	}

	quote := Quote{
		Symbol:         sym,
		InputMint:      "USDC",
		OutputMint:     "MINT_ANTHROPIC",
		InAmount:       100 * USDCUnit,
		OutTokens:      0.1,
		PriceImpactBps: 15.0,
		SwapURL:        "https://jup.ag/swap/USDC-ANTHROPIC",
		QuotedAt:       now,
	}

	decision := Decision{
		Trigger:        true,
		APIPremiumBps:  -500.0,
		ExecPremiumBps: -500.0,
	}

	sigID := SignalID("sig-1")
	sig := NewSignal(sigID, rule, snap, quote, decision, now)

	if sig.ID != sigID {
		t.Errorf("expected ID=%s, got %s", sigID, sig.ID)
	}
	if sig.RuleID != rule.ID {
		t.Errorf("expected RuleID=%s, got %s", rule.ID, sig.RuleID)
	}
	if sig.Owner != rule.Owner {
		t.Errorf("expected Owner=%s, got %s", rule.Owner, sig.Owner)
	}
	if sig.Symbol != sym {
		t.Errorf("expected Symbol=%s, got %s", sym, sig.Symbol)
	}
	if sig.TokenPrice != snap.TokenPrice {
		t.Errorf("expected TokenPrice=%v, got %v", snap.TokenPrice, sig.TokenPrice)
	}
	if sig.MarkPrice != snap.MarkPrice {
		t.Errorf("expected MarkPrice=%v, got %v", snap.MarkPrice, sig.MarkPrice)
	}
	if sig.APIPremiumBps != decision.APIPremiumBps {
		t.Errorf("expected APIPremiumBps=%v, got %v", decision.APIPremiumBps, sig.APIPremiumBps)
	}
	if sig.ExecPremiumBps != decision.ExecPremiumBps {
		t.Errorf("expected ExecPremiumBps=%v, got %v", decision.ExecPremiumBps, sig.ExecPremiumBps)
	}
	if sig.Quote.SwapURL != quote.SwapURL {
		t.Errorf("expected Quote.SwapURL=%s, got %s", quote.SwapURL, sig.Quote.SwapURL)
	}
	if !sig.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt=%v, got %v", now, sig.CreatedAt)
	}
}
