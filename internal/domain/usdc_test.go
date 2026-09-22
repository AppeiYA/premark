package domain

import (
	"errors"
	"math"
	"testing"
)

func TestUSDC(t *testing.T) {
	t.Run("USDCFromFloat valid and rounding", func(t *testing.T) {
		tests := []struct {
			input    float64
			expected USDC
		}{
			{0.0, 0},
			{1.0, 1_000_000},
			{0.000001, 1},
			{12.345678, 12_345_678},
			{12.3456784, 12_345_678}, // rounds down
			{12.3456785, 12_345_679}, // rounds up (half away from zero)
			{50000.0, 50_000_000_000},
		}

		for _, tt := range tests {
			u, err := USDCFromFloat(tt.input)
			if err != nil {
				t.Errorf("unexpected error for %v: %v", tt.input, err)
			}
			if u != tt.expected {
				t.Errorf("for %v expected USDC %d, got %d", tt.input, tt.expected, u)
			}
		}
	})

	t.Run("USDCFromFloat invalid values", func(t *testing.T) {
		invalid := []float64{
			-0.000001,
			-1.0,
			math.NaN(),
			math.Inf(1),
			math.Inf(-1),
		}

		for _, v := range invalid {
			_, err := USDCFromFloat(v)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("expected ErrInvalidArgument for %v, got %v", v, err)
			}
		}
	})

	t.Run("Float64 method", func(t *testing.T) {
		tests := []struct {
			usdc     USDC
			expected float64
		}{
			{0, 0.0},
			{1_000_000, 1.0},
			{12_345_678, 12.345678},
			{50_000_000_000, 50000.0},
		}

		for _, tt := range tests {
			if math.Abs(tt.usdc.Float64()-tt.expected) > 1e-9 {
				t.Errorf("for USDC %d expected Float64=%v, got %v", tt.usdc, tt.expected, tt.usdc.Float64())
			}
		}
	})
}
