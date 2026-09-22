package ports

import (
	"context"
	"time"

	"premark/internal/domain"
)

type RuleManager interface {
	CreateRule(ctx context.Context, p domain.NewRuleParams) (domain.Rule, error)
	ListRules(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error)
	SetRuleEnabled(ctx context.Context, owner domain.OwnerID, id domain.RuleID, enabled bool) (domain.Rule, error)
	DeleteRule(ctx context.Context, owner domain.OwnerID, id domain.RuleID) error
}

type MarketReader interface {
	ListMarket(ctx context.Context) ([]MarketView, error)
	GetMarket(ctx context.Context, symbol string) (MarketView, error)
	History(ctx context.Context, symbol string, window time.Duration) ([]domain.MarketSnapshot, error)
}

type SignalReader interface {
	ListSignals(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error)
}

type MarketIngestor interface {
	Ingest(ctx context.Context) ([]domain.MarketSnapshot, error)
}

type RuleEvaluator interface {
	EvaluateAll(ctx context.Context, snapshots []domain.MarketSnapshot) (EvaluationReport, error)
}

type ScanRunner interface {
	Scan(ctx context.Context) (ScanReport, error)
}
