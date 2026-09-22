package domain

import (
	"fmt"
	"math"
)

type PremiumLabel string

const (
	LabelCheap PremiumLabel = "cheap"
	LabelFair  PremiumLabel = "fair"
	LabelRich  PremiumLabel = "rich"
)

const (
	CheapThresholdBps = -100.0 // premium <= this  => cheap (inclusive)
	RichThresholdBps  = 100.0  // premium >= this  => rich  (inclusive)
)

type Premium struct{ ratio float64 } // unexported field, so it cannot be built with invalid state

// NewPremium = price/mark - 1. ErrInvalidPrice if price <= 0, mark <= 0, NaN or Inf.
func NewPremium(price, mark float64) (Premium, error) {
	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 ||
		math.IsNaN(mark) || math.IsInf(mark, 0) || mark <= 0 {
		return Premium{}, fmt.Errorf("%w: invalid price (%v) or mark (%v)", ErrInvalidPrice, price, mark)
	}
	return Premium{ratio: (price / mark) - 1.0}, nil
}

func (p Premium) Ratio() float64 {
	return p.ratio
}

func (p Premium) Bps() float64 {
	return p.ratio * 10000.0
}

func (p Premium) Label() PremiumLabel {
	bps := p.Bps()
	if bps <= CheapThresholdBps {
		return LabelCheap
	}
	if bps >= RichThresholdBps {
		return LabelRich
	}
	return LabelFair
}
