package testsupport

import (
	"net/http"
	"testing"
	"time"

	"premark/internal/adapter/in/httpapi"
	"premark/internal/adapter/out/memstore"
	"premark/internal/ports"
	"premark/internal/usecase"
)

type App struct {
	Router    http.Handler
	Clock     *FixedClock
	Source    *FakeSource
	Quoter    *FakeQuoter
	Notifier  *FakeNotifier
	Rules     *memstore.RuleRepository
	Snapshots *memstore.SnapshotRepository
	Signals   *memstore.SignalRepository
	Scanner   ports.ScanRunner
}

func NewApp(t testing.TB) *App {
	return NewAppWithAdminToken(t, "")
}

func NewAppWithAdminToken(t testing.TB, token string) *App {
	clock := NewFixedClock(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC))
	ids := NewSeqIDs("test-id")
	source := &FakeSource{}
	quoter := NewFakeQuoter()
	notifier := &FakeNotifier{}

	rules := memstore.NewRuleRepository()
	snapshots := memstore.NewSnapshotRepository()
	signals := memstore.NewSignalRepository()

	ruleService := usecase.NewRuleService(rules, source, clock, ids)
	marketQuery := usecase.NewMarketQuery(source, snapshots, clock)
	signalQuery := usecase.NewSignalQuery(signals)
	ingestor := usecase.NewIngestor(source, snapshots, nil)
	evaluator := usecase.NewEvaluator(rules, quoter, signals, notifier, clock, ids, nil)
	scanner := usecase.NewScanner(ingestor, evaluator, nil)

	router := httpapi.NewRouter(httpapi.Deps{
		Rules:      ruleService,
		Market:     marketQuery,
		Signals:    signalQuery,
		Scanner:    scanner,
		AdminToken: token,
		Logger:     nil,
	})

	return &App{
		Router:    router,
		Clock:     clock,
		Source:    source,
		Quoter:    quoter,
		Notifier:  notifier,
		Rules:     rules,
		Snapshots: snapshots,
		Signals:   signals,
		Scanner:   scanner,
	}
}
