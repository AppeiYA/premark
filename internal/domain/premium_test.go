package domain

import (
	"errors"
	"math"
	"testing"
)

func TestNewPremium(t *testing.T) {
	t.Run("valid prices", func(t *testing.T) {
		p, err := NewPremium(950.0, 1000.0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(p.Ratio()-(-0.05)) > 1e-9 {
			t.Errorf("expected Ratio=-0.05, got %v", p.Ratio())
		}
		if math.Abs(p.Bps()-(-500.0)) > 1e-9 {
			t.Errorf("expected Bps=-500, got %v", p.Bps())
		}
	})

	t.Run("invalid prices", func(t *testing.T) {
		testCases := []struct {
			name  string
			price float64
			mark  float64
		}{
			{"price zero", 0, 100},
			{"price negative", -10, 100},
			{"mark zero", 100, 0},
			{"mark negative", 100, -10},
			{"price NaN", math.NaN(), 100},
			{"mark NaN", 100, math.NaN()},
			{"price Inf", math.Inf(1), 100},
			{"mark Inf", 100, math.Inf(-1)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewPremium(tc.price, tc.mark)
				if !errors.Is(err, ErrInvalidPrice) {
					t.Errorf("expected ErrInvalidPrice, got %v", err)
				}
			})
		}
	})
}

func TestPremium_Label(t *testing.T) {
	tests := []struct {
		name     string
		price    float64
		mark     float64
		expected PremiumLabel
	}{
		{"cheap below threshold", 980.0, 1000.0, LabelCheap}, // -200 bps <= -100 bps
		{"cheap at threshold", 990.0, 1000.0, LabelCheap},    // -100 bps == CheapThresholdBps
		{"fair negative", 995.0, 1000.0, LabelFair},          // -50 bps
		{"fair exact zero", 1000.0, 1000.0, LabelFair},       // 0 bps
		{"fair positive", 1005.0, 1000.0, LabelFair},         // +50 bps
		{"rich at threshold", 1010.0, 1000.0, LabelRich},     // +100 bps == RichThresholdBps
		{"rich above threshold", 1020.0, 1000.0, LabelRich},  // +200 bps >= +100 bps
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPremium(tt.price, tt.mark)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Label() != tt.expected {
				t.Errorf("expected label %s for bps %v, got %s", tt.expected, p.Bps(), p.Label())
			}
		})
	}
}
