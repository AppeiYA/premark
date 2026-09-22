package testsupport

import (
	"time"

	"premark/internal/domain"
)

// Snap creates a MarketSnapshot for test setup. Sets Token.Mint = "MINT_"+symbol.
func Snap(symbol string, tokenPrice, markPrice float64, at time.Time) domain.MarketSnapshot {
	sym, _ := domain.ParseSymbol(symbol)
	return domain.MarketSnapshot{
		Token: domain.Token{
			Symbol:      sym,
			Name:        symbol + " PreStocks",
			Description: symbol + " tokenized stock",
			Mint:        "MINT_" + symbol,
		},
		TokenPrice:       tokenPrice,
		MarkPrice:        markPrice,
		Supply:           1000.0,
		MarkValuation:    markPrice * 1000.0,
		ImpliedValuation: tokenPrice * 1000.0,
		ObservedAt:       at,
	}
}

// QuoteFor creates a Quote for test setup.
func QuoteFor(symbol string, budget domain.USDC, outTokens, impactBps float64, at time.Time) domain.Quote {
	sym, _ := domain.ParseSymbol(symbol)
	return domain.Quote{
		Symbol:         sym,
		InputMint:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		OutputMint:     "MINT_" + symbol,
		InAmount:       budget,
		OutTokens:      outTokens,
		PriceImpactBps: impactBps,
		SwapURL:        "https://jup.ag/swap/EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v-MINT_" + symbol,
		QuotedAt:       at,
	}
}

// RuleParams creates a NewRuleParams with defaults for test setup.
func RuleParams(owner, symbol string, maxPremiumBps int, budget domain.USDC) domain.NewRuleParams {
	return domain.NewRuleParams{
		Owner:         owner,
		Symbol:        symbol,
		MaxPremiumBps: maxPremiumBps,
		Budget:        budget,
	}
}
