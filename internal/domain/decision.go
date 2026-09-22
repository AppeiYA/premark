package domain

import (
	"math"
	"time"
)

type SkipReason string

const (
	ReasonDisabled                  SkipReason = "disabled"
	ReasonCooldownActive            SkipReason = "cooldown_active"
	ReasonNoMarketData              SkipReason = "no_market_data"
	ReasonSnapshotStale             SkipReason = "snapshot_stale"
	ReasonInvalidPrice              SkipReason = "invalid_price"
	ReasonPremiumAboveThreshold     SkipReason = "premium_above_threshold"
	ReasonInvalidQuote              SkipReason = "invalid_quote"
	ReasonPriceDivergence           SkipReason = "price_divergence"
	ReasonPriceImpactTooHigh        SkipReason = "price_impact_too_high"
	ReasonExecPremiumAboveThreshold SkipReason = "exec_premium_above_threshold"
)

type Decision struct {
	Trigger        bool
	Reason         SkipReason // empty when Trigger is true
	APIPremiumBps  float64    // 0 when not computed
	ExecPremiumBps float64    // 0 when the quote stage was not reached
}

// Precheck is stage 1: pure, no network. Trigger==true means "worth fetching a quote".
// Check order (first failing check wins):
//  1. !r.Enabled                                           -> ReasonDisabled
//  2. cooldown active                                      -> ReasonCooldownActive
//     active = !r.LastTriggeredAt.IsZero() && now.Sub(r.LastTriggeredAt) < r.Cooldown
//     (exactly at the cooldown boundary the rule is NOT active)
//  3. now.Sub(s.ObservedAt) > MaxSnapshotAge               -> ReasonSnapshotStale (exactly equal is fine)
//  4. s.Premium() errors                                   -> ReasonInvalidPrice
//  5. premiumBps > float64(r.MaxPremiumBps)                -> ReasonPremiumAboveThreshold (equal passes)
//
// On success APIPremiumBps is set.
func (r Rule) Precheck(s MarketSnapshot, now time.Time) Decision {
	if !r.Enabled {
		return Decision{Trigger: false, Reason: ReasonDisabled}
	}

	if !r.LastTriggeredAt.IsZero() && now.Sub(r.LastTriggeredAt) < r.Cooldown {
		return Decision{Trigger: false, Reason: ReasonCooldownActive}
	}

	if now.Sub(s.ObservedAt) > MaxSnapshotAge {
		return Decision{Trigger: false, Reason: ReasonSnapshotStale}
	}

	prem, err := s.Premium()
	if err != nil {
		return Decision{Trigger: false, Reason: ReasonInvalidPrice}
	}

	apiBps := prem.Bps()
	if apiBps > float64(r.MaxPremiumBps) {
		return Decision{Trigger: false, Reason: ReasonPremiumAboveThreshold, APIPremiumBps: apiBps}
	}

	return Decision{Trigger: true, APIPremiumBps: apiBps}
}

// Decide is stage 2. It first runs Precheck (a failing Precheck is returned as-is), then:
//  6. quote invalid                                        -> ReasonInvalidQuote
//     invalid = q.Symbol != r.Symbol || q.InAmount != r.Budget || q.PricePerToken() errors
//  7. divergence: abs(quotePrice/s.TokenPrice - 1)*10000 > DivergenceLimitBps
//     -> ReasonPriceDivergence
//  8. q.PriceImpactBps > float64(r.MaxPriceImpactBps)      -> ReasonPriceImpactTooHigh
//  9. execPremiumBps := (quotePrice/s.MarkPrice - 1)*10000
//     execPremiumBps > float64(r.MaxPremiumBps)            -> ReasonExecPremiumAboveThreshold (equal passes)
//  10. otherwise Trigger = true with APIPremiumBps and ExecPremiumBps set.
//
// Once the quote stage is reached, ExecPremiumBps is populated on skips too (except reason 6).
func (r Rule) Decide(s MarketSnapshot, q Quote, now time.Time) Decision {
	d := r.Precheck(s, now)
	if !d.Trigger {
		return d
	}

	quotePrice, err := q.PricePerToken()
	if q.Symbol != r.Symbol || q.InAmount != r.Budget || err != nil {
		return Decision{
			Trigger:        false,
			Reason:         ReasonInvalidQuote,
			APIPremiumBps:  d.APIPremiumBps,
			ExecPremiumBps: 0,
		}
	}

	execPremiumBps := (quotePrice/s.MarkPrice - 1.0) * 10000.0

	divergence := math.Abs(quotePrice/s.TokenPrice-1.0) * 10000.0
	if divergence > DivergenceLimitBps {
		return Decision{
			Trigger:        false,
			Reason:         ReasonPriceDivergence,
			APIPremiumBps:  d.APIPremiumBps,
			ExecPremiumBps: execPremiumBps,
		}
	}

	if q.PriceImpactBps > float64(r.MaxPriceImpactBps) {
		return Decision{
			Trigger:        false,
			Reason:         ReasonPriceImpactTooHigh,
			APIPremiumBps:  d.APIPremiumBps,
			ExecPremiumBps: execPremiumBps,
		}
	}

	if execPremiumBps > float64(r.MaxPremiumBps) {
		return Decision{
			Trigger:        false,
			Reason:         ReasonExecPremiumAboveThreshold,
			APIPremiumBps:  d.APIPremiumBps,
			ExecPremiumBps: execPremiumBps,
		}
	}

	return Decision{
		Trigger:        true,
		Reason:         "",
		APIPremiumBps:  d.APIPremiumBps,
		ExecPremiumBps: execPremiumBps,
	}
}
