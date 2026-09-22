package domain

import "time"

type SignalID string

type Signal struct {
	ID             SignalID
	RuleID         RuleID
	Owner          OwnerID
	Symbol         Symbol
	TokenPrice     float64
	MarkPrice      float64
	APIPremiumBps  float64
	ExecPremiumBps float64
	Quote          Quote
	CreatedAt      time.Time
}

// NewSignal copies from the rule, snapshot, quote and decision.
func NewSignal(id SignalID, r Rule, s MarketSnapshot, q Quote, d Decision, now time.Time) Signal {
	return Signal{
		ID:             id,
		RuleID:         r.ID,
		Owner:          r.Owner,
		Symbol:         r.Symbol,
		TokenPrice:     s.TokenPrice,
		MarkPrice:      s.MarkPrice,
		APIPremiumBps:  d.APIPremiumBps,
		ExecPremiumBps: d.ExecPremiumBps,
		Quote:          q,
		CreatedAt:      now,
	}
}
