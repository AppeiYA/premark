package domain

import (
	"time"
)

type Token struct {
	Symbol      Symbol
	Name        string
	Description string
	Mint        string // Solana mint address
}

type MarketSnapshot struct {
	Token            Token
	TokenPrice       float64 // USD
	MarkPrice        float64 // USD
	Supply           float64
	MarkValuation    float64
	ImpliedValuation float64
	ObservedAt       time.Time
}

// Premium computes the snapshot's premium. ErrInvalidPrice if either price is <= 0, NaN or Inf.
func (s MarketSnapshot) Premium() (Premium, error) {
	return NewPremium(s.TokenPrice, s.MarkPrice)
}
