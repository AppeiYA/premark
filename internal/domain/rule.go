package domain

import (
	"fmt"
	"strings"
	"time"
)

type RuleID string
type OwnerID string

type Rule struct {
	ID                RuleID
	Owner             OwnerID
	Symbol            Symbol
	MaxPremiumBps     int // trigger when premium <= this. e.g. -200 means "2% or more below mark"
	Budget            USDC
	MaxPriceImpactBps int
	Cooldown          time.Duration
	Enabled           bool
	CreatedAt         time.Time
	LastTriggeredAt   time.Time // zero value = never triggered
}

// NewRuleParams holds raw, unvalidated input. Symbol and Owner are raw strings.
// Zero MaxPriceImpactBps and zero Cooldown mean "use default".
type NewRuleParams struct {
	Owner             string
	Symbol            string
	MaxPremiumBps     int
	Budget            USDC
	MaxPriceImpactBps int
	Cooldown          time.Duration
}

const (
	MinBudget                = 1 * USDCUnit
	MaxBudget                = 10_000 * USDCUnit
	MinMaxPremiumBps         = -5000
	MaxMaxPremiumBps         = 500
	MinPriceImpactBps        = 1
	MaxPriceImpactBpsLimit   = 1000
	DefaultMaxPriceImpactBps = 100
	MinCooldown              = 1 * time.Minute
	MaxCooldown              = 7 * 24 * time.Hour
	DefaultCooldown          = 60 * time.Minute
	MaxSnapshotAge           = 10 * time.Minute
	DivergenceLimitBps       = 1000.0 // 10%
	MaxOwnerLength           = 128
)

// NewRule validates and builds an enabled Rule with CreatedAt = now.
// Owner is trimmed, must be non-empty and <= MaxOwnerLength. Symbol goes through ParseSymbol.
// Every range above is inclusive. Any violation returns ErrInvalidRule (wrapped, with detail).
// An invalid symbol is returned as ErrInvalidRule wrapping ErrInvalidSymbol (errors.Is matches both).
func NewRule(id RuleID, p NewRuleParams, now time.Time) (Rule, error) {
	owner := strings.TrimSpace(p.Owner)
	if owner == "" {
		return Rule{}, fmt.Errorf("%w: owner is required", ErrInvalidRule)
	}
	if len(owner) > MaxOwnerLength {
		return Rule{}, fmt.Errorf("%w: owner length %d exceeds max %d", ErrInvalidRule, len(owner), MaxOwnerLength)
	}

	sym, err := ParseSymbol(p.Symbol)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: %w", ErrInvalidRule, err)
	}

	if p.MaxPremiumBps < MinMaxPremiumBps || p.MaxPremiumBps > MaxMaxPremiumBps {
		return Rule{}, fmt.Errorf("%w: max premium bps %d out of range [%d, %d]", ErrInvalidRule, p.MaxPremiumBps, MinMaxPremiumBps, MaxMaxPremiumBps)
	}

	if p.Budget < MinBudget || p.Budget > MaxBudget {
		return Rule{}, fmt.Errorf("%w: budget %d out of range [%d, %d]", ErrInvalidRule, p.Budget, MinBudget, MaxBudget)
	}

	maxImpact := p.MaxPriceImpactBps
	if maxImpact == 0 {
		maxImpact = DefaultMaxPriceImpactBps
	}
	if maxImpact < MinPriceImpactBps || maxImpact > MaxPriceImpactBpsLimit {
		return Rule{}, fmt.Errorf("%w: max price impact bps %d out of range [%d, %d]", ErrInvalidRule, maxImpact, MinPriceImpactBps, MaxPriceImpactBpsLimit)
	}

	cooldown := p.Cooldown
	if cooldown == 0 {
		cooldown = DefaultCooldown
	}
	if cooldown < MinCooldown || cooldown > MaxCooldown {
		return Rule{}, fmt.Errorf("%w: cooldown %v out of range [%v, %v]", ErrInvalidRule, cooldown, MinCooldown, MaxCooldown)
	}

	return Rule{
		ID:                id,
		Owner:             OwnerID(owner),
		Symbol:            sym,
		MaxPremiumBps:     p.MaxPremiumBps,
		Budget:            p.Budget,
		MaxPriceImpactBps: maxImpact,
		Cooldown:          cooldown,
		Enabled:           true,
		CreatedAt:         now,
		LastTriggeredAt:   time.Time{},
	}, nil
}

func (r Rule) OwnedBy(owner OwnerID) bool {
	return r.Owner == owner
}

func (r Rule) WithEnabled(enabled bool) Rule {
	r.Enabled = enabled
	return r
}

func (r Rule) MarkTriggered(now time.Time) Rule {
	r.LastTriggeredAt = now
	return r
}
