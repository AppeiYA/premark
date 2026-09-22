package domain

import (
	"fmt"
	"math"
	"time"
)

// Quote is a live executable BUY quote (USDC in, token out).
// OutTokens is in UI units: already divided by 10^decimals AND multiplied by the ScaledUiAmount
// multiplier if the mint has one. The adapter does that normalisation, the domain never sees raw units.
type Quote struct {
	Symbol         Symbol
	InputMint      string
	OutputMint     string
	InAmount       USDC
	OutTokens      float64
	PriceImpactBps float64
	SwapURL        string
	QuotedAt       time.Time
}

// PricePerToken = InAmount.Float64() / OutTokens. ErrQuoteUnavailable (wrapped) if OutTokens <= 0, NaN or Inf.
func (q Quote) PricePerToken() (float64, error) {
	if math.IsNaN(q.OutTokens) || math.IsInf(q.OutTokens, 0) || q.OutTokens <= 0 {
		return 0, fmt.Errorf("%w: invalid out tokens %v", ErrQuoteUnavailable, q.OutTokens)
	}
	return q.InAmount.Float64() / q.OutTokens, nil
}
