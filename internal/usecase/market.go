package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.MarketReader = (*MarketQuery)(nil)

type MarketQuery struct {
	source ports.MarketDataSource
	snaps  ports.SnapshotReader
	clock  ports.Clock
}

func NewMarketQuery(source ports.MarketDataSource, snaps ports.SnapshotReader, clock ports.Clock) *MarketQuery {
	return &MarketQuery{
		source: source,
		snaps:  snaps,
		clock:  clock,
	}
}

func (q *MarketQuery) ListMarket(ctx context.Context) ([]ports.MarketView, error) {
	snaps, err := q.source.FetchAll(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]ports.MarketView, 0, len(snaps))
	for _, snap := range snaps {
		prem, err := snap.Premium()
		if err != nil {
			// Omit snapshots whose Premium() fails
			continue
		}
		views = append(views, ports.MarketView{
			Snapshot: snap,
			Premium:  prem,
			Label:    prem.Label(),
		})
	}

	sort.Slice(views, func(i, j int) bool {
		bi := views[i].Premium.Bps()
		bj := views[j].Premium.Bps()
		if bi != bj {
			return bi < bj
		}
		return views[i].Snapshot.Token.Symbol < views[j].Snapshot.Token.Symbol
	})

	return views, nil
}

func (q *MarketQuery) GetMarket(ctx context.Context, symbol string) (ports.MarketView, error) {
	sym, err := domain.ParseSymbol(symbol)
	if err != nil {
		return ports.MarketView{}, err
	}

	snaps, err := q.source.FetchAll(ctx)
	if err != nil {
		return ports.MarketView{}, err
	}

	for _, snap := range snaps {
		if snap.Token.Symbol == sym {
			prem, err := snap.Premium()
			if err != nil {
				return ports.MarketView{}, err
			}
			return ports.MarketView{
				Snapshot: snap,
				Premium:  prem,
				Label:    prem.Label(),
			}, nil
		}
	}

	return ports.MarketView{}, fmt.Errorf("%w: token %s", domain.ErrTokenNotFound, sym)
}

func (q *MarketQuery) History(ctx context.Context, symbol string, window time.Duration) ([]domain.MarketSnapshot, error) {
	sym, err := domain.ParseSymbol(symbol)
	if err != nil {
		return nil, err
	}

	if window < 1*time.Hour || window > 168*time.Hour {
		return nil, fmt.Errorf("%w: window must be between 1h and 168h", domain.ErrInvalidArgument)
	}

	since := q.clock.Now().Add(-window)
	return q.snaps.History(ctx, sym, since)
}
