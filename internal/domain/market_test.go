package domain

import (
	"errors"
	"math"
	"testing"
)

func TestMarketSnapshot_Premium(t *testing.T) {
	t.Run("valid snapshot prices", func(t *testing.T) {
		snap := MarketSnapshot{
			Token:      Token{Symbol: Symbol("ANTHROPIC")},
			TokenPrice: 900.0,
			MarkPrice:  1000.0,
		}
		prem, err := snap.Premium()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(prem.Bps()-(-1000.0)) > 1e-9 {
			t.Errorf("expected -1000 bps, got %v", prem.Bps())
		}
	})

	t.Run("invalid snapshot prices", func(t *testing.T) {
		snap := MarketSnapshot{
			Token:      Token{Symbol: Symbol("ANTHROPIC")},
			TokenPrice: -10.0,
			MarkPrice:  1000.0,
		}
		_, err := snap.Premium()
		if !errors.Is(err, ErrInvalidPrice) {
			t.Errorf("expected ErrInvalidPrice, got %v", err)
		}
	})
}
