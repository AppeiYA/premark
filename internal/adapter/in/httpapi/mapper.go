package httpapi

import (
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

func toMarketDTO(view ports.MarketView) MarketDTO {
	return MarketDTO{
		Symbol:     view.Snapshot.Token.Symbol.String(),
		Name:       view.Snapshot.Token.Name,
		Mint:       view.Snapshot.Token.Mint,
		TokenPrice: view.Snapshot.TokenPrice,
		MarkPrice:  view.Snapshot.MarkPrice,
		PremiumBps: view.Premium.Bps(),
		Label:      string(view.Label),
		Supply:     view.Snapshot.Supply,
		ObservedAt: view.Snapshot.ObservedAt.UTC().Format(time.RFC3339),
	}
}

func toPointDTO(snap domain.MarketSnapshot) (PointDTO, bool) {
	prem, err := snap.Premium()
	if err != nil {
		return PointDTO{}, false
	}
	return PointDTO{
		ObservedAt: snap.ObservedAt.UTC().Format(time.RFC3339),
		TokenPrice: snap.TokenPrice,
		MarkPrice:  snap.MarkPrice,
		PremiumBps: prem.Bps(),
		Supply:     snap.Supply,
	}, true
}

func toRuleDTO(rule domain.Rule) RuleDTO {
	dto := RuleDTO{
		ID:                string(rule.ID),
		OwnerID:           string(rule.Owner),
		Symbol:            rule.Symbol.String(),
		MaxPremiumBps:     rule.MaxPremiumBps,
		BudgetUSDC:        rule.Budget.Float64(),
		MaxPriceImpactBps: rule.MaxPriceImpactBps,
		CooldownMinutes:   int(rule.Cooldown / time.Minute),
		Enabled:           rule.Enabled,
		CreatedAt:         rule.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !rule.LastTriggeredAt.IsZero() {
		t := rule.LastTriggeredAt.UTC().Format(time.RFC3339)
		dto.LastTriggeredAt = &t
	}
	return dto
}

func toSignalDTO(s domain.Signal) SignalDTO {
	return SignalDTO{
		ID:             string(s.ID),
		RuleID:         string(s.RuleID),
		Symbol:         s.Symbol.String(),
		TokenPrice:     s.TokenPrice,
		MarkPrice:      s.MarkPrice,
		APIPremiumBps:  s.APIPremiumBps,
		ExecPremiumBps: s.ExecPremiumBps,
		BudgetUSDC:     s.Quote.InAmount.Float64(),
		ExpectedTokens: s.Quote.OutTokens,
		PriceImpactBps: s.Quote.PriceImpactBps,
		SwapURL:        s.Quote.SwapURL,
		CreatedAt:      s.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toScanDTO(report ports.ScanReport) ScanDTO {
	skipReasons := make(map[string]int)
	for _, skip := range report.Evaluation.Skips {
		skipReasons[string(skip.Reason)]++
	}
	return ScanDTO{
		Snapshots:      report.Snapshots,
		RulesEvaluated: report.Evaluation.Evaluated,
		SignalsCreated: len(report.Evaluation.Signals),
		Skipped:        len(report.Evaluation.Skips),
		Failed:         len(report.Evaluation.Failures),
		SkipReasons:    skipReasons,
	}
}
