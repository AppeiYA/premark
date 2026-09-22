package domain

import (
	"errors"
	"math"
	"testing"
)

func TestQuote_PricePerToken(t *testing.T) {
	t.Run("valid quote", func(t *testing.T) {
		q := Quote{
			InAmount:  1000 * USDCUnit, // $1,000
			OutTokens: 2.0,             // 2 tokens => $500/token
		}
		price, err := q.PricePerToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(price-500.0) > 1e-9 {
			t.Errorf("expected price 500.0, got %v", price)
		}
	})

	t.Run("invalid out tokens", func(t *testing.T) {
		invalidOuts := []struct {
			name string
			out  float64
		}{
			{"zero out tokens", 0},
			{"negative out tokens", -1.5},
			{"NaN out tokens", math.NaN()},
			{"Inf out tokens", math.Inf(1)},
		}

		for _, tc := range invalidOuts {
			t.Run(tc.name, func(t *testing.T) {
				q := Quote{
					InAmount:  100 * USDCUnit,
					OutTokens: tc.out,
				}
				_, err := q.PricePerToken()
				if !errors.Is(err, ErrQuoteUnavailable) {
					t.Errorf("expected ErrQuoteUnavailable for %s, got %v", tc.name, err)
				}
			})
		}
	})
}
