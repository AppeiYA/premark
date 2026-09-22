package usecase

import (
	"context"
	"log/slog"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.RuleEvaluator = (*Evaluator)(nil)

type Evaluator struct {
	rules    ports.RuleRepository
	quotes   ports.QuoteProvider
	signals  ports.SignalRepository
	notifier ports.Notifier
	clock    ports.Clock
	ids      ports.IDGenerator
	log      *slog.Logger
}

func NewEvaluator(rules ports.RuleRepository, quotes ports.QuoteProvider,
	signals ports.SignalRepository, notifier ports.Notifier,
	clock ports.Clock, ids ports.IDGenerator, log *slog.Logger) *Evaluator {
	return &Evaluator{
		rules:    rules,
		quotes:   quotes,
		signals:  signals,
		notifier: notifier,
		clock:    clock,
		ids:      ids,
		log:      log,
	}
}

func (e *Evaluator) EvaluateAll(ctx context.Context, snapshots []domain.MarketSnapshot) (ports.EvaluationReport, error) {
	enabledRules, err := e.rules.ListEnabled(ctx)
	if err != nil {
		return ports.EvaluationReport{}, err
	}

	snapMap := make(map[domain.Symbol]domain.MarketSnapshot, len(snapshots))
	for _, snap := range snapshots {
		snapMap[snap.Token.Symbol] = snap
	}

	now := e.clock.Now()
	report := ports.EvaluationReport{}

	for _, rule := range enabledRules {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		report.Evaluated++

		snapshot, ok := snapMap[rule.Symbol]
		if !ok {
			report.Skips = append(report.Skips, ports.RuleSkip{
				RuleID: rule.ID,
				Reason: domain.ReasonNoMarketData,
			})
			continue
		}

		precheckDecision := rule.Precheck(snapshot, now)
		if !precheckDecision.Trigger {
			report.Skips = append(report.Skips, ports.RuleSkip{
				RuleID: rule.ID,
				Reason: precheckDecision.Reason,
			})
			continue
		}

		quote, err := e.quotes.QuoteBuy(ctx, snapshot.Token, rule.Budget)
		if err != nil {
			report.Failures = append(report.Failures, ports.RuleFailure{
				RuleID: rule.ID,
				Err:    err,
			})
			continue
		}

		decideDecision := rule.Decide(snapshot, quote, now)
		if !decideDecision.Trigger {
			report.Skips = append(report.Skips, ports.RuleSkip{
				RuleID: rule.ID,
				Reason: decideDecision.Reason,
			})
			continue
		}

		signalID := domain.SignalID(e.ids.NewID())
		signal := domain.NewSignal(signalID, rule, snapshot, quote, decideDecision, now)

		if err := e.signals.Save(ctx, signal); err != nil {
			report.Failures = append(report.Failures, ports.RuleFailure{
				RuleID: rule.ID,
				Err:    err,
			})
			continue
		}

		if err := e.rules.Save(ctx, rule.MarkTriggered(now)); err != nil {
			report.Failures = append(report.Failures, ports.RuleFailure{
				RuleID: rule.ID,
				Err:    err,
			})
		}

		if err := e.notifier.Notify(ctx, signal); err != nil {
			if e.log != nil {
				e.log.Error("failed to notify signal", "signal_id", signal.ID, "error", err)
			}
		}

		report.Signals = append(report.Signals, signal)
	}

	return report, nil
}
