package ports

import (
	"premark/internal/domain"
)

type MarketView struct {
	Snapshot domain.MarketSnapshot
	Premium  domain.Premium
	Label    domain.PremiumLabel
}

type RuleSkip struct {
	RuleID domain.RuleID
	Symbol domain.Symbol
	Reason domain.SkipReason
}

type RuleFailure struct {
	RuleID domain.RuleID
	Err    error
}

type EvaluationReport struct {
	Evaluated int // number of enabled rules examined
	Signals   []domain.Signal
	Skips     []RuleSkip
	Failures  []RuleFailure
}

type ScanReport struct {
	Snapshots  int
	Evaluation EvaluationReport
}
