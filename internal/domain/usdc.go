package domain

import (
	"fmt"
	"math"
)

// USDC is an amount in micro-USDC (6 decimals). Never use float for stored money.
type USDC int64

const USDCUnit USDC = 1_000_000

// USDCFromFloat rounds to 6 decimals (half away from zero).
// Errors (ErrInvalidArgument, wrapped) on NaN, Inf, negative.
func USDCFromFloat(v float64) (USDC, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, fmt.Errorf("%w: invalid usdc amount %v", ErrInvalidArgument, v)
	}
	rounded := math.Round(v * float64(USDCUnit))
	return USDC(rounded), nil
}

func (u USDC) Float64() float64 {
	return float64(u) / float64(USDCUnit)
}
